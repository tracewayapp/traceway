use std::collections::BTreeMap;
use std::path::Path;

use serde::Serialize;
use serde_json::Value;

use crate::pipeline::writer::MaintEvent;
use crate::probe::disk::DiskSample;
use crate::probe::proc::ProcSample;
use crate::probe::ContainerState;
use crate::queries::{QueryOutcome, QueryStatus};
use crate::sink::{DbSize, SettleReport};
use crate::verdict::Verdict;

#[derive(Clone, Debug, Default, Serialize)]
pub struct Pct {
    pub p50: f64,
    pub p95: f64,
    pub p99: f64,
    pub max: f64,
}

#[derive(Clone, Debug, Default, Serialize)]
pub struct Visibility {
    pub last_acked_ts: i64,
    pub visible_ts: i64,
    pub lag_sim_s: f64,
    pub points_behind: f64,
    /// points_behind at the current acknowledged rate: seconds of ingest not yet queryable.
    pub lag_wall_s: Option<f64>,
}

#[derive(Clone, Debug, Default, Serialize)]
pub struct Window {
    pub w: u64,
    pub t_s: u64,
    pub phase: String,
    pub acked_points: u64,
    pub acked_total: u64,
    pub acked_pps: f64,
    pub produced_pps: f64,
    pub write_ms: Pct,
    pub inflight: usize,
    pub batches: u64,
    pub retries: u64,
    pub errors: u64,
    pub points_lost: u64,
    pub gen_stall_pct: f64,
    pub writer_idle_pct: f64,
    pub bytes_sent: u64,
    pub maintenance: Vec<MaintEvent>,
    pub visibility: Option<Visibility>,
    pub queries: Vec<QueryOutcome>,
    /// The store's deferred work (active parts, WAL bytes, tablets, ...), see Report.debt_metric.
    pub debt: Option<f64>,
    /// Writes the store slowed down on purpose this window (ClickHouse DelayedInserts).
    pub throttled: u64,
    pub health: Option<Value>,
    pub proc: Option<ProcSample>,
    pub container: Option<ContainerState>,
    pub disk: Option<DiskSample>,
    pub flags: Vec<String>,
}

#[derive(Clone, Debug, Default, Serialize)]
pub struct BenchInfo {
    pub version: String,
    pub cpus_visible: usize,
    pub gen_threads: usize,
    pub writers: usize,
    pub batch_points: usize,
    pub rate_cap: u64,
    pub window_s: u64,
    pub data_fingerprint: Option<String>,
}

#[derive(Clone, Debug, Default, Serialize)]
pub struct DbConfig {
    pub container: Option<String>,
    pub cgroup: Option<String>,
    pub data_dir: String,
    pub settings: Value,
}

#[derive(Clone, Debug, Default, Serialize)]
pub struct SeriesModel {
    pub series: u64,
    pub hosts: usize,
    pub pods: usize,
    pub templates: usize,
    pub interval_ms: i64,
    pub rounds: u64,
    pub points_planned: u64,
    pub sim_start: String,
    pub sim_end: String,
    pub seed: u64,
    pub avg_tags_per_series: f64,
    pub logical_bytes_per_point: f64,
    pub catalog_build_ms: u64,
    pub catalog_load_ms: u64,
}

#[derive(Clone, Debug, Default, Serialize)]
pub struct SettlePhase {
    pub ms: u64,
    pub settled: bool,
    pub steps: Vec<crate::sink::SettleStep>,
}

#[derive(Clone, Debug, Default, Serialize)]
pub struct Phases {
    pub setup_ms: u64,
    pub warmup_s: u64,
    pub ingest_s: u64,
    pub drain_ms: u64,
    pub settle: Option<SettlePhase>,
    pub cold_ms: u64,
}

#[derive(Clone, Debug, Default, Serialize)]
pub struct Throughput {
    pub mode: String,
    pub plateau_pps: f64,
    pub overall_pps: f64,
    pub ingest_wall_s: f64,
    pub acked_points: u64,
    pub points_lost: u64,
    pub hit_max_ingest: bool,
    pub bench_bottleneck_suspected: bool,
    pub starved_windows: u64,
    /// Highest ramp rate that passed; None in saturate mode or when no step passed.
    pub sustainable_pps: Option<f64>,
    pub fill_rate_pps: u64,
    pub fill_points: u64,
    pub fill_wall_s: f64,
    pub fill_pps: f64,
    /// Mean acknowledged rate over the last quarter of the fill.
    pub late_pps: f64,
    /// Settle plus the wait for merged-away data to be deleted, in seconds.
    pub digest_s: f64,
    /// fill_points / (fill_wall_s + digest_s): what the store really absorbed per second.
    pub amortized_pps: f64,
    pub debt_end: Option<f64>,
}

#[derive(Clone, Debug, Default, Serialize)]
pub struct RampStep {
    pub rate_pps: u64,
    pub step_s: u64,
    pub windows: usize,
    pub achieved_pps: f64,
    pub debt_first_half: Option<f64>,
    pub debt_second_half: Option<f64>,
    pub lag_wall_s: Option<f64>,
    pub query_p95_ms: Option<f64>,
    pub query_timeouts: u64,
    pub errors: u64,
    pub points_lost: u64,
    pub throttled: u64,
    pub passed: bool,
    pub reason: String,
}

#[derive(Clone, Debug, Default, Serialize)]
pub struct RampReport {
    pub ladder: Vec<u64>,
    pub step_s: u64,
    pub ramp_in_s: u64,
    pub steps: Vec<RampStep>,
    pub sustainable_pps: Option<f64>,
    pub fill_rate_pps: u64,
    pub aborted: bool,
    pub note: String,
}

#[derive(Clone, Debug, Default, Serialize)]
pub struct QueryAgg {
    pub intent: String,
    pub n: u64,
    pub p50_ms: f64,
    pub p95_ms: f64,
    pub max_ms: f64,
    pub timeouts: u64,
    pub errors: u64,
    pub suspect: u64,
    pub last_error: Option<String>,
}

#[derive(Clone, Debug, Default, Serialize)]
pub struct ColdQuery {
    pub intent: String,
    pub first_ms: f64,
    pub warm_median_ms: f64,
    pub rows: u64,
    pub status: Option<QueryStatus>,
    pub error: Option<String>,
}

#[derive(Clone, Debug, Default, Serialize)]
pub struct DiskReport {
    pub total_bytes: u64,
    pub by_class: BTreeMap<String, u64>,
    pub bytes_per_point: f64,
    pub ratio_vs_raw16: f64,
    pub ratio_vs_logical: f64,
    pub db_reported: DbSize,
}

#[derive(Clone, Debug, Serialize)]
pub struct Report {
    pub schema_version: u32,
    pub db: String,
    pub family: String,
    pub variant: String,
    pub tier: String,
    pub run_id: String,
    pub started_at: String,
    pub ended_at: Option<String>,
    pub bench: BenchInfo,
    pub db_config: DbConfig,
    pub series_model: SeriesModel,
    pub ack_semantics: String,
    pub debt_metric: String,
    pub ramp: Option<RampReport>,
    pub timeline: Vec<Window>,
    pub phases: Phases,
    pub throughput: Throughput,
    pub queries_during_ingest: BTreeMap<String, QueryAgg>,
    pub queries_cold: BTreeMap<String, ColdQuery>,
    pub cold_method: String,
    pub disk: DiskReport,
    pub verdict: Verdict,
    pub interrupted: bool,
    pub notes: Vec<String>,
}

impl Report {
    pub fn write_atomic(&self, path: &Path) -> anyhow::Result<()> {
        let tmp = path.with_extension("json.tmp");
        let bytes = serde_json::to_vec_pretty(self)?;
        std::fs::write(&tmp, bytes)?;
        std::fs::rename(&tmp, path)?;
        Ok(())
    }

    pub fn settle_from(rep: SettleReport, ms: u64) -> SettlePhase {
        SettlePhase { ms, settled: rep.settled, steps: rep.steps }
    }
}

pub fn aggregate_queries(outcomes: &[QueryOutcome]) -> BTreeMap<String, QueryAgg> {
    let mut by: BTreeMap<String, Vec<&QueryOutcome>> = BTreeMap::new();
    for o in outcomes {
        by.entry(o.id.as_str().to_string()).or_default().push(o);
    }
    by.into_iter()
        .map(|(k, v)| {
            let mut ms: Vec<f64> = v.iter().filter(|o| o.status != QueryStatus::Error).map(|o| o.ms).collect();
            ms.sort_by(|a, b| a.partial_cmp(b).unwrap());
            let pct = |p: f64| if ms.is_empty() { 0.0 } else { ms[((ms.len() as f64 - 1.0) * p).round() as usize] };
            let agg = QueryAgg {
                intent: v[0].id.intent().to_string(),
                n: v.len() as u64,
                p50_ms: pct(0.5),
                p95_ms: pct(0.95),
                max_ms: ms.last().copied().unwrap_or(0.0),
                timeouts: v.iter().filter(|o| o.status == QueryStatus::Timeout).count() as u64,
                errors: v.iter().filter(|o| o.status == QueryStatus::Error).count() as u64,
                suspect: v.iter().filter(|o| o.status == QueryStatus::Suspect).count() as u64,
                last_error: v.iter().rev().find_map(|o| o.error.clone()),
            };
            (k, agg)
        })
        .collect()
}
