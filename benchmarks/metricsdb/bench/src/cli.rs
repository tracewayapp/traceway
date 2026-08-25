use std::path::PathBuf;
use std::time::Duration;

use clap::{ArgAction, Parser};

use crate::sink::DbKind;

#[derive(Parser, Debug, Clone)]
#[command(name = "metricsdb-bench", version, about = "Ingest, disk and query benchmark for OTel metrics stores")]
pub struct Cli {
    #[arg(long)]
    pub db: Option<DbKind>,
    #[arg(long)]
    pub url: Option<String>,

    #[arg(long, default_value_t = 10_000_000_000)]
    pub points: u64,
    #[arg(long, default_value_t = 1_000_000)]
    pub series: u64,
    #[arg(long, default_value = "10s", value_parser = humantime::parse_duration)]
    pub interval: Duration,
    #[arg(long, default_value = "2026-01-01T00:00:00Z")]
    pub sim_start: String,
    #[arg(long, default_value_t = 42)]
    pub seed: u64,

    #[arg(long, default_value_t = 2)]
    pub gen_threads: usize,
    #[arg(long)]
    pub writers: Option<usize>,
    #[arg(long)]
    pub batch_points: Option<usize>,
    #[arg(long, default_value_t = 0)]
    pub rate: u64,
    #[arg(long, default_value_t = 5)]
    pub max_retries: u32,
    #[arg(long, default_value = "60s", value_parser = humantime::parse_duration)]
    pub warmup: Duration,
    #[arg(long, default_value = "225m", value_parser = humantime::parse_duration)]
    pub max_ingest: Duration,
    #[arg(long, default_value = "30m", value_parser = humantime::parse_duration)]
    pub max_settle: Duration,
    #[arg(long, default_value = "30m", value_parser = humantime::parse_duration)]
    pub max_cold: Duration,
    #[arg(long, default_value = "10m", value_parser = humantime::parse_duration)]
    pub drain_timeout: Duration,

    #[arg(long, default_value = "15s", value_parser = humantime::parse_duration)]
    pub query_interval: Duration,
    #[arg(long, default_value = "30s", value_parser = humantime::parse_duration)]
    pub query_deadline: Duration,
    #[arg(long, default_value_t = 5000)]
    pub query_slow_ms: u64,
    #[arg(long, default_value_t = 3)]
    pub cold_runs: usize,
    #[arg(long)]
    pub no_queries: bool,
    #[arg(long)]
    pub no_cold: bool,

    #[arg(long, default_value = "120s", value_parser = humantime::parse_duration)]
    pub lag_threshold: Duration,
    #[arg(long, default_value_t = 0.5)]
    pub degrade_ratio: f64,
    #[arg(long, default_value_t = 0.01)]
    pub write_err_ratio: f64,
    #[arg(long, default_value = "5m", value_parser = humantime::parse_duration)]
    pub unusable_grace: Duration,

    #[arg(long, default_value = "30s", value_parser = humantime::parse_duration)]
    pub disk_interval: Duration,
    #[arg(long, default_value = ".")]
    pub data_dir: PathBuf,
    #[arg(long)]
    pub cgroup: Option<PathBuf>,
    #[arg(long)]
    pub db_container: Option<String>,
    #[arg(long)]
    pub cold_hook: Option<PathBuf>,

    #[arg(long, default_value = "result.json")]
    pub out: PathBuf,
    #[arg(long, default_value = "local")]
    pub tier: String,
    #[arg(long)]
    pub run_id: Option<String>,
    #[arg(long, default_value = "10s", value_parser = humantime::parse_duration)]
    pub window: Duration,
    #[arg(long, default_value_t = true, action = ArgAction::Set)]
    pub reset: bool,
    #[arg(long, default_value = "120s", value_parser = humantime::parse_duration)]
    pub setup_timeout: Duration,
    /// Run the generator against a null sink for this many rounds and print the fingerprint.
    #[arg(long)]
    pub verify_gen: Option<u64>,

    #[arg(long, default_value = "bench")]
    pub ch_database: String,

    #[arg(long)]
    pub duckdb_path: Option<PathBuf>,
    #[arg(long)]
    pub duckdb_threads: Option<usize>,
    #[arg(long, default_value = "12GB")]
    pub duckdb_memory_limit: String,
    #[arg(long, default_value = "1GB")]
    pub duckdb_checkpoint_threshold: String,
    #[arg(long, default_value_t = 0)]
    pub duckdb_checkpoint_every: u64,

    #[arg(long, default_value_t = 20)]
    pub fb_vacuum_every: u64,
    #[arg(long, default_value_t = 50)]
    pub fb_max_tablets: u64,
    #[arg(long, default_value = "upload")]
    pub fb_stage: String,
    #[arg(long, default_value_t = false, action = ArgAction::Set)]
    pub fb_parquet_snappy: bool,
    #[arg(long)]
    pub fb_s3_endpoint: Option<String>,
    #[arg(long)]
    pub fb_s3_bucket: Option<String>,
    #[arg(long)]
    pub fb_s3_key: Option<String>,
    #[arg(long)]
    pub fb_s3_secret: Option<String>,
}

impl Cli {
    pub fn sim_start_ms(&self) -> i64 {
        crate::util::parse_ts_ms(&self.sim_start).unwrap_or_else(|| panic!("--sim-start must look like 2026-01-01T00:00:00Z, got {}", self.sim_start))
    }
}
