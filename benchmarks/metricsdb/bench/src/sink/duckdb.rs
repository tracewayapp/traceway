use std::io::Write;
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::{Arc, Mutex};
use std::time::{Duration, Instant};

use arrow::array::{Float64Array, TimestampMicrosecondArray, UInt64Array};
use arrow::datatypes::{DataType, Field, Schema, TimeUnit};
use arrow::record_batch::RecordBatch;
use async_trait::async_trait;
use duckdb::Connection;
use serde_json::{json, Value};

use super::{Ack, DbKind, DbSize, SettleReport, Sink, SinkReader, SinkWriter, WriteError};
use crate::model::catalog::{SeriesCatalog, SENTINEL_ID};
use crate::pipeline::batch::Batch;
use crate::pipeline::writer::MaintEvent;
use crate::queries::sql::{render, Dialect};
use crate::queries::{QueryInstance, QueryOutcome};

const SCHEMA: &str = include_str!("../../../db/duckdb/schema.sql");

pub struct DuckSink {
    path: PathBuf,
    threads: usize,
    memory_limit: String,
    checkpoint_threshold: String,
    checkpoint_every: u64,
    root: Mutex<Option<Connection>>,
    batches: Arc<AtomicU64>,
}

impl DuckSink {
    pub fn new(cli: &crate::cli::Cli) -> anyhow::Result<Self> {
        let path = cli.duckdb_path.clone().unwrap_or_else(|| cli.data_dir.join("points.duckdb"));
        let threads = cli.duckdb_threads.unwrap_or_else(|| std::thread::available_parallelism().map(|n| n.get()).unwrap_or(4).saturating_sub(2).max(1));
        Ok(Self {
            path,
            threads,
            memory_limit: cli.duckdb_memory_limit.clone(),
            checkpoint_threshold: cli.duckdb_checkpoint_threshold.clone(),
            checkpoint_every: cli.duckdb_checkpoint_every,
            root: Mutex::new(None),
            batches: Arc::new(AtomicU64::new(0)),
        })
    }

    fn wal_path(&self) -> PathBuf {
        PathBuf::from(format!("{}.wal", self.path.display()))
    }

    fn clone_conn(&self) -> anyhow::Result<Connection> {
        let g = self.root.lock().unwrap();
        let c = g.as_ref().ok_or_else(|| anyhow::anyhow!("duckdb not open"))?;
        Ok(c.try_clone()?)
    }

    fn open(&self) -> anyhow::Result<()> {
        let conn = Connection::open(&self.path)?;
        let temp = self.path.parent().map(|p| p.join("tmp")).unwrap_or_else(|| PathBuf::from("tmp"));
        std::fs::create_dir_all(&temp)?;
        conn.execute_batch(&format!(
            "SET threads = {}; SET memory_limit = '{}'; SET checkpoint_threshold = '{}'; SET temp_directory = '{}'; SET preserve_insertion_order = false;",
            self.threads,
            self.memory_limit,
            self.checkpoint_threshold,
            temp.display()
        ))?;
        *self.root.lock().unwrap() = Some(conn);
        Ok(())
    }

    fn file_sizes(&self) -> (u64, u64) {
        let db = std::fs::metadata(&self.path).map(|m| m.len()).unwrap_or(0);
        let wal = std::fs::metadata(self.wal_path()).map(|m| m.len()).unwrap_or(0);
        (db, wal)
    }
}

fn points_schema() -> Arc<Schema> {
    Arc::new(Schema::new(vec![
        Field::new("series_id", DataType::UInt64, false),
        Field::new("ts", DataType::Timestamp(TimeUnit::Microsecond, None), false),
        Field::new("value", DataType::Float64, false),
    ]))
}

pub fn to_record_batch(b: &Batch) -> RecordBatch {
    RecordBatch::try_new(
        points_schema(),
        vec![
            Arc::new(UInt64Array::from(b.series_id.clone())),
            Arc::new(TimestampMicrosecondArray::from(b.ts_ms.iter().map(|ms| ms * 1000).collect::<Vec<i64>>())),
            Arc::new(Float64Array::from(b.value.clone())),
        ],
    )
    .expect("record batch")
}

#[async_trait]
impl Sink for DuckSink {
    fn kind(&self) -> DbKind {
        DbKind::Duckdb
    }

    fn ack_semantics(&self) -> &'static str {
        "Appender flush + commit returned (rows in the WAL and visible; durable at the next checkpoint)"
    }

    async fn setup(&self, reset: bool, _timeout: Duration) -> anyhow::Result<()> {
        if reset {
            let _ = std::fs::remove_file(&self.path);
            let _ = std::fs::remove_file(self.wal_path());
        }
        if let Some(p) = self.path.parent() {
            std::fs::create_dir_all(p)?;
        }
        self.open()?;
        let conn = self.clone_conn()?;
        tokio::task::block_in_place(|| conn.execute_batch(SCHEMA))?;
        Ok(())
    }

    async fn register_series(&self, cat: &SeriesCatalog) -> anyhow::Result<()> {
        let jsonl = self.path.parent().unwrap_or(&self.path).join("series.jsonl");
        {
            let mut f = std::io::BufWriter::new(std::fs::File::create(&jsonl)?);
            for id in 0..cat.len() {
                let tags = cat.tags_of(id);
                let keys: Vec<&str> = tags.iter().map(|(k, _)| *k).collect();
                let vals: Vec<&str> = tags.iter().map(|(_, v)| *v).collect();
                serde_json::to_writer(&mut f, &json!({ "series_id": id, "name": cat.name_of(id), "keys": keys, "vals": vals }))?;
                f.write_all(b"\n")?;
            }
        }
        let conn = self.clone_conn()?;
        let sql = format!(
            "INSERT INTO series SELECT series_id, name, map(keys, vals) FROM read_json('{}', format = 'newline_delimited', columns = {{series_id: 'UBIGINT', name: 'VARCHAR', keys: 'VARCHAR[]', vals: 'VARCHAR[]'}})",
            jsonl.display()
        );
        tokio::task::block_in_place(|| conn.execute_batch(&sql))?;
        let _ = std::fs::remove_file(&jsonl);
        Ok(())
    }

    async fn writer(&self, _idx: usize) -> anyhow::Result<Box<dyn SinkWriter>> {
        Ok(Box::new(DuckWriter { conn: self.clone_conn()?, checkpoint_every: self.checkpoint_every, batches: Arc::clone(&self.batches) }))
    }

    async fn reader(&self) -> anyhow::Result<Box<dyn SinkReader>> {
        Ok(Box::new(DuckReader { conn: self.clone_conn()? }))
    }

    async fn health(&self) -> anyhow::Result<Value> {
        let (db, wal) = self.file_sizes();
        let conn = self.clone_conn()?;
        let mem: Option<String> = tokio::task::block_in_place(|| conn.query_row("SELECT memory_usage FROM pragma_database_size()", [], |r| r.get(0)).ok());
        Ok(json!({ "db": "duckdb", "db_file_bytes": db, "wal_bytes": wal, "memory_usage": mem }))
    }

    async fn is_reachable(&self) -> bool {
        self.root.lock().unwrap().is_some()
    }

    async fn settle(&self, _deadline: Instant) -> SettleReport {
        let mut rep = SettleReport::default();
        let conn = match self.clone_conn() {
            Ok(c) => c,
            Err(e) => {
                rep.step("checkpoint", Instant::now(), false, e.to_string());
                return rep;
            }
        };
        let t0 = Instant::now();
        let r = tokio::task::block_in_place(|| conn.execute_batch("CHECKPOINT"));
        rep.step("checkpoint", t0, r.is_ok(), r.err().map(|e| e.to_string()).unwrap_or_default());
        let (_, wal) = self.file_sizes();
        if wal > 0 {
            let t1 = Instant::now();
            let r = tokio::task::block_in_place(|| conn.execute_batch("FORCE CHECKPOINT"));
            rep.step("force_checkpoint", t1, r.is_ok(), format!("wal_bytes_before={wal}"));
        }
        let (_, wal) = self.file_sizes();
        rep.settled = wal == 0;
        rep
    }

    async fn db_size(&self) -> anyhow::Result<DbSize> {
        let (db, wal) = self.file_sizes();
        let conn = self.clone_conn()?;
        let rows: Option<u64> = tokio::task::block_in_place(|| conn.query_row("SELECT count(*) FROM points", [], |r| r.get::<_, i64>(0)).ok().map(|v| v as u64));
        Ok(DbSize { compressed_bytes: Some(db + wal), uncompressed_bytes: None, rows })
    }

    async fn make_cold(&self) -> anyhow::Result<()> {
        if let Some(c) = self.root.lock().unwrap().take() {
            c.close().map_err(|(_, e)| anyhow::anyhow!(e))?;
        }
        Ok(())
    }

    async fn reopen(&self) -> anyhow::Result<()> {
        self.open()
    }

    fn disk_class(&self, rel: &str) -> &'static str {
        if rel.ends_with(".wal") {
            "wal"
        } else if rel.ends_with(".duckdb") {
            "data"
        } else if rel.contains("tmp") {
            "tmp"
        } else {
            "other"
        }
    }

    fn settings(&self) -> Value {
        json!({ "path": self.path, "threads": self.threads, "memory_limit": self.memory_limit, "checkpoint_threshold": self.checkpoint_threshold, "checkpoint_every_batches": self.checkpoint_every, "preserve_insertion_order": false, "write_path": "Appender (arrow), batches sorted by (series_id, ts)" })
    }
}

struct DuckWriter {
    conn: Connection,
    checkpoint_every: u64,
    batches: Arc<AtomicU64>,
}

#[async_trait]
impl SinkWriter for DuckWriter {
    async fn write(&mut self, batch: &Batch, _cat: &SeriesCatalog) -> Result<Ack, WriteError> {
        let rb = to_record_batch(&batch.sorted_by_series_ts());
        let bytes = batch.raw_bytes();
        let mut maintenance = Vec::new();
        let r: anyhow::Result<()> = tokio::task::block_in_place(|| {
            let mut app = self.conn.appender("points")?;
            app.append_record_batch(rb)?;
            app.flush()?;
            drop(app);
            let n = self.batches.fetch_add(1, Ordering::Relaxed) + 1;
            if self.checkpoint_every > 0 && n % self.checkpoint_every == 0 {
                let t0 = Instant::now();
                self.conn.execute_batch("CHECKPOINT")?;
                maintenance.push(MaintEvent { kind: "checkpoint".into(), ms: t0.elapsed().as_millis() as u64 });
            }
            Ok(())
        });
        match r {
            Ok(()) => Ok(Ack { bytes_sent: bytes, maintenance }),
            Err(e) => Err(WriteError::fatal(format!("{e:#}"))),
        }
    }
}

struct DuckReader {
    conn: Connection,
}

#[async_trait]
impl SinkReader for DuckReader {
    async fn visible_max_ts(&mut self) -> anyhow::Result<Option<i64>> {
        let v: Option<i64> = tokio::task::block_in_place(|| {
            self.conn.query_row(&format!("SELECT epoch_ms(max(ts)) FROM points WHERE series_id = {SENTINEL_ID}"), [], |r| r.get::<_, Option<i64>>(0))
        })?;
        Ok(v)
    }

    async fn run_query(&mut self, q: &QueryInstance, deadline: Duration) -> QueryOutcome {
        let sql = render(Dialect::DuckDb, q);
        let t0 = Instant::now();
        let handle = self.conn.interrupt_handle();
        let timer = tokio::spawn(async move {
            tokio::time::sleep(deadline).await;
            handle.interrupt();
        });
        let r: Result<u64, duckdb::Error> = tokio::task::block_in_place(|| {
            let mut stmt = self.conn.prepare(&sql)?;
            let mut rows = stmt.query([])?;
            let mut n = 0u64;
            while rows.next()?.is_some() {
                n += 1;
            }
            Ok(n)
        });
        timer.abort();
        let elapsed = t0.elapsed();
        match r {
            Ok(rows) => QueryOutcome::ok(q.id, elapsed, rows, q.params.expected_rows(q.id, q.t_ms)),
            Err(e) if elapsed >= deadline || e.to_string().to_lowercase().contains("interrupt") => QueryOutcome::timeout(q.id, elapsed),
            Err(e) => QueryOutcome::error(q.id, elapsed, e.to_string()),
        }
    }
}
