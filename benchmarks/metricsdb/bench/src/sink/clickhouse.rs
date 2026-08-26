use std::time::{Duration, Instant};

use async_trait::async_trait;
use serde_json::{json, Value};

use super::http::{client, wait_until};
use super::{Ack, DbKind, DbSize, SettleReport, Sink, SinkReader, SinkWriter, WriteError};
use crate::model::catalog::{SeriesCatalog, SENTINEL_ID, SENTINEL_NAME};
use crate::pipeline::batch::Batch;
use crate::queries::sql::{render, Dialect};
use crate::queries::{QueryInstance, QueryOutcome};

const SCHEMA: &str = include_str!("../../../db/clickhouse/schema.sql");
const SCHEMA_MAP: &str = include_str!("../../../db/clickhouse/schema-map.sql");

pub struct ClickhouseSink {
    kind: DbKind,
    url: String,
    db: String,
    client: reqwest::Client,
}

impl ClickhouseSink {
    pub fn new(kind: DbKind, url: String, cli: &crate::cli::Cli) -> anyhow::Result<Self> {
        Ok(Self { kind, url: url.trim_end_matches('/').to_string(), db: cli.ch_database.clone(), client: client(Duration::from_secs(600)) })
    }

    fn map(&self) -> bool {
        self.kind == DbKind::ClickhouseMap
    }

    fn points_table(&self) -> &'static str {
        if self.map() {
            "metric_points"
        } else {
            "points"
        }
    }

    async fn exec(&self, sql: &str, database: bool) -> anyhow::Result<String> {
        let mut req = self.client.post(format!("{}/", self.url));
        if database {
            req = req.query(&[("database", self.db.as_str())]);
        }
        let resp = req.body(sql.to_string()).send().await?;
        let status = resp.status();
        let text = resp.text().await?;
        if !status.is_success() {
            anyhow::bail!("clickhouse {}: {}", status, text.trim());
        }
        Ok(text)
    }

    async fn scalar_u64(&self, sql: &str) -> anyhow::Result<u64> {
        Ok(self.exec(sql, true).await?.trim().parse().unwrap_or(0))
    }

}

fn put_varint(out: &mut Vec<u8>, mut v: u64) {
    while v >= 0x80 {
        out.push((v as u8) | 0x80);
        v >>= 7;
    }
    out.push(v as u8);
}

fn put_str(out: &mut Vec<u8>, s: &str) {
    put_varint(out, s.len() as u64);
    out.extend_from_slice(s.as_bytes());
}

fn put_tags(out: &mut Vec<u8>, tags: &[(&str, &str)]) {
    put_varint(out, tags.len() as u64);
    for (k, v) in tags {
        put_str(out, k);
        put_str(out, v);
    }
}

#[async_trait]
impl Sink for ClickhouseSink {
    fn kind(&self) -> DbKind {
        self.kind
    }

    fn ack_semantics(&self) -> &'static str {
        "INSERT ... FORMAT RowBinary answered 200 (async_insert=0: the part is written)"
    }

    async fn setup(&self, reset: bool, timeout: Duration) -> anyhow::Result<()> {
        let c = self.client.clone();
        let ping = format!("{}/ping", self.url);
        wait_until(timeout, || {
            let c = c.clone();
            let ping = ping.clone();
            async move { matches!(c.get(&ping).send().await, Ok(r) if r.status().is_success()) }
        })
        .await?;
        if reset {
            self.exec(&format!("DROP DATABASE IF EXISTS {}", self.db), false).await?;
        }
        self.exec(&format!("CREATE DATABASE IF NOT EXISTS {}", self.db), false).await?;
        let schema = if self.map() { SCHEMA_MAP } else { SCHEMA };
        for stmt in super::split_statements(schema) {
            self.exec(&stmt, true).await?;
        }
        Ok(())
    }

    async fn register_series(&self, cat: &SeriesCatalog) -> anyhow::Result<()> {
        if self.map() {
            return Ok(());
        }
        let n = cat.len();
        let mut id = 0u64;
        while id < n {
            let end = (id + 200_000).min(n);
            let mut body = Vec::with_capacity(((end - id) * 300) as usize);
            for sid in id..end {
                body.extend_from_slice(&sid.to_le_bytes());
                put_str(&mut body, cat.name_of(sid));
                put_tags(&mut body, &cat.tags_of(sid));
            }
            let resp = self
                .client
                .post(format!("{}/", self.url))
                .query(&[("database", self.db.as_str()), ("query", "INSERT INTO series (series_id, name, tags) FORMAT RowBinary")])
                .body(body)
                .send()
                .await?;
            if !resp.status().is_success() {
                anyhow::bail!("series insert failed: {} {}", resp.status(), resp.text().await.unwrap_or_default());
            }
            id = end;
        }
        Ok(())
    }

    async fn writer(&self, _idx: usize) -> anyhow::Result<Box<dyn SinkWriter>> {
        let query = if self.map() {
            "INSERT INTO metric_points (name, value, tags, recorded_at) FORMAT RowBinary"
        } else {
            "INSERT INTO points (series_id, ts, value) FORMAT RowBinary"
        };
        Ok(Box::new(ChWriter {
            client: client(Duration::from_secs(600)),
            url: format!("{}/", self.url),
            db: self.db.clone(),
            query: query.to_string(),
            map: self.map(),
        }))
    }

    async fn reader(&self) -> anyhow::Result<Box<dyn SinkReader>> {
        Ok(Box::new(ChReader { client: client(Duration::from_secs(120)), url: format!("{}/", self.url), db: self.db.clone(), map: self.map() }))
    }

    async fn health(&self) -> anyhow::Result<Value> {
        let table = self.points_table();
        let parts = self
            .exec(&format!("SELECT count(), sum(rows), sum(bytes_on_disk) FROM system.parts WHERE active AND database = '{}' AND table = '{table}' FORMAT TabSeparated", self.db), true)
            .await?;
        let mut it = parts.split_whitespace();
        let parts_n: u64 = it.next().and_then(|s| s.parse().ok()).unwrap_or(0);
        let rows: u64 = it.next().and_then(|s| s.parse().ok()).unwrap_or(0);
        let bytes: u64 = it.next().and_then(|s| s.parse().ok()).unwrap_or(0);
        let merges = self.exec("SELECT count(), max(elapsed) FROM system.merges FORMAT TabSeparated", true).await?;
        let mut it = merges.split_whitespace();
        let merges_n: u64 = it.next().and_then(|s| s.parse().ok()).unwrap_or(0);
        let longest: f64 = it.next().and_then(|s| s.parse().ok()).unwrap_or(0.0);
        let events = self
            .exec("SELECT event, value FROM system.events WHERE event IN ('DelayedInserts', 'RejectedInserts', 'InsertedRows', 'MergedRows') FORMAT TabSeparated", true)
            .await?;
        let mut ev = serde_json::Map::new();
        for line in events.lines() {
            let mut it = line.split('\t');
            if let (Some(k), Some(v)) = (it.next(), it.next()) {
                ev.insert(k.to_string(), json!(v.parse::<u64>().unwrap_or(0)));
            }
        }
        let mem = self.scalar_u64("SELECT value FROM system.metrics WHERE metric = 'MemoryTracking' FORMAT TabSeparated").await.unwrap_or(0);
        let uptime = self.scalar_u64("SELECT uptime() FORMAT TabSeparated").await.unwrap_or(0);
        Ok(json!({
            "db": "clickhouse",
            "active_parts": parts_n,
            "rows_in_parts": rows,
            "bytes_on_disk": bytes,
            "active_merges": merges_n,
            "longest_merge_s": longest,
            "delayed_inserts": ev.get("DelayedInserts").cloned().unwrap_or(json!(0)),
            "rejected_inserts": ev.get("RejectedInserts").cloned().unwrap_or(json!(0)),
            "inserted_rows": ev.get("InsertedRows").cloned().unwrap_or(json!(0)),
            "merged_rows": ev.get("MergedRows").cloned().unwrap_or(json!(0)),
            "memory_tracking": mem,
            "uptime_s": uptime,
        }))
    }

    async fn is_reachable(&self) -> bool {
        matches!(self.client.get(format!("{}/ping", self.url)).send().await, Ok(r) if r.status().is_success())
    }

    async fn settle(&self, deadline: Instant) -> SettleReport {
        let mut rep = SettleReport::default();
        let t0 = Instant::now();
        let table = self.points_table();
        let mut prev_parts: Option<u64> = None;
        let mut stable = 0;
        loop {
            let merges = self.scalar_u64("SELECT count() FROM system.merges FORMAT TabSeparated").await.unwrap_or(u64::MAX);
            let parts = self
                .scalar_u64(&format!("SELECT count() FROM system.parts WHERE active AND database = '{}' AND table = '{table}' FORMAT TabSeparated", self.db))
                .await
                .unwrap_or(u64::MAX);
            let inactive = self
                .scalar_u64(&format!("SELECT count() FROM system.parts WHERE NOT active AND database = '{}' AND table = '{table}' FORMAT TabSeparated", self.db))
                .await
                .unwrap_or(u64::MAX);
            let delta_ok = prev_parts.map(|p| (p as f64 - parts as f64).abs() <= 0.05 * p.max(1) as f64).unwrap_or(false);
            if merges == 0 && delta_ok && inactive == 0 {
                stable += 1;
            } else {
                stable = 0;
            }
            prev_parts = Some(parts);
            if stable >= 2 {
                rep.step("merges_idle", t0, true, format!("active_parts={parts}"));
                rep.settled = true;
                return rep;
            }
            if Instant::now() > deadline {
                rep.step("merges_idle", t0, false, format!("deadline: merges={merges} active_parts={parts} inactive_parts={inactive}"));
                return rep;
            }
            tokio::time::sleep(Duration::from_secs(5)).await;
        }
    }

    async fn db_size(&self) -> anyhow::Result<DbSize> {
        let table = self.points_table();
        let out = self
            .exec(&format!("SELECT sum(bytes_on_disk), sum(data_uncompressed_bytes), sum(rows) FROM system.parts WHERE active AND database = '{}' AND table = '{table}' FORMAT TabSeparated", self.db), true)
            .await?;
        let mut it = out.split_whitespace().map(|s| s.parse::<u64>().ok());
        Ok(DbSize { compressed_bytes: it.next().flatten(), uncompressed_bytes: it.next().flatten(), rows: it.next().flatten() })
    }

    fn disk_class(&self, rel: &str) -> &'static str {
        if rel.starts_with("store/") || rel.starts_with("data/") {
            "data"
        } else if rel.starts_with("tmp") {
            "tmp"
        } else {
            "other"
        }
    }

    fn settings(&self) -> Value {
        json!({ "database": self.db, "schema": if self.map() { "map" } else { "normalized" }, "insert_format": "RowBinary over HTTP", "async_insert": 0 })
    }
}

struct ChWriter {
    client: reqwest::Client,
    url: String,
    db: String,
    query: String,
    map: bool,
}

#[async_trait]
impl SinkWriter for ChWriter {
    async fn write(&mut self, batch: &Batch, cat: &SeriesCatalog) -> Result<Ack, WriteError> {
        let mut body = Vec::with_capacity(batch.len() * if self.map { 260 } else { 24 });
        if self.map {
            for i in 0..batch.len() {
                let id = batch.series_id[i];
                put_str(&mut body, cat.name_of(id));
                body.extend_from_slice(&batch.value[i].to_le_bytes());
                put_tags(&mut body, &cat.tags_of(id));
                body.extend_from_slice(&batch.ts_ms[i].to_le_bytes());
            }
        } else {
            for i in 0..batch.len() {
                body.extend_from_slice(&batch.series_id[i].to_le_bytes());
                body.extend_from_slice(&batch.ts_ms[i].to_le_bytes());
                body.extend_from_slice(&batch.value[i].to_le_bytes());
            }
        }
        let bytes = body.len() as u64;
        let resp = self.client.post(&self.url).query(&[("database", self.db.as_str()), ("query", self.query.as_str())]).body(body).send().await?;
        let status = resp.status();
        if status.is_success() {
            return Ok(Ack { bytes_sent: bytes, maintenance: Vec::new() });
        }
        let text = resp.text().await.unwrap_or_default();
        let too_many_parts = text.contains("TOO_MANY_PARTS") || text.contains("Code: 252");
        let retryable = too_many_parts || status.is_server_error() || status.as_u16() == 429;
        Err(WriteError { retryable, msg: format!("{} {}", status, text.lines().next().unwrap_or("").chars().take(300).collect::<String>()) })
    }
}

struct ChReader {
    client: reqwest::Client,
    url: String,
    db: String,
    map: bool,
}

#[async_trait]
impl SinkReader for ChReader {
    async fn visible_max_ts(&mut self) -> anyhow::Result<Option<i64>> {
        let sql = if self.map {
            format!("SELECT toUnixTimestamp64Milli(max(recorded_at)) FROM metric_points WHERE name = '{SENTINEL_NAME}' FORMAT TabSeparated")
        } else {
            format!("SELECT toUnixTimestamp64Milli(max(ts)) FROM points WHERE series_id = {SENTINEL_ID} FORMAT TabSeparated")
        };
        let resp = self.client.post(&self.url).query(&[("database", self.db.as_str())]).body(sql).send().await?;
        let text = resp.text().await?;
        let v: i64 = text.trim().parse().unwrap_or(0);
        Ok(if v <= 0 { None } else { Some(v) })
    }

    async fn run_query(&mut self, q: &QueryInstance, deadline: Duration) -> QueryOutcome {
        let sql = if self.map { render_map(q) } else { render(Dialect::ClickHouse, q) };
        let t0 = Instant::now();
        let secs = deadline.as_secs().max(1).to_string();
        let resp = self
            .client
            .post(&self.url)
            .timeout(deadline + Duration::from_secs(2))
            .query(&[("database", self.db.as_str()), ("default_format", "JSONCompact"), ("max_execution_time", secs.as_str())])
            .body(sql)
            .send()
            .await;
        let resp = match resp {
            Ok(r) => r,
            Err(e) if e.is_timeout() => return QueryOutcome::timeout(q.id, t0.elapsed()),
            Err(e) => return QueryOutcome::error(q.id, t0.elapsed(), e.to_string()),
        };
        let status = resp.status();
        let text = match resp.text().await {
            Ok(t) => t,
            Err(e) if e.is_timeout() => return QueryOutcome::timeout(q.id, t0.elapsed()),
            Err(e) => return QueryOutcome::error(q.id, t0.elapsed(), e.to_string()),
        };
        let elapsed = t0.elapsed();
        if !status.is_success() {
            if text.contains("TIMEOUT_EXCEEDED") || text.contains("Code: 159") {
                return QueryOutcome::timeout(q.id, elapsed);
            }
            return QueryOutcome::error(q.id, elapsed, text.lines().next().unwrap_or("").to_string());
        }
        let rows = serde_json::from_str::<Value>(&text).ok().and_then(|v| v.get("rows").and_then(Value::as_u64)).unwrap_or(0);
        QueryOutcome::ok(q.id, elapsed, rows, q.params.expected_rows(q.id, q.t_ms))
    }
}

/// The map-schema baseline runs the same intents against Traceway's own
/// layout, where identity lives inline in `tags`.
fn render_map(q: &QueryInstance) -> String {
    use crate::queries::QueryId;
    let p = &q.params;
    let from = format!("toDateTime64('{}', 3, 'UTC')", crate::util::fmt_ts_ms(q.from_ms()));
    let to = format!("toDateTime64('{}', 3, 'UTC')", crate::util::fmt_ts_ms(q.t_ms));
    match q.id {
        QueryId::A => format!(
            "SELECT toStartOfInterval(recorded_at, INTERVAL 60 SECOND) AS b, tags['host.name'] AS host, avg(value) AS v FROM metric_points \
             WHERE name = '{}' AND tags['state'] = 'user' AND recorded_at >= {from} AND recorded_at < {to} GROUP BY b, host ORDER BY b, host",
            p.cpu.name
        ),
        QueryId::B => format!(
            "SELECT toStartOfInterval(recorded_at, INTERVAL 60 SECOND) AS b, avg(value) AS v FROM metric_points \
             WHERE name = '{}' AND tags['host.name'] = '{}' AND tags['cpu'] = '3' AND tags['state'] = 'user' AND recorded_at >= {from} AND recorded_at < {to} GROUP BY b ORDER BY b",
            p.cpu.name, p.one_host
        ),
        QueryId::C => format!(
            "SELECT tags['host.name'] AS host, argMax(value, recorded_at) AS v FROM metric_points \
             WHERE name = '{}' AND tags['state'] = 'used' AND recorded_at >= {from} AND recorded_at < {to} GROUP BY host ORDER BY v DESC LIMIT 20",
            p.mem.name
        ),
        QueryId::D => format!(
            "SELECT toStartOfInterval(recorded_at, INTERVAL 300 SECOND) AS b, tags['k8s.cluster.name'] AS cluster, avg(value) AS v FROM metric_points \
             WHERE name = '{}' AND recorded_at >= {from} AND recorded_at < {to} GROUP BY b, cluster ORDER BY b, cluster",
            p.pod.name
        ),
        QueryId::E1 => format!(
            "SELECT name, uniqExact(cityHash64(mapKeys(tags), mapValues(tags))) AS n FROM metric_points \
             WHERE recorded_at >= {from} AND recorded_at < {to} GROUP BY name ORDER BY name"
        ),
        QueryId::E2 => format!(
            "SELECT DISTINCT tags['host.name'] AS host FROM metric_points WHERE name = '{}' AND recorded_at >= {from} AND recorded_at < {to} ORDER BY host",
            p.cpu.name
        ),
        QueryId::F => format!(
            "SELECT count(), quantile(0.95)(v) FROM (SELECT cityHash64(mapKeys(tags), mapValues(tags)) AS sid, argMax(value, recorded_at) AS v FROM metric_points \
             WHERE name = '{}' AND recorded_at >= {from} AND recorded_at < {to} GROUP BY sid)",
            p.http.name
        ),
    }
}
