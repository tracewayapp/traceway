use std::path::PathBuf;
use std::time::Duration;

use clap::{ArgAction, Parser, ValueEnum};

use crate::sink::DbKind;

#[derive(Clone, Copy, Debug, PartialEq, Eq, ValueEnum)]
pub enum Mode {
    /// Offer unbounded load; the acknowledged rate is whatever the store accepts.
    Saturate,
    /// Fixed-rate steps only; report the highest rate whose debt stays flat.
    Ramp,
    /// Ramp, then fill at a fraction of the highest passing rate.
    RampThenFill,
}

impl Mode {
    pub fn name(self) -> &'static str {
        match self {
            Mode::Saturate => "saturate",
            Mode::Ramp => "ramp",
            Mode::RampThenFill => "ramp-then-fill",
        }
    }
}

/// "250k,500k,1M,2M" -> points per second.
pub fn parse_rate(s: &str) -> Result<u64, String> {
    let t = s.trim().to_ascii_lowercase();
    let (num, mult) = if let Some(n) = t.strip_suffix('k') {
        (n, 1_000.0)
    } else if let Some(n) = t.strip_suffix('m') {
        (n, 1_000_000.0)
    } else if let Some(n) = t.strip_suffix('g') {
        (n, 1_000_000_000.0)
    } else {
        (t.as_str(), 1.0)
    };
    let f: f64 = num.trim().parse().map_err(|_| format!("bad rate '{s}' (use 250k, 1M, 2000000)"))?;
    Ok((f * mult) as u64)
}

pub fn parse_rate_list(s: &str) -> Result<Vec<u64>, String> {
    let v: Result<Vec<u64>, String> = s.split(',').filter(|x| !x.trim().is_empty()).map(parse_rate).collect();
    let v = v?;
    if v.is_empty() {
        return Err("ramp ladder is empty".into());
    }
    if v.windows(2).any(|w| w[1] <= w[0]) {
        return Err("ramp ladder must be strictly increasing".into());
    }
    Ok(v)
}

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
    #[arg(long, value_parser = parse_rate, default_value = "0")]
    pub rate: u64,
    #[arg(long, value_enum, default_value_t = Mode::RampThenFill)]
    pub mode: Mode,
    #[arg(long, default_value = "250k,500k,1M,2M,4M,8M")]
    pub ramp_rates: String,
    #[arg(long, default_value = "8m", value_parser = humantime::parse_duration)]
    pub step: Duration,
    #[arg(long, default_value = "60s", value_parser = humantime::parse_duration)]
    pub ramp_in: Duration,
    #[arg(long, default_value_t = 1)]
    pub ramp_bisect: u32,
    #[arg(long, default_value_t = 0.9)]
    pub fill_fraction: f64,
    #[arg(long, default_value = "10m", value_parser = humantime::parse_duration)]
    pub debt_window: Duration,
    #[arg(long, default_value_t = 1.3)]
    pub debt_growth: f64,
    /// Growth below this many units (parts, tablets, WAL MB) is noise, whatever the ratio.
    #[arg(long, default_value_t = 10.0)]
    pub debt_min_delta: f64,
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

    /// Gateway-routed engine names (firebolt-s3 / operator deployments):
    /// writes, queries and VACUUM can each hit a different engine via the
    /// X-Firebolt-Engine header. Unset = single-node, no header.
    #[arg(long)]
    pub fb_write_engine: Option<String>,
    #[arg(long)]
    pub fb_query_engine: Option<String>,
    #[arg(long)]
    pub fb_maint_engine: Option<String>,

    /// Firebolt schema variant: "zstd" (table-level ZSTD, the single-node
    /// baseline schema) or "codecs" (per-column Delta/DoubleDelta/Gorilla
    /// chained with ZSTD, db/firebolt-s3/schema.sql).
    #[arg(long, default_value = "zstd")]
    pub fb_schema: String,
}

impl Cli {
    pub fn ramp_ladder(&self) -> anyhow::Result<Vec<u64>> {
        parse_rate_list(&self.ramp_rates).map_err(|e| anyhow::anyhow!("--ramp-rates: {e}"))
    }

    pub fn sim_start_ms(&self) -> i64 {
        crate::util::parse_ts_ms(&self.sim_start).unwrap_or_else(|| panic!("--sim-start must look like 2026-01-01T00:00:00Z, got {}", self.sim_start))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn rates_parse() {
        assert_eq!(parse_rate("250k").unwrap(), 250_000);
        assert_eq!(parse_rate("1M").unwrap(), 1_000_000);
        assert_eq!(parse_rate("1.5m").unwrap(), 1_500_000);
        assert_eq!(parse_rate("2000000").unwrap(), 2_000_000);
        assert_eq!(parse_rate_list("250k,500k,1M").unwrap(), vec![250_000, 500_000, 1_000_000]);
        assert!(parse_rate_list("1M,500k").is_err());
    }
}
