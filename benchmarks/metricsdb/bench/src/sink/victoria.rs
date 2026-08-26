use std::sync::{Arc, RwLock};
use std::time::{Duration, Instant};

use async_trait::async_trait;
use serde_json::{json, Value};

use super::http::{client, wait_until};
use super::{Ack, DbKind, DbSize, SettleReport, Sink, SinkReader, SinkWriter, WriteError};
use crate::model::catalog::SeriesCatalog;
use crate::pipeline::batch::Batch;
use crate::queries::{QueryId, QueryInstance, QueryOutcome};

/// Per-series protobuf `Label` entries (field 1 of TimeSeries), encoded once so
/// the write path is a memcpy plus one Sample per point.
#[derive(Default)]
struct LabelCode {
    bytes: Vec<u8>,
    offsets: Vec<u32>,
}

pub struct VictoriaSink {
    url: String,
    client: reqwest::Client,
    labels: Arc<RwLock<LabelCode>>,
    sim_end_ms: i64,
}

pub fn sanitize(s: &str) -> String {
    s.chars().map(|c| if c.is_ascii_alphanumeric() || c == '_' { c } else { '_' }).collect()
}

fn put_varint(out: &mut Vec<u8>, mut v: u64) {
    while v >= 0x80 {
        out.push((v as u8) | 0x80);
        v >>= 7;
    }
    out.push(v as u8);
}

fn put_len_delim(out: &mut Vec<u8>, tag: u8, payload: &[u8]) {
    out.push(tag);
    put_varint(out, payload.len() as u64);
    out.extend_from_slice(payload);
}

fn encode_label(out: &mut Vec<u8>, k: &str, v: &str) {
    let mut lbl = Vec::with_capacity(k.len() + v.len() + 4);
    put_len_delim(&mut lbl, 0x0A, k.as_bytes());
    put_len_delim(&mut lbl, 0x12, v.as_bytes());
    put_len_delim(out, 0x0A, &lbl);
}

impl VictoriaSink {
    pub fn new(url: String, cli: &crate::cli::Cli) -> anyhow::Result<Self> {
        Ok(Self {
            url: url.trim_end_matches('/').to_string(),
            client: client(Duration::from_secs(120)),
            labels: Arc::new(RwLock::new(LabelCode::default())),
            sim_end_ms: cli.sim_start_ms() + (cli.points.div_ceil(cli.series) as i64) * cli.interval.as_millis() as i64,
        })
    }

    async fn metrics(&self) -> anyhow::Result<Vec<(String, String, f64)>> {
        let text = self.client.get(format!("{}/metrics", self.url)).send().await?.text().await?;
        Ok(parse_exposition(&text))
    }
}

pub fn parse_exposition(text: &str) -> Vec<(String, String, f64)> {
    let mut out = Vec::new();
    for line in text.lines() {
        if line.starts_with('#') || line.trim().is_empty() {
            continue;
        }
        let (head, val) = match line.rsplit_once(' ') {
            Some(x) => x,
            None => continue,
        };
        let val: f64 = match val.trim().parse() {
            Ok(v) => v,
            Err(_) => continue,
        };
        let (name, labels) = match head.split_once('{') {
            Some((n, l)) => (n.to_string(), l.trim_end_matches('}').to_string()),
            None => (head.trim().to_string(), String::new()),
        };
        out.push((name, labels, val));
    }
    out
}

fn sum(m: &[(String, String, f64)], name: &str, label_filter: Option<&str>) -> f64 {
    m.iter().filter(|(n, l, _)| n == name && label_filter.map(|f| l.contains(f)).unwrap_or(true)).map(|(_, _, v)| v).sum()
}

#[async_trait]
impl Sink for VictoriaSink {
    fn kind(&self) -> DbKind {
        DbKind::Victoriametrics
    }

    fn ack_semantics(&self) -> &'static str {
        "remote write answered 204 (samples in the in-memory part, flushed to disk within ~1s)"
    }

    async fn setup(&self, _reset: bool, timeout: Duration) -> anyhow::Result<()> {
        let c = self.client.clone();
        let url = format!("{}/health", self.url);
        wait_until(timeout, || {
            let c = c.clone();
            let url = url.clone();
            async move { matches!(c.get(&url).send().await, Ok(r) if r.status().is_success()) }
        })
        .await
    }

    async fn register_series(&self, cat: &SeriesCatalog) -> anyhow::Result<()> {
        let n = cat.len() as usize;
        let mut code = LabelCode { bytes: Vec::with_capacity(n * 260), offsets: Vec::with_capacity(n + 1) };
        for id in 0..n as u64 {
            code.offsets.push(code.bytes.len() as u32);
            encode_label(&mut code.bytes, "__name__", &sanitize(cat.name_of(id)));
            for (k, v) in cat.tags_of(id) {
                encode_label(&mut code.bytes, &sanitize(k), v);
            }
        }
        code.offsets.push(code.bytes.len() as u32);
        *self.labels.write().unwrap() = code;
        Ok(())
    }

    async fn writer(&self, _idx: usize) -> anyhow::Result<Box<dyn SinkWriter>> {
        Ok(Box::new(VmWriter { client: client(Duration::from_secs(120)), url: format!("{}/api/v1/write", self.url), labels: Arc::clone(&self.labels), buf: Vec::new(), snappy: Vec::new() }))
    }

    async fn reader(&self) -> anyhow::Result<Box<dyn SinkReader>> {
        Ok(Box::new(VmReader { client: client(Duration::from_secs(120)), url: self.url.clone(), sim_end_ms: self.sim_end_ms }))
    }

    async fn health(&self) -> anyhow::Result<Value> {
        let m = self.metrics().await?;
        Ok(json!({
            "db": "victoriametrics",
            "rows_inserted": sum(&m, "vm_rows_inserted_total", None),
            "slow_row_inserts": sum(&m, "vm_slow_row_inserts_total", None),
            "slow_per_day_index_inserts": sum(&m, "vm_slow_per_day_index_inserts_total", None),
            "slow_metric_name_loads": sum(&m, "vm_slow_metric_name_loads_total", None),
            "rows_ignored": sum(&m, "vm_rows_ignored_total", None),
            "active_merges": sum(&m, "vm_active_merges", None),
            "pending_rows": sum(&m, "vm_pending_rows", None),
            "parts_inmemory": sum(&m, "vm_parts", Some("type=\"storage/inmemory\"")),
            "parts_small": sum(&m, "vm_parts", Some("type=\"storage/small\"")),
            "parts_big": sum(&m, "vm_parts", Some("type=\"storage/big\"")),
            "rows_in_storage": sum(&m, "vm_rows", Some("type=\"storage/")),
            "data_size_bytes": sum(&m, "vm_data_size_bytes", None),
            "cache_size_bytes": sum(&m, "vm_cache_size_bytes", None),
            "new_timeseries_created": sum(&m, "vm_new_timeseries_created_total", None),
            "concurrent_insert_current": sum(&m, "vm_concurrent_insert_current", None),
            "rss_bytes": sum(&m, "process_resident_memory_bytes", None),
            "process_start_time_s": sum(&m, "process_start_time_seconds", None),
        }))
    }

    async fn is_reachable(&self) -> bool {
        matches!(self.client.get(format!("{}/health", self.url)).send().await, Ok(r) if r.status().is_success())
    }

    async fn settle(&self, deadline: Instant) -> SettleReport {
        let mut rep = SettleReport::default();
        let t0 = Instant::now();
        let ok = matches!(self.client.get(format!("{}/internal/force_flush", self.url)).send().await, Ok(r) if r.status().is_success());
        rep.step("force_flush", t0, ok, "");
        let t1 = Instant::now();
        let ok = matches!(self.client.get(format!("{}/internal/force_merge", self.url)).send().await, Ok(r) if r.status().is_success());
        rep.step("force_merge_requested", t1, ok, "");
        let t2 = Instant::now();
        let mut prev_size = None;
        let mut stable = 0;
        loop {
            let m = self.metrics().await.unwrap_or_default();
            let merges = sum(&m, "vm_active_merges", None);
            let size = sum(&m, "vm_data_size_bytes", None) as u64;
            if merges == 0.0 && prev_size == Some(size) {
                stable += 1;
            } else {
                stable = 0;
            }
            prev_size = Some(size);
            if stable >= 2 {
                rep.step("merges_idle", t2, true, format!("data_size_bytes={size}"));
                rep.settled = true;
                return rep;
            }
            if Instant::now() > deadline {
                rep.step("merges_idle", t2, false, format!("deadline: active_merges={merges} data_size_bytes={size}"));
                return rep;
            }
            tokio::time::sleep(Duration::from_secs(5)).await;
        }
    }

    async fn db_size(&self) -> anyhow::Result<DbSize> {
        let m = self.metrics().await?;
        Ok(DbSize {
            compressed_bytes: Some(sum(&m, "vm_data_size_bytes", None) as u64),
            uncompressed_bytes: None,
            rows: Some(sum(&m, "vm_rows", Some("type=\"storage/")) as u64),
        })
    }

    fn disk_class(&self, rel: &str) -> &'static str {
        if rel.starts_with("data/") {
            "data"
        } else if rel.starts_with("indexdb/") {
            "index"
        } else if rel.starts_with("cache/") {
            "cache"
        } else {
            "other"
        }
    }

    fn settings(&self) -> Value {
        json!({ "protocol": "prometheus remote write (protobuf + snappy)", "labels": "pre-encoded per series, sorted" })
    }
}

struct VmWriter {
    client: reqwest::Client,
    url: String,
    labels: Arc<RwLock<LabelCode>>,
    buf: Vec<u8>,
    snappy: Vec<u8>,
}

#[async_trait]
impl SinkWriter for VmWriter {
    async fn write(&mut self, batch: &Batch, _cat: &SeriesCatalog) -> Result<Ack, WriteError> {
        self.buf.clear();
        {
            let code = self.labels.read().unwrap();
            let mut ts_msg: Vec<u8> = Vec::with_capacity(400);
            let mut sample = [0u8; 20];
            for i in 0..batch.len() {
                let id = batch.series_id[i] as usize;
                let (lo, hi) = (code.offsets[id] as usize, code.offsets[id + 1] as usize);
                ts_msg.clear();
                ts_msg.extend_from_slice(&code.bytes[lo..hi]);
                let mut n = 0;
                sample[n] = 0x09;
                n += 1;
                sample[n..n + 8].copy_from_slice(&batch.value[i].to_le_bytes());
                n += 8;
                sample[n] = 0x10;
                n += 1;
                let mut v = batch.ts_ms[i] as u64;
                while v >= 0x80 {
                    sample[n] = (v as u8) | 0x80;
                    n += 1;
                    v >>= 7;
                }
                sample[n] = v as u8;
                n += 1;
                put_len_delim(&mut ts_msg, 0x12, &sample[..n]);
                put_len_delim(&mut self.buf, 0x0A, &ts_msg);
            }
        }
        self.snappy.clear();
        self.snappy.resize(snap::raw::max_compress_len(self.buf.len()), 0);
        let n = snap::raw::Encoder::new().compress(&self.buf, &mut self.snappy).map_err(|e| WriteError::fatal(e.to_string()))?;
        self.snappy.truncate(n);
        let body = self.snappy.clone();
        let bytes = body.len() as u64;
        let resp = self
            .client
            .post(&self.url)
            .header("Content-Encoding", "snappy")
            .header("Content-Type", "application/x-protobuf")
            .header("X-Prometheus-Remote-Write-Version", "0.1.0")
            .body(body)
            .send()
            .await?;
        let status = resp.status();
        if status.is_success() {
            return Ok(Ack { bytes_sent: bytes, maintenance: Vec::new() });
        }
        let text = resp.text().await.unwrap_or_default();
        Err(WriteError { retryable: status.is_server_error() || status.as_u16() == 429, msg: format!("{} {}", status, text.chars().take(300).collect::<String>()) })
    }
}

struct VmReader {
    client: reqwest::Client,
    url: String,
    sim_end_ms: i64,
}

impl VmReader {
    fn metric(name: &str) -> String {
        sanitize(name)
    }
}

fn count_range_rows(v: &Value) -> u64 {
    v.pointer("/data/result").and_then(Value::as_array).map(|r| r.iter().map(|s| s.get("values").and_then(Value::as_array).map(|x| x.len() as u64).unwrap_or(1)).sum()).unwrap_or(0)
}

#[async_trait]
impl SinkReader for VmReader {
    async fn visible_max_ts(&mut self) -> anyhow::Result<Option<i64>> {
        let t = self.sim_end_ms / 1000 + 86_400;
        let resp = self
            .client
            .get(format!("{}/api/v1/query", self.url))
            .query(&[("query", "tlast_over_time(bench_sentinel[400d])"), ("time", &t.to_string())])
            .send()
            .await?;
        let v: Value = resp.json().await?;
        let val = v
            .pointer("/data/result/0/value/1")
            .and_then(Value::as_str)
            .and_then(|s| s.parse::<f64>().ok());
        Ok(val.map(|s| (s * 1000.0).round() as i64))
    }

    async fn run_query(&mut self, q: &QueryInstance, deadline: Duration) -> QueryOutcome {
        let p = &q.params;
        let t0 = Instant::now();
        let start = (q.from_ms() / 1000).to_string();
        let end = (q.t_ms / 1000).to_string();
        let one_host = p.one_host.clone();
        let (path, params): (&str, Vec<(&str, String)>) = match q.id {
            QueryId::A => ("/api/v1/query_range", vec![("query", format!("avg by (host_name) (avg_over_time({}{{state=\"user\"}}[1m]))", Self::metric(&p.cpu.name))), ("start", start), ("end", end), ("step", "60".into())]),
            QueryId::B => ("/api/v1/query_range", vec![("query", format!("avg_over_time({}{{host_name=\"{one_host}\",cpu=\"3\",state=\"user\"}}[1m])", Self::metric(&p.cpu.name))), ("start", start), ("end", end), ("step", "60".into())]),
            QueryId::C => ("/api/v1/query", vec![("query", format!("topk(20, max by (host_name) (last_over_time({}{{state=\"used\"}}[10m])))", Self::metric(&p.mem.name))), ("time", end)]),
            QueryId::D => ("/api/v1/query_range", vec![("query", format!("sum by (k8s_cluster_name) (avg_over_time({}[5m]))", Self::metric(&p.pod.name))), ("start", start), ("end", end), ("step", "300".into())]),
            QueryId::E1 => ("/api/v1/label/__name__/values", vec![("start", start), ("end", end)]),
            QueryId::E2 => ("/api/v1/label/host_name/values", vec![("match[]", Self::metric(&p.cpu.name)), ("start", start), ("end", end)]),
            QueryId::F => ("/api/v1/query", vec![("query", format!("count(last_over_time({m}[5m])) or quantile(0.95, last_over_time({m}[5m]))", m = Self::metric(&p.http.name))), ("time", end)]),
        };
        let send = |params: Vec<(&str, String)>| {
            self.client.get(format!("{}{}", self.url, path)).timeout(deadline + Duration::from_secs(2)).query(&params).query(&[("timeout", format!("{}s", deadline.as_secs().max(1)))]).send()
        };
        let mut rows = 0u64;
        let calls: Vec<Vec<(&str, String)>> = if q.id == QueryId::F {
            let m = Self::metric(&p.http.name);
            vec![
                vec![("query", format!("count(last_over_time({m}[5m]))")), ("time", (q.t_ms / 1000).to_string())],
                vec![("query", format!("quantile(0.95, last_over_time({m}[5m]))")), ("time", (q.t_ms / 1000).to_string())],
            ]
        } else {
            vec![params]
        };
        for call in calls {
            let resp = match send(call).await {
                Ok(r) => r,
                Err(e) if e.is_timeout() => return QueryOutcome::timeout(q.id, t0.elapsed()),
                Err(e) => return QueryOutcome::error(q.id, t0.elapsed(), e.to_string()),
            };
            let status = resp.status();
            let text = match resp.text().await {
                Ok(t) => t,
                Err(e) => return QueryOutcome::error(q.id, t0.elapsed(), e.to_string()),
            };
            if !status.is_success() {
                if text.contains("deadline") || text.contains("timeout") {
                    return QueryOutcome::timeout(q.id, t0.elapsed());
                }
                return QueryOutcome::error(q.id, t0.elapsed(), format!("{status} {}", text.chars().take(300).collect::<String>()));
            }
            let v: Value = match serde_json::from_str(&text) {
                Ok(v) => v,
                Err(e) => return QueryOutcome::error(q.id, t0.elapsed(), e.to_string()),
            };
            rows += match q.id {
                QueryId::A | QueryId::B | QueryId::D => count_range_rows(&v),
                QueryId::C | QueryId::F => v.pointer("/data/result").and_then(Value::as_array).map(|a| a.len() as u64).unwrap_or(0),
                QueryId::E1 | QueryId::E2 => v.get("data").and_then(Value::as_array).map(|a| a.len() as u64).unwrap_or(0),
            };
        }
        let expected = if q.id == QueryId::F { 2 } else { p.expected_rows(q.id, q.t_ms) };
        QueryOutcome::ok(q.id, t0.elapsed(), rows, expected)
    }
}

