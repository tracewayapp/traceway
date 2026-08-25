use std::collections::HashMap;

use xxhash_rust::xxh3::xxh3_64;

use super::rng::Xoshiro256;
use super::templates::{MetricTemplate, Scope, NAMESPACES, PODS_PER_HOST, REGIONS, SERVICES, TEMPLATES, ZONES};

pub type Sym = u32;

pub const SENTINEL_ID: u64 = 0;
pub const SENTINEL_NAME: &str = "bench.sentinel";
pub const SENTINEL_HOST: &str = "bench-sentinel";
pub const HOST_KEY: &str = "host.name";
pub const CLUSTER_KEY: &str = "k8s.cluster.name";

#[derive(Default)]
pub struct StringTable {
    strs: Vec<String>,
    map: HashMap<String, Sym>,
}

impl StringTable {
    pub fn intern(&mut self, s: &str) -> Sym {
        if let Some(&i) = self.map.get(s) {
            return i;
        }
        let i = self.strs.len() as Sym;
        self.strs.push(s.to_string());
        self.map.insert(s.to_string(), i);
        i
    }
    #[inline]
    pub fn get(&self, s: Sym) -> &str {
        &self.strs[s as usize]
    }
}

pub struct SeriesMeta {
    pub name: Sym,
    /// Sorted by key string, which is what VictoriaMetrics expects and what
    /// keeps every store's series identity byte-identical.
    pub tags: Box<[(Sym, Sym)]>,
    pub template: u16,
}

pub struct MetricRange {
    pub name: Sym,
    pub lo: u64,
    pub hi: u64,
}

pub struct HostMeta {
    pub name: Sym,
    pub jitter_ms: i64,
    pub quiet: bool,
    pub series: Vec<u64>,
}

pub struct CatalogConfig {
    pub series: u64,
    pub points: u64,
    pub interval_ms: i64,
    pub sim_start_ms: i64,
    pub seed: u64,
}

pub struct SeriesCatalog {
    pub strings: StringTable,
    pub series: Vec<SeriesMeta>,
    pub metrics: Vec<MetricRange>,
    pub hosts: Vec<HostMeta>,
    pub pods: usize,
    pub clusters: usize,
    pub interval_ms: i64,
    pub sim_start_ms: i64,
    pub rounds: u64,
    pub seed: u64,
    pub logical_bytes_per_point: f64,
}

impl SeriesCatalog {
    pub fn build(cfg: &CatalogConfig) -> Self {
        assert!(cfg.series >= 2, "need at least the sentinel plus one series");
        let per_host = super::templates::series_per_host() as u64;
        let n_data = cfg.series - 1;
        let full_hosts = (n_data / per_host) as usize;
        let rem = (n_data % per_host) as usize;
        let n_hosts = full_hosts + usize::from(rem > 0);

        let per_template: Vec<usize> = TEMPLATES
            .iter()
            .map(|t| match t.scope {
                Scope::Host => t.instances(),
                _ => t.instances() * PODS_PER_HOST,
            })
            .collect();

        let mut strings = StringTable::default();
        let mut metrics = Vec::with_capacity(TEMPLATES.len());
        let mut lo = 1u64;
        let mut prefix = 0usize;
        for (ti, t) in TEMPLATES.iter().enumerate() {
            let k = per_template[ti];
            let partial = rem.saturating_sub(prefix).min(k);
            let count = full_hosts as u64 * k as u64 + partial as u64;
            metrics.push(MetricRange { name: strings.intern(t.name), lo, hi: lo + count });
            lo += count;
            prefix += k;
        }
        debug_assert_eq!(lo, cfg.series);

        let sym = |st: &mut StringTable, s: &str| st.intern(s);
        let k_host = sym(&mut strings, HOST_KEY);
        let k_host_id = sym(&mut strings, "host.id");
        let k_os = sym(&mut strings, "os.type");
        let k_region = sym(&mut strings, "cloud.region");
        let k_zone = sym(&mut strings, "cloud.availability_zone");
        let k_cluster = sym(&mut strings, CLUSTER_KEY);
        let k_node = sym(&mut strings, "k8s.node.name");
        let k_server = sym(&mut strings, "server_name");
        let k_ns = sym(&mut strings, "k8s.namespace.name");
        let k_pod = sym(&mut strings, "k8s.pod.name");
        let k_container = sym(&mut strings, "container.name");
        let k_pid = sym(&mut strings, "process.pid");
        let v_linux = sym(&mut strings, "linux");

        let mut series: Vec<Option<SeriesMeta>> = (0..cfg.series).map(|_| None).collect();
        let sentinel_name = strings.intern(SENTINEL_NAME);
        let sentinel_host = strings.intern(SENTINEL_HOST);
        series[0] = Some(SeriesMeta { name: sentinel_name, tags: vec![(k_host, sentinel_host)].into_boxed_slice(), template: u16::MAX });

        let mut hosts = Vec::with_capacity(n_hosts);
        let mut logical_bytes = 0u64;
        for h in 0..n_hosts {
            let mut rng = Xoshiro256::seeded(cfg.seed ^ (h as u64).wrapping_mul(0x9E37_79B9_7F4A_7C15));
            let host_name = format!("h-{h:05}");
            let region_i = (rng.next_u64() % REGIONS.len() as u64) as usize;
            let region = REGIONS[region_i];
            let zone = format!("{}{}", region, ZONES[(rng.next_u64() % ZONES.len() as u64) as usize]);
            let cluster = format!("{}-{}", region, 1 + (rng.next_u64() % 2));
            let host_id = format!("{:016x}", xxh3_64(format!("host-id:{host_name}").as_bytes()));
            let quiet = rng.next_f64() < 0.05;
            let jitter_ms = (xxh3_64(format!("{}:{}", cfg.seed, host_name).as_bytes()) % cfg.interval_ms as u64) as i64;

            let s_host = strings.intern(&host_name);
            let s_host_id = strings.intern(&host_id);
            let s_region = strings.intern(region);
            let s_zone = strings.intern(&zone);
            let s_cluster = strings.intern(&cluster);
            let host_attrs: Vec<(Sym, Sym)> = vec![
                (k_host, s_host),
                (k_host_id, s_host_id),
                (k_os, v_linux),
                (k_region, s_region),
                (k_zone, s_zone),
                (k_cluster, s_cluster),
                (k_node, s_host),
                (k_server, s_host),
            ];

            let mut pod_attrs: Vec<Vec<(Sym, Sym)>> = Vec::with_capacity(PODS_PER_HOST);
            let mut svc_attrs: Vec<Vec<(Sym, Sym)>> = Vec::with_capacity(PODS_PER_HOST);
            let mut container_attrs: Vec<Vec<(Sym, Sym)>> = Vec::with_capacity(PODS_PER_HOST);
            for p in 0..PODS_PER_HOST {
                let svc = SERVICES[((h * PODS_PER_HOST + p) as u64 % SERVICES.len() as u64) as usize];
                let ns = NAMESPACES[(rng.next_u64() % NAMESPACES.len() as u64) as usize];
                let pod_name = format!("{}-{:06x}", svc, rng.next_u64() & 0xFF_FFFF);
                let pid = 1000 + rng.next_u64() % 60_000;
                let s_svc = strings.intern(svc);
                let s_ns = strings.intern(ns);
                let s_pod = strings.intern(&pod_name);
                let s_pid = strings.intern(&pid.to_string());
                let mut pa = vec![(k_host, s_host), (k_region, s_region), (k_cluster, s_cluster), (k_node, s_host), (k_ns, s_ns), (k_pod, s_pod), (k_server, s_host)];
                let mut ca = pa.clone();
                ca.push((k_container, s_svc));
                let mut sa = vec![(k_host, s_host), (k_region, s_region), (k_cluster, s_cluster), (k_ns, s_ns), (k_pod, s_pod), (k_server, s_svc), (k_pid, s_pid)];
                pa.shrink_to_fit();
                sa.shrink_to_fit();
                pod_attrs.push(pa);
                container_attrs.push(ca);
                svc_attrs.push(sa);
            }

            let mut host_series = Vec::with_capacity(if h < full_hosts { per_host as usize } else { rem });
            let mut prefix = 0usize;
            for (ti, t) in TEMPLATES.iter().enumerate() {
                let k = per_template[ti];
                let n = if h < full_hosts { k } else { rem.saturating_sub(prefix).min(k) };
                let range = &metrics[ti];
                for i in 0..n {
                    let id = range.lo + (h * k + i) as u64;
                    let (pod, dim_i) = match t.scope {
                        Scope::Host => (usize::MAX, i),
                        _ => (i / t.instances(), i % t.instances()),
                    };
                    let mut tags: Vec<(Sym, Sym)> = match t.scope {
                        Scope::Host => host_attrs.clone(),
                        Scope::Pod => pod_attrs[pod].clone(),
                        Scope::Container => container_attrs[pod].clone(),
                        Scope::Service => svc_attrs[pod].clone(),
                    };
                    push_dims(t, dim_i, &mut strings, &mut tags);
                    tags.sort_by(|a, b| strings.get(a.0).cmp(strings.get(b.0)));
                    logical_bytes += t.name.len() as u64
                        + tags.iter().map(|(k, v)| (strings.get(*k).len() + strings.get(*v).len()) as u64).sum::<u64>();
                    series[id as usize] = Some(SeriesMeta { name: range.name, tags: tags.into_boxed_slice(), template: ti as u16 });
                    host_series.push(id);
                }
                prefix += k;
            }
            hosts.push(HostMeta { name: s_host, jitter_ms, quiet, series: host_series });
        }

        let series: Vec<SeriesMeta> = series.into_iter().map(|s| s.expect("every id assigned")).collect();
        let rounds = cfg.points.div_ceil(cfg.series);
        SeriesCatalog {
            strings,
            series,
            metrics,
            pods: n_hosts * PODS_PER_HOST,
            clusters: REGIONS.len() * 2,
            hosts,
            interval_ms: cfg.interval_ms,
            sim_start_ms: cfg.sim_start_ms,
            rounds,
            seed: cfg.seed,
            logical_bytes_per_point: 16.0 + logical_bytes as f64 / n_data.max(1) as f64,
        }
    }

    pub fn len(&self) -> u64 {
        self.series.len() as u64
    }

    pub fn sim_end_ms(&self) -> i64 {
        self.sim_start_ms + (self.rounds as i64 - 1) * self.interval_ms
    }


    pub fn metric_range(&self, name: &str) -> Option<&MetricRange> {
        self.metrics.iter().find(|m| self.strings.get(m.name) == name)
    }

    pub fn tag(&self, id: u64, key: &str) -> Option<&str> {
        let s = &self.series[id as usize];
        s.tags.iter().find(|(k, _)| self.strings.get(*k) == key).map(|(_, v)| self.strings.get(*v))
    }

    pub fn find_series(&self, name: &str, want: &[(&str, &str)]) -> Option<u64> {
        let r = self.metric_range(name)?;
        (r.lo..r.hi).find(|&id| want.iter().all(|(k, v)| self.tag(id, k) == Some(*v)))
    }

    pub fn tags_of(&self, id: u64) -> Vec<(&str, &str)> {
        self.series[id as usize].tags.iter().map(|(k, v)| (self.strings.get(*k), self.strings.get(*v))).collect()
    }

    pub fn name_of(&self, id: u64) -> &str {
        self.strings.get(self.series[id as usize].name)
    }

}

fn push_dims(t: &MetricTemplate, mut dim_i: usize, strings: &mut StringTable, tags: &mut Vec<(Sym, Sym)>) {
    // Mixed-radix decode, last dim fastest, so instance order is stable.
    let mut idx = vec![0usize; t.dims.len()];
    for (d, dim) in t.dims.iter().enumerate().rev() {
        idx[d] = dim_i % dim.len();
        dim_i /= dim.len();
    }
    for (d, dim) in t.dims.iter().enumerate() {
        let k = strings.intern(dim.key());
        let v = strings.intern(&dim.value(idx[d]));
        tags.push((k, v));
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn cfg(series: u64) -> CatalogConfig {
        CatalogConfig { series, points: series * 3, interval_ms: 10_000, sim_start_ms: 1_767_225_600_000, seed: 42 }
    }

    #[test]
    fn exact_series_count_and_contiguous_ranges() {
        for n in [2u64, 100, 1_001, 10_000, 12_345] {
            let c = SeriesCatalog::build(&cfg(n));
            assert_eq!(c.len(), n);
            let mut expect = 1u64;
            for m in &c.metrics {
                assert_eq!(m.lo, expect);
                expect = m.hi;
            }
            assert_eq!(expect, n);
            let owned: u64 = c.hosts.iter().map(|h| h.series.len() as u64).sum();
            assert_eq!(owned, n - 1);
            for m in &c.metrics {
                for id in m.lo..m.hi {
                    assert_eq!(c.series[id as usize].name, m.name);
                }
            }
        }
    }

    #[test]
    fn tags_sorted_and_host_lookup_works() {
        let c = SeriesCatalog::build(&cfg(20_000));
        for s in &c.series {
            let keys: Vec<&str> = s.tags.iter().map(|(k, _)| c.strings.get(*k)).collect();
            let mut sorted = keys.clone();
            sorted.sort();
            assert_eq!(keys, sorted);
        }
        let id = c.find_series("system.cpu.utilization", &[("host.name", "h-00003"), ("cpu", "3"), ("state", "user")]).unwrap();
        assert_eq!(c.tag(id, "cpu"), Some("3"));
        assert_eq!(c.name_of(id), "system.cpu.utilization");
    }
}
