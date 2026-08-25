use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::Arc;
use std::time::{Duration, Instant};

use arrow::array::{Float64Array, Int64Array, StringArray, TimestampMicrosecondArray};
use arrow::datatypes::{DataType, Field, Schema, TimeUnit};
use arrow::record_batch::RecordBatch;
use async_trait::async_trait;
use parquet::arrow::ArrowWriter;
use parquet::basic::Compression;
use parquet::file::properties::WriterProperties;
use serde_json::{json, Value};

use super::http::{client, wait_until};
use super::{Ack, DbKind, DbSize, SettleReport, Sink, SinkReader, SinkWriter, WriteError};
use crate::model::catalog::{SeriesCatalog, SENTINEL_ID};
use crate::pipeline::batch::Batch;
use crate::pipeline::writer::MaintEvent;
use crate::queries::sql::{render, Dialect};
use crate::queries::{QueryInstance, QueryOutcome};
use crate::util::parse_ts_ms;

const SCHEMA: &str = include_str!("../../../db/firebolt/schema.sql");

#[derive(Clone)]
pub struct FireboltSink {
    url: String,
    client: reqwest::Client,
    vacuum_every: u64,
    max_tablets: u64,
    inserts: Arc<AtomicU64>,
    snappy: bool,
}

impl FireboltSink {
    pub fn new(url: String, cli: &crate::cli::Cli) -> anyhow::Result<Self> {
        if cli.fb_stage != "upload" {
            anyhow::bail!("--fb-stage {} is not implemented in this build; only 'upload' (inline parquet) is", cli.fb_stage);
        }
        Ok(Self {
            url: url.trim_end_matches('/').to_string(),
            client: client(Duration::from_secs(900)),
            vacuum_every: cli.fb_vacuum_every,
            max_tablets: cli.fb_max_tablets,
            inserts: Arc::new(AtomicU64::new(0)),
            snappy: cli.fb_parquet_snappy,
        })
    }

    async fn sql(client: &reqwest::Client, url: &str, sql: &str, timeout: Duration) -> anyhow::Result<Value> {
        let resp = client.post(format!("{url}/")).timeout(timeout).query(&[("output_format", "JSON_Compact")]).body(sql.to_string()).send().await?;
        let status = resp.status();
        let text = resp.text().await?;
        if !status.is_success() {
            anyhow::bail!("firebolt {}: {}", status, text.trim().chars().take(500).collect::<String>());
        }
        if text.trim().is_empty() {
            return Ok(json!({}));
        }
        Ok(serde_json::from_str(&text).unwrap_or_else(|_| json!({ "raw": text })))
    }

    async fn table_stats(client: &reqwest::Client, url: &str) -> anyhow::Result<Value> {
        let v = Self::sql(client, url, "SELECT * FROM information_schema.tables WHERE table_name = 'points'", Duration::from_secs(60)).await?;
        let meta: Vec<String> = v.get("meta").and_then(Value::as_array).map(|m| m.iter().filter_map(|c| c.get("name").and_then(Value::as_str)).map(|s| s.to_string()).collect()).unwrap_or_default();
        let row = v.pointer("/data/0").and_then(Value::as_array).cloned().unwrap_or_default();
        let mut out = serde_json::Map::new();
        for (k, val) in meta.iter().zip(row.into_iter()) {
            if k.contains("tablet") || k.contains("rows") || k.contains("bytes") || k.contains("size") {
                out.insert(k.clone(), val);
            }
        }
        Ok(Value::Object(out))
    }

    fn tablets(stats: &Value) -> Option<u64> {
        stats.as_object()?.iter().find(|(k, _)| k.contains("tablet")).and_then(|(_, v)| v.as_u64().or_else(|| v.as_str().and_then(|s| s.parse().ok())))
    }
}

fn parquet_bytes(rb: &RecordBatch, snappy: bool) -> anyhow::Result<Vec<u8>> {
    let props = WriterProperties::builder().set_compression(if snappy { Compression::SNAPPY } else { Compression::UNCOMPRESSED }).set_max_row_group_row_count(Some(1 << 20)).build();
    let mut out = Vec::with_capacity(rb.num_rows() * 20);
    let mut w = ArrowWriter::try_new(&mut out, rb.schema(), Some(props))?;
    w.write(rb)?;
    w.close()?;
    Ok(out)
}

async fn upload(client: &reqwest::Client, url: &str, sql: &str, part: &str, bytes: Vec<u8>) -> Result<(), WriteError> {
    let form = reqwest::multipart::Form::new()
        .text("sql", sql.to_string())
        .part(part.to_string(), reqwest::multipart::Part::bytes(bytes).file_name(format!("{part}.parquet")).mime_str("application/octet-stream").map_err(|e| WriteError::fatal(e.to_string()))?);
    let resp = client.post(format!("{url}/")).query(&[("output_format", "JSON_Compact")]).multipart(form).send().await?;
    let status = resp.status();
    if status.is_success() {
        return Ok(());
    }
    let text = resp.text().await.unwrap_or_default();
    Err(WriteError { retryable: status.is_server_error(), msg: format!("{} {}", status, text.chars().take(400).collect::<String>()) })
}

#[async_trait]
impl Sink for FireboltSink {
    fn kind(&self) -> DbKind {
        DbKind::Firebolt
    }

    fn ack_semantics(&self) -> &'static str {
        "INSERT INTO points SELECT ... FROM READ_PARQUET('upload://batch') answered 200 (tablet committed)"
    }

    async fn setup(&self, reset: bool, timeout: Duration) -> anyhow::Result<()> {
        let c = self.client.clone();
        let url = self.url.clone();
        wait_until(timeout, || {
            let c = c.clone();
            let url = url.clone();
            async move { Self::sql(&c, &url, "SELECT 1", Duration::from_secs(10)).await.map(|v| v.get("data").is_some()).unwrap_or(false) }
        })
        .await?;
        if reset {
            Self::sql(&self.client, &self.url, "DROP TABLE IF EXISTS points", Duration::from_secs(300)).await?;
            Self::sql(&self.client, &self.url, "DROP TABLE IF EXISTS series", Duration::from_secs(300)).await?;
        }
        for stmt in super::split_statements(SCHEMA) {
            Self::sql(&self.client, &self.url, &stmt, Duration::from_secs(60)).await?;
        }
        Ok(())
    }

    async fn register_series(&self, cat: &SeriesCatalog) -> anyhow::Result<()> {
        let schema = Arc::new(Schema::new(vec![Field::new("series_id", DataType::Int64, false), Field::new("name", DataType::Utf8, false), Field::new("tags", DataType::Utf8, false)]));
        let n = cat.len();
        let mut id = 0u64;
        while id < n {
            let end = (id + 250_000).min(n);
            let ids: Vec<i64> = (id..end).map(|i| i as i64).collect();
            let names: Vec<&str> = (id..end).map(|i| cat.name_of(i)).collect();
            let tags: Vec<String> = (id..end)
                .map(|i| {
                    let m: serde_json::Map<String, Value> = cat.tags_of(i).into_iter().map(|(k, v)| (k.to_string(), Value::String(v.to_string()))).collect();
                    Value::Object(m).to_string()
                })
                .collect();
            let rb = RecordBatch::try_new(schema.clone(), vec![Arc::new(Int64Array::from(ids)), Arc::new(StringArray::from(names)), Arc::new(StringArray::from(tags))])?;
            let bytes = parquet_bytes(&rb, self.snappy)?;
            upload(&self.client, &self.url, "INSERT INTO series SELECT series_id, name, tags FROM READ_PARQUET('upload://series')", "series", bytes)
                .await
                .map_err(|e| anyhow::anyhow!(e.msg))?;
            id = end;
        }
        Ok(())
    }

    async fn writer(&self, idx: usize) -> anyhow::Result<Box<dyn SinkWriter>> {
        Ok(Box::new(FbWriter { sink: self.clone(), maintainer: idx == 0 }))
    }

    async fn reader(&self) -> anyhow::Result<Box<dyn SinkReader>> {
        Ok(Box::new(FbReader { client: client(Duration::from_secs(120)), url: self.url.clone() }))
    }

    async fn health(&self) -> anyhow::Result<Value> {
        let mut v = Self::table_stats(&self.client, &self.url).await.unwrap_or_else(|e| json!({ "error": e.to_string() }));
        if let Some(o) = v.as_object_mut() {
            o.insert("db".into(), json!("firebolt"));
            o.insert("inserts".into(), json!(self.inserts.load(Ordering::Relaxed)));
        }
        Ok(v)
    }

    async fn is_reachable(&self) -> bool {
        Self::sql(&self.client, &self.url, "SELECT 1", Duration::from_secs(10)).await.is_ok()
    }

    async fn settle(&self, deadline: Instant) -> SettleReport {
        let mut rep = SettleReport::default();
        let mut prev = Self::table_stats(&self.client, &self.url).await.ok().and_then(|s| Self::tablets(&s));
        for pass in 0..6 {
            let t0 = Instant::now();
            let r = Self::sql(&self.client, &self.url, "VACUUM points", deadline.saturating_duration_since(Instant::now()).max(Duration::from_secs(10))).await;
            let now = Self::table_stats(&self.client, &self.url).await.ok().and_then(|s| Self::tablets(&s));
            rep.step(&format!("vacuum_{pass}"), t0, r.is_ok(), format!("tablets {prev:?} -> {now:?}{}", r.err().map(|e| format!(" ({e})")).unwrap_or_default()));
            if now.is_some() && now >= prev || Instant::now() > deadline {
                rep.settled = now.is_some() && now >= prev;
                return rep;
            }
            prev = now;
        }
        rep.settled = true;
        rep
    }

    async fn db_size(&self) -> anyhow::Result<DbSize> {
        let s = Self::table_stats(&self.client, &self.url).await?;
        let get = |key: &str| s.get(key).and_then(|v| v.as_u64().or_else(|| v.as_str().and_then(|x| x.parse().ok())));
        Ok(DbSize { compressed_bytes: get("compressed_bytes"), uncompressed_bytes: get("uncompressed_bytes"), rows: get("number_of_rows") })
    }

    fn disk_class(&self, rel: &str) -> &'static str {
        if rel.contains("tablet") || rel.contains("data") {
            "data"
        } else if rel.contains("tmp") || rel.contains("spill") {
            "tmp"
        } else {
            "other"
        }
    }

    fn settings(&self) -> Value {
        json!({ "stage": "upload:// multipart parquet", "parquet_snappy": self.snappy, "vacuum_every_inserts": self.vacuum_every, "max_tablets": self.max_tablets })
    }
}

struct FbWriter {
    sink: FireboltSink,
    maintainer: bool,
}

#[async_trait]
impl SinkWriter for FbWriter {
    async fn write(&mut self, batch: &Batch, _cat: &SeriesCatalog) -> Result<Ack, WriteError> {
        let schema = Arc::new(Schema::new(vec![
            Field::new("series_id", DataType::Int64, false),
            Field::new("ts", DataType::Timestamp(TimeUnit::Microsecond, None), false),
            Field::new("value", DataType::Float64, false),
        ]));
        let rb = RecordBatch::try_new(
            schema,
            vec![
                Arc::new(Int64Array::from(batch.series_id.iter().map(|v| *v as i64).collect::<Vec<i64>>())),
                Arc::new(TimestampMicrosecondArray::from(batch.ts_ms.iter().map(|ms| ms * 1000).collect::<Vec<i64>>())),
                Arc::new(Float64Array::from(batch.value.clone())),
            ],
        )
        .map_err(|e| WriteError::fatal(e.to_string()))?;
        let bytes = parquet_bytes(&rb, self.sink.snappy).map_err(|e| WriteError::fatal(e.to_string()))?;
        let sent = bytes.len() as u64;
        upload(&self.sink.client, &self.sink.url, "INSERT INTO points SELECT series_id, ts, value FROM READ_PARQUET('upload://batch')", "batch", bytes).await?;
        let n = self.sink.inserts.fetch_add(1, Ordering::Relaxed) + 1;
        let mut maintenance = Vec::new();
        if self.maintainer {
            let due = self.sink.vacuum_every > 0 && n % self.sink.vacuum_every == 0;
            let over = if !due && n % 5 == 0 {
                FireboltSink::table_stats(&self.sink.client, &self.sink.url).await.ok().and_then(|s| FireboltSink::tablets(&s)).map(|t| t > self.sink.max_tablets).unwrap_or(false)
            } else {
                false
            };
            if due || over {
                let t0 = Instant::now();
                if let Err(e) = FireboltSink::sql(&self.sink.client, &self.sink.url, "VACUUM points", Duration::from_secs(1800)).await {
                    return Err(WriteError::retryable(format!("vacuum failed: {e}")));
                }
                maintenance.push(MaintEvent { kind: "vacuum".into(), ms: t0.elapsed().as_millis() as u64 });
            }
        }
        Ok(Ack { bytes_sent: sent, maintenance })
    }
}

struct FbReader {
    client: reqwest::Client,
    url: String,
}

#[async_trait]
impl SinkReader for FbReader {
    async fn visible_max_ts(&mut self) -> anyhow::Result<Option<i64>> {
        let v = FireboltSink::sql(&self.client, &self.url, &format!("SELECT CAST(max(ts) AS TEXT) FROM points WHERE series_id = {SENTINEL_ID}"), Duration::from_secs(60)).await?;
        Ok(v.pointer("/data/0/0").and_then(Value::as_str).and_then(parse_ts_ms))
    }

    async fn run_query(&mut self, q: &QueryInstance, deadline: Duration) -> QueryOutcome {
        let sql = render(Dialect::Firebolt, q);
        let t0 = Instant::now();
        match FireboltSink::sql(&self.client, &self.url, &sql, deadline + Duration::from_secs(2)).await {
            Ok(v) => {
                let rows = v.get("rows").and_then(Value::as_u64).or_else(|| v.get("data").and_then(Value::as_array).map(|a| a.len() as u64)).unwrap_or(0);
                QueryOutcome::ok(q.id, t0.elapsed(), rows, q.params.expected_rows(q.id, q.t_ms))
            }
            Err(_) if t0.elapsed() >= deadline => QueryOutcome::timeout(q.id, t0.elapsed()),
            Err(e) => QueryOutcome::error(q.id, t0.elapsed(), e.to_string()),
        }
    }
}
