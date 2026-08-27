use std::str::FromStr;
use std::time::{Duration, Instant};

use async_trait::async_trait;
use serde::Serialize;

use crate::model::catalog::SeriesCatalog;
use crate::pipeline::batch::Batch;
use crate::pipeline::writer::MaintEvent;
use crate::queries::{QueryInstance, QueryOutcome};

pub mod http;
#[cfg(feature = "clickhouse")]
pub mod clickhouse;
#[cfg(feature = "victoria")]
pub mod victoria;
#[cfg(feature = "duckdb")]
pub mod duckdb;
#[cfg(feature = "firebolt")]
pub mod firebolt;

#[derive(Clone, Copy, Debug, PartialEq, Eq, Serialize)]
#[serde(rename_all = "kebab-case")]
pub enum DbKind {
    Victoriametrics,
    Clickhouse,
    ClickhouseMap,
    Duckdb,
    Firebolt,
    FireboltS3,
}

impl DbKind {
    pub fn all() -> &'static [DbKind] {
        &[DbKind::Victoriametrics, DbKind::Clickhouse, DbKind::ClickhouseMap, DbKind::Duckdb, DbKind::Firebolt, DbKind::FireboltS3]
    }
    pub fn name(self) -> &'static str {
        match self {
            DbKind::Victoriametrics => "victoriametrics",
            DbKind::Clickhouse => "clickhouse",
            DbKind::ClickhouseMap => "clickhouse-map",
            DbKind::Duckdb => "duckdb",
            DbKind::Firebolt => "firebolt",
            DbKind::FireboltS3 => "firebolt-s3",
        }
    }
    pub fn family(self) -> &'static str {
        match self {
            DbKind::Clickhouse | DbKind::ClickhouseMap => "clickhouse",
            DbKind::Firebolt | DbKind::FireboltS3 => "firebolt",
            other => other.name(),
        }
    }
    pub fn variant(self) -> &'static str {
        match self {
            DbKind::ClickhouseMap => "map",
            DbKind::FireboltS3 => "s3-disaggregated",
            DbKind::Clickhouse | DbKind::Duckdb | DbKind::Firebolt => "normalized",
            DbKind::Victoriametrics => "native",
        }
    }
    pub fn default_url(self) -> &'static str {
        match self {
            DbKind::Victoriametrics => "http://127.0.0.1:8428",
            DbKind::Clickhouse | DbKind::ClickhouseMap => "http://127.0.0.1:8123",
            DbKind::Duckdb => "",
            DbKind::Firebolt => "http://127.0.0.1:3473",
            DbKind::FireboltS3 => "http://127.0.0.1:3473",
        }
    }
    pub fn defaults(self) -> SinkDefaults {
        match self {
            DbKind::Victoriametrics => SinkDefaults { writers: 8, batch_points: 20_000 },
            DbKind::Clickhouse => SinkDefaults { writers: 4, batch_points: 500_000 },
            DbKind::ClickhouseMap => SinkDefaults { writers: 4, batch_points: 200_000 },
            DbKind::Duckdb => SinkDefaults { writers: 1, batch_points: 4_000_000 },
            DbKind::Firebolt => SinkDefaults { writers: 2, batch_points: 1_000_000 },
            DbKind::FireboltS3 => SinkDefaults { writers: 2, batch_points: 1_000_000 },
        }
    }
}

impl FromStr for DbKind {
    type Err = String;
    fn from_str(s: &str) -> Result<Self, Self::Err> {
        DbKind::all()
            .iter()
            .copied()
            .find(|k| k.name() == s)
            .ok_or_else(|| format!("unknown db '{s}', expected one of victoriametrics, clickhouse, clickhouse-map, duckdb, firebolt, firebolt-s3"))
    }
}

#[derive(Clone, Copy, Debug)]
pub struct SinkDefaults {
    pub writers: usize,
    pub batch_points: usize,
}

#[derive(Debug, Default)]
pub struct Ack {
    pub bytes_sent: u64,
    pub maintenance: Vec<MaintEvent>,
}

#[derive(Debug)]
pub struct WriteError {
    pub retryable: bool,
    pub msg: String,
}

impl WriteError {
    pub fn retryable(msg: impl Into<String>) -> Self {
        Self { retryable: true, msg: msg.into() }
    }
    pub fn fatal(msg: impl Into<String>) -> Self {
        Self { retryable: false, msg: msg.into() }
    }
}

impl From<reqwest::Error> for WriteError {
    fn from(e: reqwest::Error) -> Self {
        Self { retryable: e.is_connect() || e.is_timeout() || e.is_request() || e.is_body(), msg: e.to_string() }
    }
}

impl From<anyhow::Error> for WriteError {
    fn from(e: anyhow::Error) -> Self {
        Self { retryable: false, msg: format!("{e:#}") }
    }
}

#[derive(Clone, Debug, Serialize)]
pub struct SettleStep {
    pub name: String,
    pub ms: u64,
    pub ok: bool,
    pub detail: String,
}

#[derive(Clone, Debug, Default, Serialize)]
pub struct SettleReport {
    pub steps: Vec<SettleStep>,
    pub settled: bool,
}

impl SettleReport {
    pub fn step(&mut self, name: &str, started: Instant, ok: bool, detail: impl Into<String>) {
        self.steps.push(SettleStep { name: name.to_string(), ms: started.elapsed().as_millis() as u64, ok, detail: detail.into() });
    }
}

#[derive(Clone, Debug, Default, Serialize)]
pub struct DbSize {
    pub compressed_bytes: Option<u64>,
    pub uncompressed_bytes: Option<u64>,
    pub rows: Option<u64>,
}

#[async_trait]
pub trait Sink: Send + Sync {
    #[allow(dead_code)]
    fn kind(&self) -> DbKind;
    fn ack_semantics(&self) -> &'static str;
    async fn setup(&self, reset: bool, timeout: Duration) -> anyhow::Result<()>;
    async fn register_series(&self, cat: &SeriesCatalog) -> anyhow::Result<()>;
    async fn writer(&self, idx: usize) -> anyhow::Result<Box<dyn SinkWriter>>;
    async fn reader(&self) -> anyhow::Result<Box<dyn SinkReader>>;
    async fn health(&self) -> anyhow::Result<serde_json::Value>;
    async fn is_reachable(&self) -> bool;
    async fn settle(&self, deadline: Instant) -> SettleReport;
    async fn db_size(&self) -> anyhow::Result<DbSize>;
    /// Close in-process handles before the cold phase; network sinks do nothing here.
    async fn make_cold(&self) -> anyhow::Result<()> {
        Ok(())
    }
    async fn reopen(&self) -> anyhow::Result<()> {
        Ok(())
    }
    fn disk_class(&self, rel_path: &str) -> &'static str;
    fn settings(&self) -> serde_json::Value;
}

#[async_trait]
pub trait SinkWriter: Send {
    async fn write(&mut self, batch: &Batch, cat: &SeriesCatalog) -> Result<Ack, WriteError>;
    async fn finish(&mut self) -> anyhow::Result<()> {
        Ok(())
    }
}

#[async_trait]
pub trait SinkReader: Send {
    async fn visible_max_ts(&mut self) -> anyhow::Result<Option<i64>>;
    async fn run_query(&mut self, q: &QueryInstance, deadline: Duration) -> QueryOutcome;
}

pub fn make_sink(cli: &crate::cli::Cli) -> anyhow::Result<Box<dyn Sink>> {
    let kind = cli.db.ok_or_else(|| anyhow::anyhow!("--db is required"))?;
    let url = cli.url.clone().unwrap_or_else(|| kind.default_url().to_string());
    match kind {
        #[cfg(feature = "victoria")]
        DbKind::Victoriametrics => Ok(Box::new(victoria::VictoriaSink::new(url, cli)?)),
        #[cfg(feature = "clickhouse")]
        DbKind::Clickhouse | DbKind::ClickhouseMap => Ok(Box::new(clickhouse::ClickhouseSink::new(kind, url, cli)?)),
        #[cfg(feature = "duckdb")]
        DbKind::Duckdb => Ok(Box::new(duckdb::DuckSink::new(cli)?)),
        #[cfg(feature = "firebolt")]
        DbKind::Firebolt | DbKind::FireboltS3 => Ok(Box::new(firebolt::FireboltSink::new(url, cli)?)),
        #[allow(unreachable_patterns)]
        other => anyhow::bail!("this binary was built without support for {}", other.name()),
    }
}

pub fn split_statements(sql: &str) -> Vec<String> {
    let without_comments = sql.lines().filter(|l| !l.trim_start().starts_with("--")).collect::<Vec<_>>().join("\n");
    without_comments.split(';').map(|s| s.trim().to_string()).filter(|s| !s.is_empty()).collect()
}

#[cfg(test)]
mod tests {
    #[test]
    fn comments_with_semicolons_do_not_split() {
        let sql = "-- a; b\nCREATE TABLE t (x INT);\n-- c; d\nCREATE TABLE u (y INT)";
        let v = super::split_statements(sql);
        assert_eq!(v, vec!["CREATE TABLE t (x INT)", "CREATE TABLE u (y INT)"]);
    }
}
