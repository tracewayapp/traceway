#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Scope {
    Host,
    Pod,
    Container,
    Service,
}

#[allow(dead_code)]
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Kind {
    Gauge,
    Counter,
}

#[derive(Clone, Copy, Debug)]
pub enum Dim {
    Values(&'static str, &'static [&'static str]),
    Range(&'static str, u32),
}

impl Dim {
    pub fn key(&self) -> &'static str {
        match self {
            Dim::Values(k, _) | Dim::Range(k, _) => k,
        }
    }
    pub fn len(&self) -> usize {
        match self {
            Dim::Values(_, v) => v.len(),
            Dim::Range(_, n) => *n as usize,
        }
    }
    pub fn value(&self, i: usize) -> String {
        match self {
            Dim::Values(_, v) => v[i].to_string(),
            Dim::Range(_, _) => i.to_string(),
        }
    }
}

#[derive(Clone, Copy, Debug)]
pub enum ValueModel {
    Walk { lo: f64, hi: f64, sigma: f64, revert: f64 },
    Counter { rate_mean: f64, rate_cv: f64 },
    Const { lo: f64, hi: f64 },
    ZeroMostly { p: f64, burst: f64 },
    Spiky { base: f64, sigma: f64, spike_p: f64, spike_mag: f64 },
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Precision {
    Int,
    Dec2,
    Dec4,
}

#[derive(Clone, Copy, Debug)]
pub struct MetricTemplate {
    pub name: &'static str,
    pub scope: Scope,
    pub dims: &'static [Dim],
    #[allow(dead_code)]
    pub kind: Kind,
    pub value: ValueModel,
    pub precision: Precision,
}

impl MetricTemplate {
    pub fn instances(&self) -> usize {
        self.dims.iter().map(Dim::len).product::<usize>()
    }
}

const CPU_STATES: &[&str] = &["user", "system", "idle", "interrupt", "nice", "softirq", "steal", "wait"];
const MEM_STATES: &[&str] = &["used", "free", "cached", "buffered", "slab_reclaimable", "slab_unreclaimable"];
const DISKS: &[&str] = &["nvme0n1", "sda"];
const DIRECTION_IO: &[&str] = &["read", "write"];
const DIRECTION_NET: &[&str] = &["receive", "transmit"];
const MOUNTS: &[&str] = &["/", "/data", "/run"];
const FS_STATES: &[&str] = &["used", "free", "reserved"];
const NICS: &[&str] = &["eth0", "eth1"];
const CONN_STATES: &[&str] = &["ESTABLISHED", "TIME_WAIT", "CLOSE_WAIT", "LISTEN", "SYN_SENT"];
const ROUTES: &[&str] = &["/api/users", "/api/orders", "/api/search"];
const METHODS: &[&str] = &["GET", "POST"];

const UTIL: ValueModel = ValueModel::Walk { lo: 0.0, hi: 1.0, sigma: 0.015, revert: 0.02 };
const BYTES_GAUGE: ValueModel = ValueModel::Walk { lo: 1.0e8, hi: 3.2e10, sigma: 4.0e6, revert: 0.002 };
const SMALL_GAUGE: ValueModel = ValueModel::Walk { lo: 0.0, hi: 64.0, sigma: 0.5, revert: 0.02 };
const IO_COUNTER: ValueModel = ValueModel::Counter { rate_mean: 2.0e6, rate_cv: 0.4 };
const OPS_COUNTER: ValueModel = ValueModel::Counter { rate_mean: 250.0, rate_cv: 0.3 };
const SECONDS_COUNTER: ValueModel = ValueModel::Counter { rate_mean: 1.2, rate_cv: 0.15 };
const RARE: ValueModel = ValueModel::ZeroMostly { p: 0.01, burst: 3.0 };
const LATENCY: ValueModel = ValueModel::Spiky { base: 42.0, sigma: 2.5, spike_p: 0.01, spike_mag: 30.0 };

macro_rules! t {
    ($name:expr, $scope:ident, $dims:expr, $kind:ident, $value:expr, $prec:ident) => {
        MetricTemplate {
            name: $name,
            scope: Scope::$scope,
            dims: $dims,
            kind: Kind::$kind,
            value: $value,
            precision: Precision::$prec,
        }
    };
}

/// Modeled on the OTel hostmetrics / kubeletstats receivers and a typical
/// HTTP service. Dimension products are what real fleets emit (8 cpus x 8
/// states per host, one interface x two directions per pod, ...).
pub const TEMPLATES: &[MetricTemplate] = &[
    t!("system.cpu.time", Host, &[Dim::Range("cpu", 8), Dim::Values("state", CPU_STATES)], Counter, SECONDS_COUNTER, Dec2),
    t!("system.cpu.utilization", Host, &[Dim::Range("cpu", 8), Dim::Values("state", CPU_STATES)], Gauge, UTIL, Dec4),
    t!("system.cpu.load_average.1m", Host, &[], Gauge, SMALL_GAUGE, Dec2),
    t!("system.cpu.load_average.5m", Host, &[], Gauge, SMALL_GAUGE, Dec2),
    t!("system.cpu.load_average.15m", Host, &[], Gauge, SMALL_GAUGE, Dec2),
    t!("system.memory.usage", Host, &[Dim::Values("state", MEM_STATES)], Gauge, BYTES_GAUGE, Int),
    t!("system.memory.utilization", Host, &[Dim::Values("state", MEM_STATES)], Gauge, UTIL, Dec4),
    t!("system.disk.io", Host, &[Dim::Values("device", DISKS), Dim::Values("direction", DIRECTION_IO)], Counter, IO_COUNTER, Int),
    t!("system.disk.operations", Host, &[Dim::Values("device", DISKS), Dim::Values("direction", DIRECTION_IO)], Counter, OPS_COUNTER, Int),
    t!("system.disk.io_time", Host, &[Dim::Values("device", DISKS)], Counter, SECONDS_COUNTER, Dec2),
    t!("system.disk.pending_operations", Host, &[Dim::Values("device", DISKS)], Gauge, SMALL_GAUGE, Int),
    t!("system.filesystem.usage", Host, &[Dim::Values("mountpoint", MOUNTS), Dim::Values("state", FS_STATES)], Gauge, BYTES_GAUGE, Int),
    t!("system.filesystem.utilization", Host, &[Dim::Values("mountpoint", MOUNTS)], Gauge, UTIL, Dec4),
    t!("system.network.io", Host, &[Dim::Values("device", NICS), Dim::Values("direction", DIRECTION_NET)], Counter, IO_COUNTER, Int),
    t!("system.network.packets", Host, &[Dim::Values("device", NICS), Dim::Values("direction", DIRECTION_NET)], Counter, OPS_COUNTER, Int),
    t!("system.network.errors", Host, &[Dim::Values("device", NICS), Dim::Values("direction", DIRECTION_NET)], Counter, RARE, Int),
    t!("system.network.dropped", Host, &[Dim::Values("device", NICS), Dim::Values("direction", DIRECTION_NET)], Counter, RARE, Int),
    t!("system.network.connections", Host, &[Dim::Values("protocol", &["tcp"]), Dim::Values("state", CONN_STATES)], Gauge, SMALL_GAUGE, Int),
    t!("system.paging.operations", Host, &[Dim::Values("direction", &["page_in", "page_out"]), Dim::Values("type", &["major", "minor"])], Counter, OPS_COUNTER, Int),
    t!("system.paging.usage", Host, &[Dim::Values("state", &["used", "free"])], Gauge, BYTES_GAUGE, Int),
    t!("system.processes.count", Host, &[Dim::Values("status", &["running", "sleeping", "blocked", "zombies"])], Gauge, SMALL_GAUGE, Int),
    t!("system.processes.created", Host, &[], Counter, OPS_COUNTER, Int),
    t!("system.uptime", Host, &[], Gauge, ValueModel::Counter { rate_mean: 1.0, rate_cv: 0.0 }, Int),
    t!("k8s.pod.cpu.utilization", Pod, &[], Gauge, UTIL, Dec4),
    t!("k8s.pod.cpu.time", Pod, &[], Counter, SECONDS_COUNTER, Dec2),
    t!("k8s.pod.memory.usage", Pod, &[], Gauge, BYTES_GAUGE, Int),
    t!("k8s.pod.memory.working_set", Pod, &[], Gauge, BYTES_GAUGE, Int),
    t!("k8s.pod.memory.rss", Pod, &[], Gauge, BYTES_GAUGE, Int),
    t!("k8s.pod.memory.available", Pod, &[], Gauge, BYTES_GAUGE, Int),
    t!("k8s.pod.memory.page_faults", Pod, &[], Counter, OPS_COUNTER, Int),
    t!("k8s.pod.memory.major_page_faults", Pod, &[], Counter, RARE, Int),
    t!("k8s.pod.network.io", Pod, &[Dim::Values("interface", &["eth0"]), Dim::Values("direction", DIRECTION_NET)], Counter, IO_COUNTER, Int),
    t!("k8s.pod.network.errors", Pod, &[Dim::Values("interface", &["eth0"]), Dim::Values("direction", DIRECTION_NET)], Counter, RARE, Int),
    t!("k8s.pod.filesystem.usage", Pod, &[], Gauge, BYTES_GAUGE, Int),
    t!("k8s.pod.filesystem.available", Pod, &[], Gauge, BYTES_GAUGE, Int),
    t!("k8s.pod.filesystem.capacity", Pod, &[], Gauge, ValueModel::Const { lo: 1.0e10, hi: 2.0e11 }, Int),
    t!("k8s.pod.phase", Pod, &[], Gauge, ValueModel::Const { lo: 2.0, hi: 2.0 }, Int),
    t!("k8s.container.restarts", Pod, &[], Counter, ValueModel::ZeroMostly { p: 0.0005, burst: 1.0 }, Int),
    t!("k8s.container.cpu_request", Pod, &[], Gauge, ValueModel::Const { lo: 0.1, hi: 2.0 }, Dec2),
    t!("k8s.container.cpu_limit", Pod, &[], Gauge, ValueModel::Const { lo: 0.5, hi: 4.0 }, Dec2),
    t!("k8s.container.memory_request", Pod, &[], Gauge, ValueModel::Const { lo: 1.0e8, hi: 4.0e9 }, Int),
    t!("k8s.container.memory_limit", Pod, &[], Gauge, ValueModel::Const { lo: 2.0e8, hi: 8.0e9 }, Int),
    t!("container.cpu.utilization", Container, &[], Gauge, UTIL, Dec4),
    t!("container.cpu.time", Container, &[], Counter, SECONDS_COUNTER, Dec2),
    t!("container.memory.usage", Container, &[], Gauge, BYTES_GAUGE, Int),
    t!("container.memory.working_set", Container, &[], Gauge, BYTES_GAUGE, Int),
    t!("container.memory.rss", Container, &[], Gauge, BYTES_GAUGE, Int),
    t!("container.filesystem.usage", Container, &[], Gauge, BYTES_GAUGE, Int),
    t!("container.filesystem.available", Container, &[], Gauge, BYTES_GAUGE, Int),
    t!("container.cpu.throttling_data.periods", Container, &[], Counter, OPS_COUNTER, Int),
    t!("container.cpu.throttling_data.throttled_periods", Container, &[], Counter, RARE, Int),
    t!("http.server.duration.avg", Service, &[Dim::Values("http.route", ROUTES), Dim::Values("http.method", METHODS)], Gauge, LATENCY, Dec2),
    t!("http.server.duration.count", Service, &[Dim::Values("http.route", ROUTES), Dim::Values("http.method", METHODS)], Counter, OPS_COUNTER, Int),
    t!("http.server.active_requests", Service, &[], Gauge, SMALL_GAUGE, Int),
    t!("process.cpu.time", Service, &[Dim::Values("state", &["user", "system"])], Counter, SECONDS_COUNTER, Dec2),
    t!("process.memory.usage", Service, &[], Gauge, BYTES_GAUGE, Int),
    t!("process.open_file_descriptors", Service, &[], Gauge, SMALL_GAUGE, Int),
];

pub const PODS_PER_HOST: usize = 8;

pub fn series_per_host() -> usize {
    TEMPLATES
        .iter()
        .map(|t| match t.scope {
            Scope::Host => t.instances(),
            Scope::Pod | Scope::Container | Scope::Service => t.instances() * PODS_PER_HOST,
        })
        .sum()
}

pub const REGIONS: &[&str] = &["eu-central-1", "eu-west-1", "us-east-1", "ap-southeast-1"];
pub const ZONES: &[&str] = &["a", "b", "c"];
pub const NAMESPACES: &[&str] = &[
    "default", "payments", "search", "checkout", "identity", "ingest", "billing", "notifications", "catalog", "analytics", "edge", "ops",
];
pub const SERVICES: &[&str] = &[
    "api-gateway", "user-service", "order-service", "search-indexer", "payment-worker", "catalog-api", "notifier", "auth", "billing-api",
    "recommendation", "cart", "inventory", "shipping", "pricing", "reporting", "audit", "session-store", "mailer", "webhooks", "scheduler",
    "feed", "media", "geo", "fraud", "ledger", "exporter", "importer", "cache-warmer", "rate-limiter", "graphql", "admin-api", "mobile-bff",
    "web-bff", "search-api", "events", "metrics-agent", "log-shipper", "tracer", "profiler", "sync",
];
