use std::sync::Arc;
use std::time::Duration;

use serde::Serialize;

use crate::model::catalog::{MetricRange, SeriesCatalog};

pub mod sql;

#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash, PartialOrd, Ord, Serialize)]
pub enum QueryId {
    A,
    B,
    C,
    D,
    E1,
    E2,
    F,
}

impl QueryId {
    pub fn all() -> &'static [QueryId] {
        &[QueryId::A, QueryId::B, QueryId::C, QueryId::D, QueryId::E1, QueryId::E2, QueryId::F]
    }
    pub fn as_str(self) -> &'static str {
        match self {
            QueryId::A => "a",
            QueryId::B => "b",
            QueryId::C => "c",
            QueryId::D => "d",
            QueryId::E1 => "e1",
            QueryId::E2 => "e2",
            QueryId::F => "f",
        }
    }
    pub fn intent(self) -> &'static str {
        match self {
            QueryId::A => "one metric, avg per host, 1m buckets, last 1h",
            QueryId::B => "one series, 1m buckets, last 6h",
            QueryId::C => "latest value per host, top 20, last 10m",
            QueryId::D => "pod cpu grouped by cluster, 5m buckets, last 24h",
            QueryId::E1 => "discovery: active metric names + series count, last 1h",
            QueryId::E2 => "discovery: distinct hosts for one metric, last 1h",
            QueryId::F => "alert: count + p95 of latest value per series, last 5m",
        }
    }
    /// (lookback, bucket) in seconds.
    pub fn window(self) -> (i64, i64) {
        match self {
            QueryId::A => (3_600, 60),
            QueryId::B => (21_600, 60),
            QueryId::C => (600, 0),
            QueryId::D => (86_400, 300),
            QueryId::E1 => (3_600, 0),
            QueryId::E2 => (3_600, 0),
            QueryId::F => (300, 0),
        }
    }
}

#[derive(Clone, Debug)]
pub struct Metric {
    pub name: String,
    pub lo: u64,
    pub hi: u64,
}

impl Metric {
    fn from(r: &MetricRange, cat: &SeriesCatalog) -> Self {
        Self { name: cat.strings.get(r.name).to_string(), lo: r.lo, hi: r.hi }
    }
}

#[derive(Clone, Debug)]
pub struct QueryParams {
    pub cpu: Metric,
    pub mem: Metric,
    pub pod: Metric,
    pub http: Metric,
    pub one_series: u64,
    pub one_host: String,
    pub hosts: u64,
    pub clusters: u64,
    pub names: u64,
    pub interval_ms: i64,
    pub sim_start_ms: i64,
}

impl QueryParams {
    pub fn from_catalog(cat: &SeriesCatalog) -> Self {
        let get = |n: &str| Metric::from(cat.metric_range(n).unwrap_or_else(|| panic!("template {n} missing")), cat);
        let host_idx = 17.min(cat.hosts.len().saturating_sub(1));
        let one_host = cat.strings.get(cat.hosts[host_idx].name).to_string();
        let one_series = cat
            .find_series("system.cpu.utilization", &[("host.name", &one_host), ("cpu", "3"), ("state", "user")])
            .or_else(|| cat.find_series("system.cpu.utilization", &[("host.name", &one_host)]))
            .or_else(|| cat.metric_range("system.cpu.utilization").map(|r| r.lo))
            .unwrap_or(1);
        Self {
            cpu: get("system.cpu.utilization"),
            mem: get("system.memory.utilization"),
            pod: get("k8s.pod.cpu.utilization"),
            http: get("http.server.duration.avg"),
            one_series,
            one_host,
            hosts: cat.hosts.len() as u64,
            clusters: cat.clusters as u64,
            names: cat.metrics.len() as u64 + 1,
            interval_ms: cat.interval_ms,
            sim_start_ms: cat.sim_start_ms,
        }
    }

    /// Buckets are capped by how much simulated time exists before `t_ms`, so
    /// a short run does not flag every windowed query as suspect.
    pub fn expected_rows(&self, id: QueryId, t_ms: i64) -> u64 {
        let (lookback, bucket) = id.window();
        let elapsed_s = ((t_ms - self.sim_start_ms) / 1000).max(0);
        let buckets = if bucket > 0 { (lookback.min(elapsed_s + self.interval_ms / 1000) / bucket).max(1) as u64 } else { 0 };
        match id {
            QueryId::A => buckets * self.hosts,
            QueryId::B => buckets,
            QueryId::C => 20.min(self.hosts),
            QueryId::D => buckets * self.clusters.min(self.hosts),
            QueryId::E1 => self.names,
            QueryId::E2 => self.hosts,
            QueryId::F => 1,
        }
    }
}

#[derive(Clone, Debug)]
pub struct QueryInstance {
    pub id: QueryId,
    pub t_ms: i64,
    pub params: Arc<QueryParams>,
}

impl QueryInstance {
    pub fn from_ms(&self) -> i64 {
        self.t_ms - self.id.window().0 * 1000
    }
}

#[derive(Clone, Copy, Debug, PartialEq, Eq, Serialize)]
#[serde(rename_all = "lowercase")]
pub enum QueryStatus {
    Ok,
    Timeout,
    Error,
    Suspect,
}

#[derive(Clone, Debug, Serialize)]
pub struct QueryOutcome {
    pub id: QueryId,
    pub ms: f64,
    pub rows: u64,
    pub status: QueryStatus,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error: Option<String>,
}

impl QueryOutcome {
    pub fn ok(id: QueryId, elapsed: Duration, rows: u64, expected: u64) -> Self {
        let status = if expected > 0 && (rows as f64) < 0.5 * expected as f64 { QueryStatus::Suspect } else { QueryStatus::Ok };
        Self { id, ms: elapsed.as_secs_f64() * 1000.0, rows, status, error: None }
    }
    pub fn timeout(id: QueryId, elapsed: Duration) -> Self {
        Self { id, ms: elapsed.as_secs_f64() * 1000.0, rows: 0, status: QueryStatus::Timeout, error: None }
    }
    pub fn error(id: QueryId, elapsed: Duration, e: impl Into<String>) -> Self {
        let mut msg: String = e.into();
        msg.truncate(400);
        Self { id, ms: elapsed.as_secs_f64() * 1000.0, rows: 0, status: QueryStatus::Error, error: Some(msg) }
    }
}
