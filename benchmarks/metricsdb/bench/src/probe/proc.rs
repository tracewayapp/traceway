use std::path::PathBuf;
use std::time::Instant;

use serde::Serialize;

#[derive(Clone, Debug, Default, Serialize)]
pub struct ProcSample {
    pub rss_bytes: u64,
    pub peak_rss_bytes: u64,
    pub cpu_pct: f64,
    pub io_write_bytes: u64,
    pub oom_kills: u64,
    pub bench_cpu_pct: f64,
    pub bench_rss_bytes: u64,
}

pub struct ProcSampler {
    cgroup: Option<PathBuf>,
    in_process: bool,
    last_cg: Option<(Instant, u64)>,
    last_self: Option<(Instant, u64, u64)>,
    ticks_per_s: f64,
}

impl ProcSampler {
    pub fn new(cgroup: Option<PathBuf>, in_process: bool) -> Self {
        let ticks = unsafe { libc::sysconf(libc::_SC_CLK_TCK) };
        Self { cgroup: cgroup.filter(|p| p.exists()), in_process, last_cg: None, last_self: None, ticks_per_s: if ticks > 0 { ticks as f64 } else { 100.0 } }
    }

    pub fn sample(&mut self) -> ProcSample {
        let mut s = ProcSample::default();
        let now = Instant::now();
        let (self_rss, self_peak, self_io) = self_mem_io();
        let (db_ticks, bench_ticks) = self_cpu_ticks(self.in_process);
        if let Some((t, d, b)) = self.last_self {
            let dt = now.duration_since(t).as_secs_f64().max(0.001);
            s.bench_cpu_pct = (bench_ticks.saturating_sub(b)) as f64 / self.ticks_per_s / dt * 100.0;
            if self.in_process {
                s.cpu_pct = (db_ticks.saturating_sub(d)) as f64 / self.ticks_per_s / dt * 100.0;
            }
        }
        self.last_self = Some((now, db_ticks, bench_ticks));
        s.bench_rss_bytes = self_rss;
        if self.in_process {
            s.rss_bytes = self_rss;
            s.peak_rss_bytes = self_peak;
            s.io_write_bytes = self_io;
        } else if let Some(cg) = &self.cgroup {
            s.rss_bytes = read_u64(cg.join("memory.current")).unwrap_or(0);
            s.peak_rss_bytes = read_u64(cg.join("memory.peak")).unwrap_or(0);
            s.oom_kills = read_kv(cg.join("memory.events"), "oom_kill").unwrap_or(0);
            let usage = read_kv(cg.join("cpu.stat"), "usage_usec").unwrap_or(0);
            if let Some((t, u)) = self.last_cg {
                let dt = now.duration_since(t).as_secs_f64().max(0.001);
                s.cpu_pct = usage.saturating_sub(u) as f64 / 1e6 / dt * 100.0;
            }
            self.last_cg = Some((now, usage));
            s.io_write_bytes = std::fs::read_to_string(cg.join("io.stat"))
                .map(|txt| txt.lines().flat_map(|l| l.split_whitespace().filter_map(|kv| kv.strip_prefix("wbytes=")).filter_map(|v| v.parse::<u64>().ok()).next()).sum())
                .unwrap_or(0);
        }
        s
    }
}

fn read_u64(p: PathBuf) -> Option<u64> {
    std::fs::read_to_string(p).ok()?.trim().parse().ok()
}

fn read_kv(p: PathBuf, key: &str) -> Option<u64> {
    let txt = std::fs::read_to_string(p).ok()?;
    txt.lines().find_map(|l| {
        let mut it = l.split_whitespace();
        (it.next()? == key).then(|| it.next()?.parse().ok())?
    })
}

fn self_mem_io() -> (u64, u64, u64) {
    let status = std::fs::read_to_string("/proc/self/status").unwrap_or_default();
    let kb = |key: &str| status.lines().find(|l| l.starts_with(key)).and_then(|l| l.split_whitespace().nth(1)).and_then(|v| v.parse::<u64>().ok()).unwrap_or(0) * 1024;
    let io = std::fs::read_to_string("/proc/self/io").unwrap_or_default();
    let wb = io.lines().find(|l| l.starts_with("write_bytes:")).and_then(|l| l.split_whitespace().nth(1)).and_then(|v| v.parse().ok()).unwrap_or(0);
    (kb("VmRSS:"), kb("VmHWM:"), wb)
}

/// DuckDB's worker threads inherit the process name, so the main thread is
/// told apart by its id (tid == pid) rather than by name.
fn is_bench_thread(comm: &str, tid: u32, pid: u32) -> bool {
    comm.starts_with("gen-") || comm.starts_with("tokio-") || tid == pid
}

/// (db ticks, bench ticks): with an in-process DB the split is by thread name,
/// otherwise every thread is the bench.
fn self_cpu_ticks(in_process: bool) -> (u64, u64) {
    let mut db = 0u64;
    let mut bench = 0u64;
    let tasks = match std::fs::read_dir("/proc/self/task") {
        Ok(t) => t,
        Err(_) => return (0, 0),
    };
    let pid = std::process::id();
    for t in tasks.flatten() {
        let tid: u32 = t.file_name().to_string_lossy().parse().unwrap_or(0);
        let comm = std::fs::read_to_string(t.path().join("comm")).unwrap_or_default();
        let stat = std::fs::read_to_string(t.path().join("stat")).unwrap_or_default();
        let after = match stat.rfind(')') {
            Some(i) => &stat[i + 1..],
            None => continue,
        };
        let f: Vec<&str> = after.split_whitespace().collect();
        let ticks: u64 = f.get(11).and_then(|v| v.parse::<u64>().ok()).unwrap_or(0) + f.get(12).and_then(|v| v.parse::<u64>().ok()).unwrap_or(0);
        if !in_process || is_bench_thread(comm.trim(), tid, pid) {
            bench += ticks;
        } else {
            db += ticks;
        }
    }
    (db, bench)
}

pub fn affinity_cpus() -> usize {
    #[cfg(target_os = "linux")]
    {
        unsafe {
            let mut set: libc::cpu_set_t = std::mem::zeroed();
            if libc::sched_getaffinity(0, std::mem::size_of::<libc::cpu_set_t>(), &mut set) == 0 {
                return libc::CPU_COUNT(&set) as usize;
            }
        }
    }
    std::thread::available_parallelism().map(|n| n.get()).unwrap_or(1)
}
