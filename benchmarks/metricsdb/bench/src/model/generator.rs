use std::sync::atomic::{AtomicBool, AtomicI64, AtomicU64, AtomicUsize, Ordering};
use std::sync::{Arc, Barrier};
use std::thread::JoinHandle;
use std::time::{Duration, Instant};

use xxhash_rust::xxh3::xxh3_64;

use super::catalog::{SeriesCatalog, SENTINEL_ID};
use super::templates::TEMPLATES;
use super::values::ValueState;
use crate::pipeline::batch::Batch;

pub struct GenConfig {
    pub threads: usize,
    pub max_rounds: u64,
}

/// Shared with the orchestrator, which changes `rate_pps` and `batch_points`
/// between ramp steps; 0 = no rate cap (saturation).
#[derive(Default)]
pub struct GenShared {
    pub stop: AtomicBool,
    pub produced: AtomicU64,
    pub stall_ns: AtomicU64,
    pub credits: AtomicI64,
    pub rounds_done: AtomicU64,
    pub seq: AtomicU64,
    pub rate_pps: AtomicU64,
    pub batch_points: AtomicUsize,
}

impl GenShared {
    pub fn with(rate_pps: u64, batch_points: usize) -> Self {
        let g = Self::default();
        g.rate_pps.store(rate_pps, Ordering::Relaxed);
        g.batch_points.store(batch_points.max(1), Ordering::Relaxed);
        g
    }

    /// Cap batches at one second of the rate so a low rate still delivers a
    /// batch every second instead of one huge batch every few seconds.
    pub fn set_rate(&self, rate_pps: u64, configured_batch: usize) {
        self.rate_pps.store(rate_pps, Ordering::Relaxed);
        let batch = if rate_pps > 0 { configured_batch.min((rate_pps as usize).max(1_000)) } else { configured_batch };
        self.batch_points.store(batch.max(1), Ordering::Relaxed);
        let cap = rate_pps as i64;
        if cap > 0 && self.credits.load(Ordering::Relaxed) > cap {
            self.credits.store(cap, Ordering::Relaxed);
        }
    }
}

pub struct GenResult {
    pub fingerprint: u64,
    pub points: u64,
}

#[inline]
pub fn point_hash(id: u64, ts: i64, v: f64) -> u64 {
    let mut b = [0u8; 24];
    b[..8].copy_from_slice(&id.to_le_bytes());
    b[8..16].copy_from_slice(&ts.to_le_bytes());
    b[16..].copy_from_slice(&v.to_bits().to_le_bytes());
    xxh3_64(&b)
}

pub fn spawn_generators(
    cfg: &GenConfig,
    cat: Arc<SeriesCatalog>,
    tx: async_channel::Sender<Batch>,
    shared: Arc<GenShared>,
) -> Vec<JoinHandle<GenResult>> {
    let threads = cfg.threads.max(1);
    let barrier = Arc::new(Barrier::new(threads));
    let interval_s = cat.interval_ms as f64 / 1000.0;
    (0..threads)
        .map(|g| {
            let cat = Arc::clone(&cat);
            let tx = tx.clone();
            let shared = Arc::clone(&shared);
            let barrier = Arc::clone(&barrier);
            let max_rounds = cfg.max_rounds.min(cat.rounds);
            std::thread::Builder::new()
                .name(format!("gen-{g}"))
                .spawn(move || {
                    let local: Vec<usize> = (0..cat.hosts.len()).filter(|h| h % threads == g).collect();
                    let mut states: Vec<Vec<ValueState>> = local
                        .iter()
                        .map(|&h| {
                            cat.hosts[h]
                                .series
                                .iter()
                                .map(|&id| {
                                    let t = &TEMPLATES[cat.series[id as usize].template as usize];
                                    ValueState::new(&t.value, cat.seed ^ id.wrapping_mul(0x2545_F491_4F6C_DD1D))
                                })
                                .collect()
                        })
                        .collect();
                    let mut fp = 0u64;
                    let mut points = 0u64;
                    let mut batch_points = shared.batch_points.load(Ordering::Relaxed).max(1);
                    let mut batch = Batch::with_capacity(shared.seq.fetch_add(1, Ordering::Relaxed), batch_points);
                    'rounds: for r in 0..max_rounds {
                        barrier.wait();
                        if shared.stop.load(Ordering::Relaxed) {
                            break;
                        }
                        let ts_base = cat.sim_start_ms + r as i64 * cat.interval_ms;
                        if g == 0 {
                            batch.push(SENTINEL_ID, ts_base, r as f64);
                            fp ^= point_hash(SENTINEL_ID, ts_base, r as f64);
                            points += 1;
                        }
                        for (li, &h) in local.iter().enumerate() {
                            let host = &cat.hosts[h];
                            let ts = ts_base + host.jitter_ms;
                            let st = &mut states[li];
                            for (k, &id) in host.series.iter().enumerate() {
                                let t = &TEMPLATES[cat.series[id as usize].template as usize];
                                let v = st[k].next(&t.value, interval_s, host.quiet, t.precision);
                                batch.push(id, ts, v);
                                fp ^= point_hash(id, ts, v);
                                points += 1;
                                if batch.len() >= batch_points {
                                    batch_points = shared.batch_points.load(Ordering::Relaxed).max(1);
                                    let full = std::mem::replace(&mut batch, Batch::with_capacity(shared.seq.fetch_add(1, Ordering::Relaxed), batch_points));
                                    if !send(&tx, &shared, full) {
                                        break 'rounds;
                                    }
                                }
                            }
                        }
                        shared.rounds_done.fetch_max(r + 1, Ordering::Relaxed);
                    }
                    if !batch.is_empty() {
                        let _ = send(&tx, &shared, batch);
                    }
                    GenResult { fingerprint: fp, points }
                })
                .expect("spawn generator thread")
        })
        .collect()
}

fn send(tx: &async_channel::Sender<Batch>, shared: &GenShared, batch: Batch) -> bool {
    let n = batch.len() as u64;
    loop {
        if shared.rate_pps.load(Ordering::Relaxed) == 0 {
            break;
        }
        if shared.stop.load(Ordering::Relaxed) {
            return false;
        }
        if shared.credits.load(Ordering::Relaxed) >= n as i64 {
            shared.credits.fetch_sub(n as i64, Ordering::Relaxed);
            break;
        }
        std::thread::sleep(Duration::from_millis(1));
    }
    let t0 = Instant::now();
    let ok = tx.send_blocking(batch).is_ok();
    shared.stall_ns.fetch_add(t0.elapsed().as_nanos() as u64, Ordering::Relaxed);
    if ok {
        shared.produced.fetch_add(n, Ordering::Relaxed);
    }
    ok
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::model::catalog::CatalogConfig;

    fn run(threads: usize, batch_points: usize) -> (u64, u64) {
        let cat = Arc::new(SeriesCatalog::build(&CatalogConfig { series: 5_000, points: 5_000 * 7, interval_ms: 10_000, sim_start_ms: 0, seed: 7 }));
        let (tx, rx) = async_channel::bounded::<Batch>(64);
        let shared = Arc::new(GenShared::with(0, batch_points));
        let handles = spawn_generators(&GenConfig { threads, max_rounds: u64::MAX }, cat, tx, shared);
        let drain = std::thread::spawn(move || {
            let mut n = 0u64;
            while let Ok(b) = rx.recv_blocking() {
                n += b.len() as u64;
            }
            n
        });
        let mut fp = 0u64;
        let mut points = 0u64;
        for h in handles {
            let r = h.join().unwrap();
            fp ^= r.fingerprint;
            points += r.points;
        }
        assert_eq!(drain.join().unwrap(), points);
        (fp, points)
    }

    #[test]
    fn fingerprint_independent_of_threads_and_batch_size() {
        let a = run(1, 1000);
        let b = run(3, 777);
        let c = run(2, 100_000);
        assert_eq!(a, b);
        assert_eq!(a, c);
        assert_eq!(a.1, 5_000 * 7);
    }
}
