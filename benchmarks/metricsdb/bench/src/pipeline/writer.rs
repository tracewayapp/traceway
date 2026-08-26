use std::sync::atomic::{AtomicI64, AtomicU64, AtomicUsize, Ordering};
use std::sync::{Arc, Mutex};
use std::time::{Duration, Instant};

use hdrhistogram::Histogram;
use serde::Serialize;

use crate::model::catalog::SeriesCatalog;
use crate::pipeline::batch::Batch;
use crate::sink::{SinkWriter, WriteError};

#[derive(Clone, Debug, Serialize)]
pub struct MaintEvent {
    pub kind: String,
    pub ms: u64,
}

pub struct Accounting {
    pub acked: AtomicU64,
    pub failed: AtomicU64,
    pub bytes_sent: AtomicU64,
    pub batches: AtomicU64,
    pub retries: AtomicU64,
    pub errors: AtomicU64,
    pub writer_idle_ns: AtomicU64,
    pub writer_busy_ns: AtomicU64,
    pub last_acked_ts_ms: AtomicI64,
    pub inflight: AtomicUsize,
    pub write_hist: Mutex<Histogram<u64>>,
    pub maintenance: Mutex<Vec<MaintEvent>>,
    pub last_error: Mutex<Option<String>>,
    pub first_ack: Mutex<Option<Instant>>,
    pub last_ack: Mutex<Option<Instant>>,
}

impl Default for Accounting {
    fn default() -> Self {
        Self {
            acked: AtomicU64::new(0),
            failed: AtomicU64::new(0),
            bytes_sent: AtomicU64::new(0),
            batches: AtomicU64::new(0),
            retries: AtomicU64::new(0),
            errors: AtomicU64::new(0),
            writer_idle_ns: AtomicU64::new(0),
            writer_busy_ns: AtomicU64::new(0),
            last_acked_ts_ms: AtomicI64::new(i64::MIN),
            inflight: AtomicUsize::new(0),
            write_hist: Mutex::new(Histogram::new_with_bounds(1, 3_600_000_000, 3).expect("histogram bounds")),
            maintenance: Mutex::new(Vec::new()),
            last_error: Mutex::new(None),
            first_ack: Mutex::new(None),
            last_ack: Mutex::new(None),
        }
    }
}

impl Accounting {
    pub fn record_latency(&self, d: Duration) {
        let us = (d.as_micros() as u64).clamp(1, 3_600_000_000);
        let _ = self.write_hist.lock().unwrap().record(us);
    }

    pub fn take_hist(&self) -> Histogram<u64> {
        let mut g = self.write_hist.lock().unwrap();
        let out = g.clone();
        g.reset();
        out
    }

    pub fn take_maintenance(&self) -> Vec<MaintEvent> {
        std::mem::take(&mut *self.maintenance.lock().unwrap())
    }
}

pub async fn run_writer(
    idx: usize,
    rx: async_channel::Receiver<Batch>,
    mut writer: Box<dyn SinkWriter>,
    acct: Arc<Accounting>,
    cat: Arc<SeriesCatalog>,
    max_retries: u32,
) {
    loop {
        let t_idle = Instant::now();
        let batch = match rx.recv().await {
            Ok(b) => b,
            Err(_) => break,
        };
        acct.writer_idle_ns.fetch_add(t_idle.elapsed().as_nanos() as u64, Ordering::Relaxed);
        acct.inflight.fetch_add(1, Ordering::Relaxed);
        let t0 = Instant::now();
        let mut attempt = 0u32;
        loop {
            match writer.write(&batch, &cat).await {
                Ok(ack) => {
                    acct.acked.fetch_add(batch.len() as u64, Ordering::Relaxed);
                    acct.bytes_sent.fetch_add(ack.bytes_sent, Ordering::Relaxed);
                    acct.batches.fetch_add(1, Ordering::Relaxed);
                    acct.last_acked_ts_ms.fetch_max(batch.max_ts_ms, Ordering::Relaxed);
                    acct.record_latency(t0.elapsed());
                    let now = Instant::now();
                    acct.first_ack.lock().unwrap().get_or_insert(t0);
                    *acct.last_ack.lock().unwrap() = Some(now);
                    if !ack.maintenance.is_empty() {
                        acct.maintenance.lock().unwrap().extend(ack.maintenance);
                    }
                    break;
                }
                Err(WriteError { retryable, msg }) => {
                    acct.errors.fetch_add(1, Ordering::Relaxed);
                    *acct.last_error.lock().unwrap() = Some(format!("writer {idx}: {msg}"));
                    if retryable && attempt < max_retries {
                        attempt += 1;
                        acct.retries.fetch_add(1, Ordering::Relaxed);
                        let backoff = Duration::from_millis(200u64.saturating_mul(1 << attempt.min(5))).min(Duration::from_secs(5));
                        tokio::time::sleep(backoff).await;
                        continue;
                    }
                    acct.failed.fetch_add(batch.len() as u64, Ordering::Relaxed);
                    acct.batches.fetch_add(1, Ordering::Relaxed);
                    break;
                }
            }
        }
        acct.writer_busy_ns.fetch_add(t0.elapsed().as_nanos() as u64, Ordering::Relaxed);
        acct.inflight.fetch_sub(1, Ordering::Relaxed);
    }
    if let Err(e) = writer.finish().await {
        *acct.last_error.lock().unwrap() = Some(format!("writer {idx} finish: {e}"));
    }
}
