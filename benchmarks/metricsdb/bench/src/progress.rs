use std::io::Write;
use std::sync::Mutex;

use crate::report::Window;
use crate::util::{fmt_bytes, fmt_count};

static LOCK: Mutex<()> = Mutex::new(());

pub fn log(line: impl AsRef<str>) {
    let _g = LOCK.lock().unwrap();
    let mut e = std::io::stderr().lock();
    let _ = writeln!(e, "{}", line.as_ref());
}

pub fn window_line(w: &Window, writers: usize, verdict: &str) -> String {
    let lag = w.visibility.as_ref().map(|v| format!("{:.0}s({})", v.lag_sim_s, fmt_count(v.points_behind))).unwrap_or_else(|| "-".into());
    let q = w
        .queries
        .iter()
        .map(|q| format!("{}:{:.0}ms/{}", q.id.as_str(), q.ms, serde_json::to_string(&q.status).unwrap_or_default().trim_matches('"')))
        .collect::<Vec<_>>()
        .join(",");
    let rss = w.proc.as_ref().map(|p| fmt_bytes(p.rss_bytes)).unwrap_or_else(|| "-".into());
    let cpu = w.proc.as_ref().map(|p| format!("{:.0}%", p.cpu_pct)).unwrap_or_else(|| "-".into());
    let disk = w
        .disk
        .as_ref()
        .map(|d| format!("{}({:.1}B/pt)", fmt_bytes(d.total_bytes), if w.acked_total > 0 { d.total_bytes as f64 / w.acked_total as f64 } else { 0.0 }))
        .unwrap_or_else(|| "-".into());
    format!(
        "w={:04} t={}s phase={} acked={} pps={} p50={:.0}ms p99={:.0}ms inflight={}/{} lag={} q=[{}] rss={} cpu={} disk={} stall={:.0}% idle={:.0}% err={} verdict={}",
        w.w,
        w.t_s,
        w.phase,
        fmt_count(w.acked_total as f64),
        fmt_count(w.acked_pps),
        w.write_ms.p50,
        w.write_ms.p99,
        w.inflight,
        writers,
        lag,
        q,
        rss,
        cpu,
        disk,
        w.gen_stall_pct,
        w.writer_idle_pct,
        w.errors,
        verdict
    )
}
