mod cli;
mod model;
mod pipeline;
mod probe;
mod progress;
mod queries;
mod report;
mod sink;
mod util;
mod verdict;

use std::collections::BTreeMap;
use std::future::Future;
use std::process::ExitCode;
use std::sync::atomic::{AtomicBool, AtomicI64, AtomicU8, Ordering};
use std::sync::{Arc, Mutex};
use std::time::{Duration, Instant};

use clap::Parser;
use serde_json::Value;

use crate::cli::{Cli, Mode};
use crate::model::catalog::{CatalogConfig, SeriesCatalog};
use crate::model::generator::{spawn_generators, GenConfig, GenShared};
use crate::pipeline::batch::Batch;
use crate::pipeline::writer::{run_writer, Accounting};
use crate::probe::disk::DiskSample;
use crate::probe::proc::{ProcSample, ProcSampler};
use crate::probe::ContainerState;
use crate::progress::log;
use crate::queries::{QueryId, QueryInstance, QueryOutcome, QueryParams, QueryStatus};
use crate::report::*;
use crate::sink::{DbKind, Sink};
use crate::util::{fmt_count, fmt_rfc3339_ms, now_ms, now_rfc3339};
use crate::verdict::{VerdictCfg, VerdictState};

const PH_WARMUP: u8 = 0;
const PH_RAMP: u8 = 1;
const PH_INGEST: u8 = 2;
const PH_DRAIN: u8 = 3;
const PH_SETTLE: u8 = 4;
const PH_COLD: u8 = 5;
const PH_DONE: u8 = 6;

fn phase_name(p: u8) -> &'static str {
    match p {
        PH_WARMUP => "warmup",
        PH_RAMP => "ramp",
        PH_INGEST => "ingest",
        PH_DRAIN => "drain",
        PH_SETTLE => "settle",
        PH_COLD => "cold",
        _ => "done",
    }
}

#[derive(Default)]
struct ProbeState {
    health: Option<Value>,
    proc: Option<ProcSample>,
    container: Option<ContainerState>,
    disk: Option<DiskSample>,
    disk_seq: u64,
    reachable: bool,
    last_started_at: Option<String>,
    restarted: bool,
}

struct Shared {
    phase: AtomicU8,
    interrupted: AtomicBool,
    probe: Mutex<ProbeState>,
    pending_outcomes: Mutex<Vec<QueryOutcome>>,
    all_outcomes: Mutex<Vec<QueryOutcome>>,
    visible_ts: AtomicI64,
}

impl Shared {
    fn phase(&self) -> u8 {
        self.phase.load(Ordering::Relaxed)
    }
    fn set_phase(&self, p: u8) {
        self.phase.store(p, Ordering::Relaxed);
    }
}

fn num(v: Option<&Value>) -> Option<f64> {
    match v? {
        Value::Number(n) => n.as_f64(),
        Value::String(s) => s.trim().parse().ok(),
        _ => None,
    }
}

/// The store's deferred work: what an LSM-style store can let pile up while
/// still acknowledging every write.
fn debt_metric(kind: DbKind) -> &'static str {
    match kind {
        DbKind::Clickhouse | DbKind::ClickhouseMap => "active parts",
        DbKind::Victoriametrics => "small + in-memory parts",
        DbKind::Duckdb => "WAL MB",
        DbKind::Firebolt => "tablets",
    }
}

/// Lower quartile: merges, checkpoints and vacuums make the debt a sawtooth,
/// and only its floor rising means the store is not keeping up.
fn debt_floor(it: impl Iterator<Item = f64>) -> Option<f64> {
    let mut v: Vec<f64> = it.collect();
    if v.is_empty() {
        return None;
    }
    v.sort_by(|a, b| a.partial_cmp(b).unwrap());
    Some(v[(v.len() - 1) / 4])
}

fn debt_of(kind: DbKind, h: &Value) -> Option<f64> {
    match kind {
        DbKind::Clickhouse | DbKind::ClickhouseMap => num(h.get("active_parts")),
        DbKind::Victoriametrics => Some(num(h.get("parts_small"))? + num(h.get("parts_inmemory"))?),
        DbKind::Duckdb => num(h.get("wal_bytes")).map(|b| b / 1e6),
        DbKind::Firebolt => num(h.get("number_of_tablets")),
    }
}

fn main() -> ExitCode {
    let cli = Cli::parse();
    let rt = tokio::runtime::Builder::new_multi_thread().enable_all().thread_name("tokio-w").build().expect("tokio runtime");
    let result = rt.block_on(run(cli));
    rt.shutdown_timeout(Duration::from_secs(5));
    match result {
        Ok(code) => ExitCode::from(code),
        Err(e) => {
            log(format!("error: {e:#}"));
            ExitCode::from(3)
        }
    }
}

async fn run(cli: Cli) -> anyhow::Result<u8> {
    let run_id = cli.run_id.clone().unwrap_or_else(|| format!("local-{}", now_ms()));
    let sim_start_ms = cli.sim_start_ms();
    let t_build = Instant::now();
    let cat = Arc::new(SeriesCatalog::build(&CatalogConfig { series: cli.series, points: cli.points, interval_ms: cli.interval.as_millis() as i64, sim_start_ms, seed: cli.seed }));
    let catalog_build_ms = t_build.elapsed().as_millis() as u64;
    log(format!(
        "setup: catalog series={} hosts={} pods={} templates={} rounds={} sim={}..{} logical_bytes_per_point={:.1} build={}ms",
        cat.len(),
        cat.hosts.len(),
        cat.pods,
        cat.metrics.len(),
        cat.rounds,
        fmt_rfc3339_ms(cat.sim_start_ms),
        fmt_rfc3339_ms(cat.sim_end_ms()),
        cat.logical_bytes_per_point,
        catalog_build_ms
    ));

    if let Some(rounds) = cli.verify_gen {
        return verify_gen(&cli, cat, rounds);
    }

    let kind = cli.db.ok_or_else(|| anyhow::anyhow!("--db is required (victoriametrics, clickhouse, clickhouse-map, duckdb, firebolt)"))?;
    let defaults = kind.defaults();
    let writers = cli.writers.unwrap_or(defaults.writers).max(1);
    let batch_points = cli.batch_points.unwrap_or(defaults.batch_points).max(1);
    let sink: Arc<dyn Sink> = Arc::from(sink::make_sink(&cli)?);
    let mode = cli.mode;
    let ladder = cli.ramp_ladder()?;

    let mut report = Report {
        schema_version: 2,
        db: kind.name().into(),
        family: kind.family().into(),
        variant: kind.variant().into(),
        tier: cli.tier.clone(),
        run_id: run_id.clone(),
        started_at: now_rfc3339(),
        ended_at: None,
        bench: BenchInfo {
            version: env!("CARGO_PKG_VERSION").into(),
            cpus_visible: probe::proc::affinity_cpus(),
            gen_threads: cli.gen_threads,
            writers,
            batch_points,
            rate_cap: cli.rate,
            window_s: cli.window.as_secs(),
            data_fingerprint: None,
        },
        db_config: DbConfig {
            container: cli.db_container.clone(),
            cgroup: cli.cgroup.as_ref().map(|p| p.display().to_string()),
            data_dir: cli.data_dir.display().to_string(),
            settings: sink.settings(),
        },
        series_model: SeriesModel {
            series: cat.len(),
            hosts: cat.hosts.len(),
            pods: cat.pods,
            templates: cat.metrics.len(),
            interval_ms: cat.interval_ms,
            rounds: cat.rounds,
            points_planned: cat.rounds * cat.len(),
            sim_start: fmt_rfc3339_ms(cat.sim_start_ms),
            sim_end: fmt_rfc3339_ms(cat.sim_end_ms()),
            seed: cat.seed,
            avg_tags_per_series: cat.series.iter().map(|s| s.tags.len() as f64).sum::<f64>() / cat.len() as f64,
            logical_bytes_per_point: cat.logical_bytes_per_point,
            catalog_build_ms,
            catalog_load_ms: 0,
        },
        ack_semantics: sink.ack_semantics().into(),
        debt_metric: debt_metric(kind).into(),
        ramp: None,
        timeline: Vec::new(),
        phases: Phases { warmup_s: if mode == Mode::Saturate { cli.warmup.as_secs() } else { 0 }, ..Default::default() },
        throughput: Throughput { mode: mode.name().into(), ..Default::default() },
        queries_during_ingest: BTreeMap::new(),
        queries_cold: BTreeMap::new(),
        cold_method: if cli.no_cold { "skipped".into() } else if cli.cold_hook.is_some() { "cold_hook+drop_caches".into() } else { "reopen+drop_caches".into() },
        disk: DiskReport::default(),
        verdict: Default::default(),
        interrupted: false,
        notes: Vec::new(),
    };
    report.verdict.stopped_reason = "setup".into();
    report.write_atomic(&cli.out)?;

    let t_setup = Instant::now();
    if let Err(e) = sink.setup(cli.reset, cli.setup_timeout).await {
        report.verdict.stopped_reason = "setup_failed".into();
        report.notes.push(format!("setup failed: {e:#}"));
        report.ended_at = Some(now_rfc3339());
        report.write_atomic(&cli.out)?;
        log(format!("setup: FAILED {e:#}"));
        return Ok(3);
    }
    log(format!("setup: {} ready in {}ms, loading {} series into the dimension table", kind.name(), t_setup.elapsed().as_millis(), cat.len()));
    let t_cat = Instant::now();
    if let Err(e) = sink.register_series(&cat).await {
        report.verdict.stopped_reason = "setup_failed".into();
        report.notes.push(format!("series registration failed: {e:#}"));
        report.ended_at = Some(now_rfc3339());
        report.write_atomic(&cli.out)?;
        log(format!("setup: series registration FAILED {e:#}"));
        return Ok(3);
    }
    report.series_model.catalog_load_ms = t_cat.elapsed().as_millis() as u64;
    report.phases.setup_ms = t_setup.elapsed().as_millis() as u64;
    log(format!("setup: series loaded in {}ms", report.series_model.catalog_load_ms));

    let shared = Arc::new(Shared {
        phase: AtomicU8::new(if mode == Mode::Saturate { PH_WARMUP } else { PH_RAMP }),
        interrupted: AtomicBool::new(false),
        probe: Mutex::new(ProbeState::default()),
        pending_outcomes: Mutex::new(Vec::new()),
        all_outcomes: Mutex::new(Vec::new()),
        visible_ts: AtomicI64::new(0),
    });
    let initial_rate = if mode == Mode::Saturate { cli.rate } else { ladder[0] };
    let gen = Arc::new(GenShared::with(initial_rate, batch_points));
    gen.set_rate(initial_rate, batch_points);
    let acct = Arc::new(Accounting::default());

    {
        let shared = Arc::clone(&shared);
        let gen = Arc::clone(&gen);
        tokio::spawn(async move {
            #[cfg(unix)]
            {
                let mut term = tokio::signal::unix::signal(tokio::signal::unix::SignalKind::terminate()).expect("sigterm handler");
                tokio::select! { _ = tokio::signal::ctrl_c() => {}, _ = term.recv() => {} }
            }
            #[cfg(not(unix))]
            {
                let _ = tokio::signal::ctrl_c().await;
            }
            log("signal: stopping ingest, the report will be flushed");
            shared.interrupted.store(true, Ordering::Relaxed);
            gen.stop.store(true, Ordering::Relaxed);
        });
    }

    let params = Arc::new(QueryParams::from_catalog(&cat));
    let probe_handle = tokio::spawn(probe_task(Arc::clone(&sink), cli.clone(), Arc::clone(&shared), kind == DbKind::Duckdb));
    let query_handle = if cli.no_queries { None } else { Some(tokio::spawn(query_task(Arc::clone(&sink), cli.clone(), Arc::clone(&shared), Arc::clone(&params)))) };

    let (tx, rx) = async_channel::bounded::<Batch>((2 * writers).max(2));
    let mut writer_handles = Vec::with_capacity(writers);
    for i in 0..writers {
        let w = sink.writer(i).await?;
        writer_handles.push(tokio::spawn(run_writer(i, rx.clone(), w, Arc::clone(&acct), Arc::clone(&cat), cli.max_retries)));
    }
    {
        let gen = Arc::clone(&gen);
        tokio::spawn(async move {
            let mut iv = tokio::time::interval(Duration::from_millis(10));
            loop {
                iv.tick().await;
                let rate = gen.rate_pps.load(Ordering::Relaxed);
                if rate == 0 {
                    continue;
                }
                let cap = rate as i64;
                let add = (rate / 100).max(1) as i64;
                let cur = gen.credits.load(Ordering::Relaxed);
                if cur + add <= cap {
                    gen.credits.fetch_add(add, Ordering::Relaxed);
                } else if cur < cap {
                    gen.credits.store(cap, Ordering::Relaxed);
                }
            }
        });
    }
    let t_ingest = Instant::now();
    let gen_handles = spawn_generators(&GenConfig { threads: cli.gen_threads, max_rounds: u64::MAX }, Arc::clone(&cat), tx, Arc::clone(&gen));
    log(format!(
        "ingest: started mode={} writers={writers} batch_points={batch_points} gen_threads={} target_points={} initial_rate={}",
        mode.name(),
        cli.gen_threads,
        fmt_count((cat.rounds * cat.len()) as f64),
        if initial_rate == 0 { "saturation".to_string() } else { format!("{}/s", fmt_count(initial_rate as f64)) }
    ));

    let mut ctx = Ctx {
        cli: &cli,
        kind,
        cat: Arc::clone(&cat),
        acct: Arc::clone(&acct),
        gen: Arc::clone(&gen),
        shared: Arc::clone(&shared),
        report,
        verdict: VerdictState::new(VerdictCfg {
            lag_threshold_s: cli.lag_threshold.as_secs_f64(),
            degrade_ratio: cli.degrade_ratio,
            query_slow_ms: cli.query_slow_ms as f64,
            write_err_ratio: cli.write_err_ratio,
            interval_s: cat.interval_ms as f64 / 1000.0,
            debt_windows: (cli.debt_window.as_secs() / cli.window.as_secs().max(1)).max(2) as usize,
            debt_growth: cli.debt_growth,
            debt_min_delta: cli.debt_min_delta,
            debt_metric: debt_metric(kind).into(),
        }),
        t0: t_ingest,
        prev: Prev::default(),
        writers,
        batch_points,
        last_disk_seq: 0,
        starved_windows: 0,
        ingest_windows: 0,
        prev_delayed: None,
        last_debt: None,
        fill_start: None,
        gen_handles,
        rx,
        ctl: IngestCtl::new(cli.window),
    };

    match mode {
        Mode::Saturate => {
            ctx.report.throughput.fill_rate_pps = cli.rate;
            ctx.fill_start = Some((t_ingest, 0));
            ctx.run_until_done().await;
        }
        Mode::Ramp | Mode::RampThenFill => {
            let ramp = ctx.run_ramp().await;
            let aborted = ramp.aborted;
            let fill_rate = ramp.fill_rate_pps;
            ctx.report.throughput.sustainable_pps = ramp.sustainable_pps;
            ctx.report.ramp = Some(ramp);
            if aborted {
                ctx.report.notes.push("the ramp did not finish (points exhausted, deadline or interrupt); no fill phase".into());
            } else if mode == Mode::RampThenFill {
                log(format!(
                    "fill: rate={} for the remaining points",
                    if fill_rate == 0 { "saturation".to_string() } else { format!("{}/s", fmt_count(fill_rate as f64)) }
                ));
                ctx.gen.set_rate(fill_rate, batch_points);
                ctx.report.throughput.fill_rate_pps = fill_rate;
                ctx.shared.set_phase(PH_INGEST);
                ctx.fill_start = Some((Instant::now(), ctx.acct.acked.load(Ordering::Relaxed)));
                ctx.run_until_done().await;
            } else {
                ctx.report.verdict.stopped_reason = "ramp_complete".into();
                ctx.gen.stop.store(true, Ordering::Relaxed);
                ctx.ctl.stopping = true;
                ctx.ctl.stop_started = Some(Instant::now());
                ctx.run_until_done().await;
            }
        }
    }
    ctx.report.phases.ingest_s = t_ingest.elapsed().as_secs();
    shared.set_phase(PH_DRAIN);
    ctx.report.interrupted = shared.interrupted.load(Ordering::Relaxed);

    let t_drain = Instant::now();
    let gen_handles = std::mem::take(&mut ctx.gen_handles);
    if gen_handles.iter().all(|h| h.is_finished()) {
        let mut fp = 0u64;
        for h in gen_handles {
            if let Ok(r) = h.join() {
                fp ^= r.fingerprint;
            }
        }
        ctx.report.bench.data_fingerprint = Some(format!("xxh3:{fp:016x}"));
    } else {
        ctx.report.notes.push("generator threads were still blocked at drain; fingerprint unavailable".into());
    }
    let writers_done = async {
        for h in writer_handles {
            let _ = h.await;
        }
    };
    if tokio::time::timeout(cli.drain_timeout, ctx.tick_until(writers_done)).await.is_err() {
        ctx.report.notes.push("writers did not finish within --drain-timeout".into());
    }
    if let Some(q) = query_handle {
        let _ = tokio::time::timeout(cli.query_deadline + Duration::from_secs(5), q).await;
    }
    ctx.report.phases.drain_ms = t_drain.elapsed().as_millis() as u64;
    ctx.finalize_throughput();
    let ingest_end = ctx.acct.last_ack.lock().unwrap().unwrap_or_else(Instant::now);
    ctx.report.write_atomic(&cli.out)?;
    log(format!(
        "ingest: done acked={} lost={} sustainable={} fill={}/s late={}/s fingerprint={}",
        fmt_count(ctx.report.throughput.acked_points as f64),
        ctx.report.throughput.points_lost,
        ctx.report.throughput.sustainable_pps.map(|v| format!("{}/s", fmt_count(v))).unwrap_or_else(|| "-".into()),
        fmt_count(ctx.report.throughput.fill_pps),
        fmt_count(ctx.report.throughput.late_pps),
        ctx.report.bench.data_fingerprint.clone().unwrap_or_default()
    ));

    shared.set_phase(PH_SETTLE);
    let reachable = sink.is_reachable().await;
    if reachable {
        let t_settle = Instant::now();
        let deadline = t_settle + cli.max_settle;
        let rep = ctx.tick_until(sink.settle(deadline)).await;
        let ms = t_settle.elapsed().as_millis() as u64;
        for s in &rep.steps {
            log(format!("settle [{}]: {} ok={} {}ms {}", kind.name(), s.name, s.ok, s.ms, s.detail));
        }
        ctx.report.phases.settle = Some(Report::settle_from(rep, ms));
    } else {
        ctx.report.notes.push("database unreachable after ingest; settle and cold phases skipped".into());
    }

    // Stores delete merged-away parts and vacuumed tablets asynchronously;
    // measure only once the directory has stopped shrinking.
    let walk_dir = || {
        let dir = cli.data_dir.clone();
        let sink = Arc::clone(&sink);
        tokio::task::spawn_blocking(move || probe::disk::walk(&dir, &|rel| sink.disk_class(rel)))
    };
    let mut disk = walk_dir().await.unwrap_or_default();
    let gc_deadline = Instant::now() + cli.max_settle.min(Duration::from_secs(180));
    let mut stable = 0;
    while stable < 2 && Instant::now() < gc_deadline {
        tokio::time::sleep(Duration::from_secs(5)).await;
        let again = walk_dir().await.unwrap_or_default();
        if again.total_bytes == disk.total_bytes {
            stable += 1;
        } else {
            stable = 0;
        }
        disk = again;
    }
    if stable < 2 {
        ctx.report.notes.push("data dir was still shrinking when the disk measure was taken".into());
    }
    let digest_s = ingest_end.elapsed().as_secs_f64();
    ctx.report.throughput.digest_s = digest_s;
    let fill_wall = ctx.report.throughput.fill_wall_s;
    ctx.report.throughput.amortized_pps = if fill_wall + digest_s > 0.0 { ctx.report.throughput.fill_points as f64 / (fill_wall + digest_s) } else { 0.0 };
    let db_reported = if reachable { sink.db_size().await.unwrap_or_default() } else { Default::default() };
    let acked = ctx.report.throughput.acked_points.max(1);
    let mut disk = disk;
    if let Some(c) = db_reported.compressed_bytes.filter(|c| *c > disk.total_bytes.saturating_mul(2)) {
        ctx.report.notes.push(format!(
            "data dir walk saw {} but the store reports {} (bind mount not fully visible from the host?); disk figures use the store-reported size",
            util::fmt_bytes(disk.total_bytes),
            util::fmt_bytes(c)
        ));
        disk.total_bytes = c;
        disk.by_class.insert("db_reported".into(), c);
    }
    ctx.report.disk = DiskReport {
        total_bytes: disk.total_bytes,
        by_class: disk.by_class.clone(),
        bytes_per_point: disk.total_bytes as f64 / acked as f64,
        ratio_vs_raw16: if disk.total_bytes > 0 { (acked as f64 * 16.0) / disk.total_bytes as f64 } else { 0.0 },
        ratio_vs_logical: if disk.total_bytes > 0 { (acked as f64 * cat.logical_bytes_per_point) / disk.total_bytes as f64 } else { 0.0 },
        db_reported,
    };
    ctx.report.write_atomic(&cli.out)?;
    log(format!(
        "disk: {} total, {:.2} bytes/point, {:.1}x vs raw16, {:.1}x vs logical; digest={:.0}s amortized={}/s",
        util::fmt_bytes(disk.total_bytes),
        ctx.report.disk.bytes_per_point,
        ctx.report.disk.ratio_vs_raw16,
        ctx.report.disk.ratio_vs_logical,
        digest_s,
        fmt_count(ctx.report.throughput.amortized_pps)
    ));

    if !cli.no_cold && reachable && !shared.interrupted.load(Ordering::Relaxed) {
        shared.set_phase(PH_COLD);
        let t_cold = Instant::now();
        let cold = ctx.tick_until(cold_phase(Arc::clone(&sink), &cli, Arc::clone(&params), cat.sim_end_ms(), t_cold + cli.max_cold)).await;
        match cold {
            Ok(q) => ctx.report.queries_cold = q,
            Err(e) => ctx.report.notes.push(format!("cold phase failed: {e:#}")),
        }
        ctx.report.phases.cold_ms = t_cold.elapsed().as_millis() as u64;
    }

    shared.set_phase(PH_DONE);
    let _ = tokio::time::timeout(Duration::from_secs(15), probe_handle).await;
    ctx.report.ended_at = Some(now_rfc3339());
    ctx.report.write_atomic(&cli.out)?;
    log(format!("done: stopped_reason={} fell_behind_at={:?} unusable_at={:?} out={}", ctx.report.verdict.stopped_reason, ctx.report.verdict.fell_behind_at_s, ctx.report.verdict.unusable_at_s, cli.out.display()));
    Ok(if ctx.report.interrupted { 1 } else { 0 })
}

fn verify_gen(cli: &Cli, cat: Arc<SeriesCatalog>, rounds: u64) -> anyhow::Result<u8> {
    let (tx, rx) = async_channel::bounded::<Batch>(64);
    let shared = Arc::new(GenShared::with(0, cli.batch_points.unwrap_or(500_000)));
    let t0 = Instant::now();
    let handles = spawn_generators(&GenConfig { threads: cli.gen_threads, max_rounds: rounds }, cat, tx, Arc::clone(&shared));
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
        let r = h.join().map_err(|_| anyhow::anyhow!("generator panicked"))?;
        fp ^= r.fingerprint;
        points += r.points;
    }
    let drained = drain.join().map_err(|_| anyhow::anyhow!("drain panicked"))?;
    let secs = t0.elapsed().as_secs_f64();
    log(format!("verify-gen: rounds={rounds} points={points} drained={drained} fingerprint=xxh3:{fp:016x} elapsed={secs:.1}s gen_pps={}", fmt_count(points as f64 / secs.max(0.001))));
    Ok(if drained == points { 0 } else { 4 })
}

#[derive(Default)]
struct Prev {
    acked: u64,
    produced: u64,
    batches: u64,
    retries: u64,
    errors: u64,
    failed: u64,
    bytes: u64,
    stall_ns: u64,
    idle_ns: u64,
    at: Option<Instant>,
}

struct IngestCtl {
    interval: tokio::time::Interval,
    stopping: bool,
    stop_started: Option<Instant>,
    unusable_since: Option<u64>,
}

impl IngestCtl {
    fn new(window: Duration) -> Self {
        // interval() fires immediately; the first window must cover a full period.
        let mut interval = tokio::time::interval_at(tokio::time::Instant::now() + window, window);
        interval.set_missed_tick_behavior(tokio::time::MissedTickBehavior::Delay);
        Self { interval, stopping: false, stop_started: None, unusable_since: None }
    }
}

struct Ctx<'a> {
    cli: &'a Cli,
    kind: DbKind,
    cat: Arc<SeriesCatalog>,
    acct: Arc<Accounting>,
    gen: Arc<GenShared>,
    shared: Arc<Shared>,
    report: Report,
    verdict: VerdictState,
    t0: Instant,
    prev: Prev,
    writers: usize,
    batch_points: usize,
    last_disk_seq: u64,
    starved_windows: u64,
    ingest_windows: u64,
    prev_delayed: Option<f64>,
    last_debt: Option<f64>,
    fill_start: Option<(Instant, u64)>,
    gen_handles: Vec<std::thread::JoinHandle<crate::model::generator::GenResult>>,
    rx: async_channel::Receiver<Batch>,
    ctl: IngestCtl,
}

impl<'a> Ctx<'a> {
    fn record_window(&mut self) {
        let now = Instant::now();
        let dt = self.prev.at.map(|p| now.duration_since(p).as_secs_f64()).unwrap_or(self.cli.window.as_secs_f64()).max(0.001);
        let acked = self.acct.acked.load(Ordering::Relaxed);
        let produced = self.gen.produced.load(Ordering::Relaxed);
        let batches = self.acct.batches.load(Ordering::Relaxed);
        let retries = self.acct.retries.load(Ordering::Relaxed);
        let errors = self.acct.errors.load(Ordering::Relaxed);
        let failed = self.acct.failed.load(Ordering::Relaxed);
        let bytes = self.acct.bytes_sent.load(Ordering::Relaxed);
        let stall = self.gen.stall_ns.load(Ordering::Relaxed);
        let idle = self.acct.writer_idle_ns.load(Ordering::Relaxed);
        let hist = self.acct.take_hist();
        let pct = |q: f64| hist.value_at_quantile(q) as f64 / 1000.0;
        let phase = self.shared.phase();
        let window_ns = dt * 1e9;
        let gen_stall_pct = (stall - self.prev.stall_ns) as f64 / (window_ns * self.cli.gen_threads.max(1) as f64) * 100.0;
        let writer_idle_pct = (idle - self.prev.idle_ns) as f64 / (window_ns * self.writers as f64) * 100.0;
        let (health, proc, container, disk, reachable, restarted) = {
            let mut p = self.shared.probe.lock().unwrap();
            let disk = if p.disk_seq != self.last_disk_seq {
                self.last_disk_seq = p.disk_seq;
                p.disk.clone()
            } else {
                None
            };
            let restarted = std::mem::take(&mut p.restarted);
            (p.health.clone(), p.proc.clone(), p.container.clone(), disk, p.reachable, restarted)
        };
        let debt = health.as_ref().and_then(|h| debt_of(self.kind, h));
        if debt.is_some() {
            self.last_debt = debt;
        }
        let delayed_now = health.as_ref().and_then(|h| num(h.get("delayed_inserts")));
        let throttled = match (self.prev_delayed, delayed_now) {
            (Some(prev), Some(cur)) if cur > prev => (cur - prev) as u64,
            _ => 0,
        };
        if delayed_now.is_some() {
            self.prev_delayed = delayed_now;
        }
        let acked_pps = (acked - self.prev.acked) as f64 / dt;
        let last_acked_ts = self.acct.last_acked_ts_ms.load(Ordering::Relaxed);
        let visible_ts = self.shared.visible_ts.load(Ordering::Relaxed);
        let visibility = if phase < PH_DRAIN && visible_ts > 0 && last_acked_ts > i64::MIN {
            let lag_s = ((last_acked_ts - visible_ts).max(0)) as f64 / 1000.0;
            let points_behind = lag_s / (self.cat.interval_ms as f64 / 1000.0) * self.cat.len() as f64;
            Some(Visibility { last_acked_ts, visible_ts, lag_sim_s: lag_s, points_behind, lag_wall_s: if acked_pps > 0.0 { Some(points_behind / acked_pps) } else { None } })
        } else {
            None
        };
        let queries = std::mem::take(&mut *self.shared.pending_outcomes.lock().unwrap());
        let mut flags = Vec::new();
        let saturating = self.gen.rate_pps.load(Ordering::Relaxed) == 0;
        if phase == PH_INGEST && saturating && writer_idle_pct > 20.0 && gen_stall_pct < 5.0 {
            flags.push("generator_starved".to_string());
            self.starved_windows += 1;
        }
        if phase == PH_INGEST {
            self.ingest_windows += 1;
        }
        let w = Window {
            w: self.report.timeline.len() as u64 + 1,
            t_s: self.t0.elapsed().as_secs(),
            phase: phase_name(phase).into(),
            acked_points: acked - self.prev.acked,
            acked_total: acked,
            acked_pps,
            produced_pps: (produced - self.prev.produced) as f64 / dt,
            write_ms: Pct { p50: pct(0.5), p95: pct(0.95), p99: pct(0.99), max: hist.max() as f64 / 1000.0 },
            inflight: self.acct.inflight.load(Ordering::Relaxed),
            batches: batches - self.prev.batches,
            retries: retries - self.prev.retries,
            errors: errors - self.prev.errors,
            points_lost: failed - self.prev.failed,
            gen_stall_pct,
            writer_idle_pct,
            bytes_sent: bytes - self.prev.bytes,
            maintenance: self.acct.take_maintenance(),
            visibility,
            queries,
            debt,
            throttled,
            health,
            proc,
            container,
            disk,
            flags,
        };
        self.prev = Prev { acked, produced, batches, retries, errors, failed, bytes, stall_ns: stall, idle_ns: idle, at: Some(now) };
        self.verdict.observe(&w, reachable || phase >= PH_DRAIN, restarted);
        let verdict_tag = if self.verdict.verdict.unusable_at_s.is_some() {
            "UNUSABLE"
        } else if self.verdict.verdict.fell_behind_at_s.is_some() {
            "behind"
        } else {
            "ok"
        };
        log(progress::window_line(&w, self.writers, verdict_tag));
        self.report.timeline.push(w);
        self.report.verdict.fell_behind_at_s = self.verdict.verdict.fell_behind_at_s;
        self.report.verdict.fell_behind_reason = self.verdict.verdict.fell_behind_reason.clone();
        self.report.verdict.unusable_at_s = self.verdict.verdict.unusable_at_s;
        self.report.verdict.unusable_reason = self.verdict.verdict.unusable_reason.clone();
        self.report.verdict.process_restarts = self.verdict.verdict.process_restarts;
        self.report.throughput.plateau_pps = self.verdict.plateau_pps;
        self.report.throughput.acked_points = acked;
        self.report.throughput.points_lost = failed;
        self.update_throughput(acked);
        self.report.throughput.starved_windows = self.starved_windows;
        self.report.throughput.bench_bottleneck_suspected = self.ingest_windows >= 10 && self.starved_windows as f64 > 0.1 * self.ingest_windows as f64;
        self.report.throughput.debt_end = self.last_debt;
        self.report.queries_during_ingest = aggregate_queries(&self.shared.all_outcomes.lock().unwrap());
        if let Err(e) = self.report.write_atomic(&self.cli.out) {
            log(format!("report: write failed: {e}"));
        }
    }

    fn update_throughput(&mut self, acked: u64) {
        let first = self.acct.first_ack.lock().unwrap().unwrap_or(self.t0);
        let last = self.acct.last_ack.lock().unwrap().unwrap_or_else(Instant::now);
        let wall = last.duration_since(first).as_secs_f64();
        self.report.throughput.ingest_wall_s = wall;
        self.report.throughput.overall_pps = if wall > 0.0 { acked as f64 / wall } else { 0.0 };
        if let Some((t_fill, acked_at_start)) = self.fill_start {
            let fill_points = acked.saturating_sub(acked_at_start);
            let fill_wall = last.max(t_fill).duration_since(t_fill).as_secs_f64();
            self.report.throughput.fill_points = fill_points;
            self.report.throughput.fill_wall_s = fill_wall;
            self.report.throughput.fill_pps = if fill_wall > 0.0 { fill_points as f64 / fill_wall } else { 0.0 };
            let fill_windows: Vec<f64> = self.report.timeline.iter().filter(|w| w.phase == "ingest" && w.acked_points > 0).map(|w| w.acked_pps).collect();
            if !fill_windows.is_empty() {
                let q = (fill_windows.len() / 4).max(1);
                let tail = &fill_windows[fill_windows.len() - q..];
                self.report.throughput.late_pps = tail.iter().sum::<f64>() / tail.len() as f64;
            }
        }
    }

    fn finalize_throughput(&mut self) {
        let acked = self.acct.acked.load(Ordering::Relaxed);
        self.update_throughput(acked);
    }

    /// One reporting window; true when the ingest loop must end (drained, or
    /// the drain timed out after a stop).
    async fn tick(&mut self) -> bool {
        self.ctl.interval.tick().await;
        let elapsed = self.t0.elapsed();
        if self.shared.phase() == PH_WARMUP && elapsed >= self.cli.warmup {
            self.shared.set_phase(PH_INGEST);
        }
        self.record_window();
        let gens_done = self.gen_handles.iter().all(|h| h.is_finished());
        let drained = gens_done && self.rx.is_empty() && self.acct.inflight.load(Ordering::Relaxed) == 0;
        if drained {
            if matches!(self.report.verdict.stopped_reason.as_str(), "running" | "setup") {
                self.report.verdict.stopped_reason = "completed".into();
            }
            return true;
        }
        if let (Some(at), None) = (self.verdict.verdict.unusable_at_s, self.ctl.unusable_since) {
            self.ctl.unusable_since = Some(at);
            log(format!("verdict: unusable at t={at}s ({}), continuing for the grace period", self.verdict.verdict.unusable_reason.clone().unwrap_or_default()));
        }
        if !self.ctl.stopping {
            let reason = if self.shared.interrupted.load(Ordering::Relaxed) {
                Some("interrupted")
            } else if elapsed >= self.cli.max_ingest {
                self.report.throughput.hit_max_ingest = true;
                Some("wall_deadline")
            } else if self.ctl.unusable_since.map(|u| elapsed.as_secs().saturating_sub(u) >= self.cli.unusable_grace.as_secs()).unwrap_or(false) {
                Some("unusable")
            } else {
                None
            };
            if let Some(r) = reason {
                log(format!("ingest: stopping ({r})"));
                self.report.verdict.stopped_reason = r.into();
                self.gen.stop.store(true, Ordering::Relaxed);
                self.ctl.stopping = true;
                self.ctl.stop_started = Some(Instant::now());
            }
        } else if self.ctl.stop_started.map(|s| s.elapsed() > self.cli.drain_timeout).unwrap_or(false) {
            log("ingest: drain timeout, writers still busy; moving on");
            self.report.notes.push("drain timed out: writers were still busy when ingest was abandoned".into());
            return true;
        }
        false
    }

    async fn run_until_done(&mut self) {
        while !self.tick().await {}
    }

    async fn ramp_step(&mut self, rate: u64) -> (RampStep, bool) {
        self.gen.set_rate(rate, self.batch_points);
        let start_idx = self.report.timeline.len();
        let start_t_s = self.t0.elapsed().as_secs();
        let t_start = Instant::now();
        let ticks = (self.cli.step.as_secs_f64() / self.cli.window.as_secs_f64().max(1.0)).round().max(1.0) as usize;
        log(format!("ramp: step {}/s for {}s", fmt_count(rate as f64), self.cli.step.as_secs()));
        let mut done = false;
        for _ in 0..ticks {
            if self.tick().await {
                done = true;
                break;
            }
        }
        let step = self.evaluate_step(rate, start_idx, start_t_s, t_start.elapsed());
        log(format!(
            "ramp: {}/s achieved={}/s debt={}->{} lag={}s q95={}ms timeouts={} errors={} throttled={} passed={} {}",
            fmt_count(rate as f64),
            fmt_count(step.achieved_pps),
            step.debt_first_half.map(|d| format!("{d:.0}")).unwrap_or_else(|| "-".into()),
            step.debt_second_half.map(|d| format!("{d:.0}")).unwrap_or_else(|| "-".into()),
            step.lag_wall_s.map(|d| format!("{d:.0}")).unwrap_or_else(|| "-".into()),
            step.query_p95_ms.map(|d| format!("{d:.0}")).unwrap_or_else(|| "-".into()),
            step.query_timeouts,
            step.errors,
            step.throttled,
            step.passed,
            step.reason
        ));
        (step, done)
    }

    fn evaluate_step(&self, rate: u64, start_idx: usize, start_t_s: u64, took: Duration) -> RampStep {
        let all = &self.report.timeline[start_idx.min(self.report.timeline.len())..];
        // A window records the 10 s that ended at its t_s, so the window ending
        // exactly ramp_in after the step start is the first one fully past it.
        let ws: Vec<&Window> = all.iter().filter(|w| w.t_s >= start_t_s + self.cli.ramp_in.as_secs() + self.cli.window.as_secs()).collect();
        let mut step = RampStep { rate_pps: rate, step_s: took.as_secs(), windows: ws.len(), ..Default::default() };
        if ws.is_empty() {
            step.reason = "no windows after ramp-in; step too short to judge".into();
            return step;
        }
        step.achieved_pps = ws.iter().map(|w| w.acked_pps).sum::<f64>() / ws.len() as f64;
        let half = ws.len() / 2;
        step.debt_first_half = debt_floor(ws[..half].iter().filter_map(|w| w.debt));
        step.debt_second_half = debt_floor(ws[half..].iter().filter_map(|w| w.debt));
        step.lag_wall_s = ws.iter().rev().find_map(|w| w.visibility.as_ref().and_then(|v| v.lag_wall_s));
        let mut q_ms: Vec<f64> = ws.iter().flat_map(|w| w.queries.iter()).filter(|q| q.status != QueryStatus::Error).map(|q| q.ms).collect();
        q_ms.sort_by(|a, b| a.partial_cmp(b).unwrap());
        step.query_p95_ms = if q_ms.is_empty() { None } else { Some(q_ms[((q_ms.len() as f64 - 1.0) * 0.95).round() as usize]) };
        step.query_timeouts = ws.iter().flat_map(|w| w.queries.iter()).filter(|q| q.status == QueryStatus::Timeout).count() as u64;
        step.errors = ws.iter().map(|w| w.errors).sum();
        step.points_lost = ws.iter().map(|w| w.points_lost).sum();
        step.throttled = ws.iter().map(|w| w.throttled).sum();

        let want = rate as f64;
        let (passed, reason) = if ws.len() < 2 {
            (false, format!("only {} window after ramp-in; step too short to judge", ws.len()))
        } else if step.achieved_pps < 0.9 * want {
            (false, format!("accepted only {:.0}% of the offered rate", step.achieved_pps / want * 100.0))
        } else if step.points_lost > 0 || step.errors > 0 {
            (false, format!("{} write errors, {} points lost", step.errors, step.points_lost))
        } else if step.throttled > 0 {
            (false, format!("store throttled {} writes", step.throttled))
        } else if half >= 4 && matches!((step.debt_first_half, step.debt_second_half), (Some(a), Some(b)) if a > 0.0 && b > self.cli.debt_growth * a && b - a >= self.cli.debt_min_delta) {
            (false, format!("{} grew {:.0} -> {:.0} within the step", self.report.debt_metric, step.debt_first_half.unwrap(), step.debt_second_half.unwrap()))
        } else if step.lag_wall_s.map(|l| l > self.cli.lag_threshold.as_secs_f64()).unwrap_or(false) {
            (false, format!("visibility lag {:.0}s of ingest", step.lag_wall_s.unwrap()))
        } else if step.query_timeouts > 0 {
            (false, format!("{} query timeouts", step.query_timeouts))
        } else if step.query_p95_ms.map(|p| p > self.cli.query_slow_ms as f64).unwrap_or(false) {
            (false, format!("query p95 {:.0}ms over the threshold", step.query_p95_ms.unwrap()))
        } else {
            (true, String::new())
        };
        step.passed = passed;
        step.reason = reason;
        step
    }

    async fn run_ramp(&mut self) -> RampReport {
        let ladder = self.cli.ramp_ladder().unwrap_or_else(|_| vec![250_000]);
        let mut rep = RampReport { ladder: ladder.clone(), step_s: self.cli.step.as_secs(), ramp_in_s: self.cli.ramp_in.as_secs(), ..Default::default() };
        let mut last_pass: Option<u64> = None;
        let mut first_fail: Option<u64> = None;
        for &rate in &ladder {
            let (step, aborted) = self.ramp_step(rate).await;
            let passed = step.passed;
            rep.steps.push(step);
            if aborted {
                rep.aborted = true;
                break;
            }
            if passed {
                last_pass = Some(rate);
            } else {
                first_fail = Some(rate);
                break;
            }
        }
        if !rep.aborted {
            for _ in 0..self.cli.ramp_bisect {
                let (lp, ff) = match (last_pass, first_fail) {
                    (Some(lp), Some(ff)) if ff as f64 > lp as f64 * 1.15 => (lp, ff),
                    _ => break,
                };
                let mid = ((lp as f64) * (ff as f64)).sqrt() as u64;
                let (step, aborted) = self.ramp_step(mid).await;
                let passed = step.passed;
                rep.steps.push(step);
                if aborted {
                    rep.aborted = true;
                    break;
                }
                if passed {
                    last_pass = Some(mid);
                } else {
                    first_fail = Some(mid);
                }
            }
        }
        rep.sustainable_pps = last_pass.map(|r| r as f64);
        rep.fill_rate_pps = match (last_pass, first_fail) {
            (Some(lp), Some(_)) => (lp as f64 * self.cli.fill_fraction) as u64,
            (Some(_), None) => {
                rep.note = "every ramp step passed; the fill runs at saturation, so the sustainable rate is a lower bound".into();
                0
            }
            (None, _) => {
                rep.note = "no ramp step passed; the fill runs at half the lowest rate".into();
                (ladder[0] / 2).max(1)
            }
        };
        if !rep.note.is_empty() {
            log(format!("ramp: {}", rep.note));
            self.report.notes.push(rep.note.clone());
        }
        rep
    }

    async fn tick_until<F: Future>(&mut self, fut: F) -> F::Output {
        tokio::pin!(fut);
        let mut interval = tokio::time::interval(self.cli.window);
        interval.set_missed_tick_behavior(tokio::time::MissedTickBehavior::Delay);
        interval.tick().await;
        loop {
            tokio::select! {
                out = &mut fut => return out,
                _ = interval.tick() => self.record_window(),
            }
        }
    }
}

async fn probe_task(sink: Arc<dyn Sink>, cli: Cli, shared: Arc<Shared>, in_process: bool) {
    let mut sampler = ProcSampler::new(cli.cgroup.clone(), in_process);
    let mut last_disk: Option<Instant> = None;
    let mut vis_reader = None;
    let mut vis_err_logged = false;
    loop {
        let phase = shared.phase();
        if phase == PH_DONE {
            break;
        }
        if phase < PH_DRAIN {
            if vis_reader.is_none() {
                vis_reader = sink.reader().await.ok();
            }
            if let Some(r) = vis_reader.as_mut() {
                match tokio::time::timeout(Duration::from_secs(20), r.visible_max_ts()).await {
                    Ok(Ok(Some(t))) => shared.visible_ts.store(t, Ordering::Relaxed),
                    Ok(Ok(None)) => {}
                    Ok(Err(e)) => {
                        if !vis_err_logged {
                            log(format!("probe: visibility query failed: {e:#}"));
                            vis_err_logged = true;
                        }
                    }
                    Err(_) => {}
                }
            }
        } else {
            vis_reader = None;
        }
        let health = tokio::time::timeout(Duration::from_secs(8), sink.health()).await.ok().and_then(Result::ok);
        let reachable = tokio::time::timeout(Duration::from_secs(8), sink.is_reachable()).await.unwrap_or(false);
        let proc = sampler.sample();
        let container = match &cli.db_container {
            Some(n) => probe::container_state(n).await,
            None => None,
        };
        let disk_due = last_disk.map(|t| t.elapsed() >= cli.disk_interval).unwrap_or(true);
        let disk = if disk_due {
            last_disk = Some(Instant::now());
            let dir = cli.data_dir.clone();
            let s = Arc::clone(&sink);
            tokio::task::spawn_blocking(move || probe::disk::walk(&dir, &|rel| s.disk_class(rel))).await.ok()
        } else {
            None
        };
        {
            let mut p = shared.probe.lock().unwrap();
            p.health = health;
            p.proc = Some(proc);
            p.reachable = reachable;
            if let Some(c) = &container {
                if let Some(prev) = &p.last_started_at {
                    if prev != &c.started_at {
                        p.restarted = true;
                    }
                }
                p.last_started_at = Some(c.started_at.clone());
            }
            p.container = container;
            if let Some(d) = disk {
                p.disk = Some(d);
                p.disk_seq += 1;
            }
        }
        tokio::time::sleep(cli.window).await;
    }
}

async fn query_task(sink: Arc<dyn Sink>, cli: Cli, shared: Arc<Shared>, params: Arc<QueryParams>) {
    let mut reader = match sink.reader().await {
        Ok(r) => r,
        Err(e) => {
            log(format!("queries: reader unavailable: {e:#}"));
            return;
        }
    };
    let ids = QueryId::all();
    let mut i = 0usize;
    loop {
        if shared.phase() >= PH_DRAIN {
            break;
        }
        let t = shared.visible_ts.load(Ordering::Relaxed);
        if t <= 0 {
            tokio::time::sleep(Duration::from_secs(2)).await;
            continue;
        }
        let q = QueryInstance { id: ids[i % ids.len()], t_ms: t, params: Arc::clone(&params) };
        let o = reader.run_query(&q, cli.query_deadline).await;
        shared.pending_outcomes.lock().unwrap().push(o.clone());
        shared.all_outcomes.lock().unwrap().push(o);
        i += 1;
        let deadline = Instant::now() + cli.query_interval;
        while Instant::now() < deadline && shared.phase() < PH_DRAIN {
            tokio::time::sleep(Duration::from_millis(500)).await;
        }
    }
}

async fn cold_phase(sink: Arc<dyn Sink>, cli: &Cli, params: Arc<QueryParams>, sim_end_ms: i64, deadline: Instant) -> anyhow::Result<BTreeMap<String, ColdQuery>> {
    sink.make_cold().await?;
    if let Some(hook) = &cli.cold_hook {
        log(format!("cold: running hook {}", hook.display()));
        let status = tokio::time::timeout(Duration::from_secs(600), tokio::process::Command::new("bash").arg(hook).status()).await;
        match status {
            Ok(Ok(s)) if s.success() => {}
            Ok(Ok(s)) => log(format!("cold: hook exited {s}")),
            Ok(Err(e)) => log(format!("cold: hook failed to start: {e}")),
            Err(_) => log("cold: hook timed out"),
        }
    }
    unsafe { libc::sync() };
    match std::fs::write("/proc/sys/vm/drop_caches", "3") {
        Ok(()) => log("cold: page cache dropped"),
        Err(e) => log(format!("cold: could not drop page cache ({e}); cold numbers include cache hits")),
    }
    sink.reopen().await?;
    sink::http::wait_until(cli.setup_timeout, || {
        let s = Arc::clone(&sink);
        async move { s.is_reachable().await }
    })
    .await?;
    let mut reader = sink.reader().await?;
    let t = reader.visible_max_ts().await.ok().flatten().unwrap_or(sim_end_ms);
    let mut out = BTreeMap::new();
    for &id in QueryId::all() {
        if Instant::now() > deadline {
            log("cold: deadline reached, remaining queries skipped");
            break;
        }
        let q = QueryInstance { id, t_ms: t, params: Arc::clone(&params) };
        let mut runs: Vec<QueryOutcome> = Vec::new();
        for _ in 0..cli.cold_runs.max(1) {
            let o = reader.run_query(&q, cli.query_deadline).await;
            let stop = matches!(o.status, QueryStatus::Timeout | QueryStatus::Error);
            runs.push(o);
            if stop {
                break;
            }
        }
        let first = &runs[0];
        let mut warm: Vec<f64> = runs.iter().skip(1).map(|o| o.ms).collect();
        warm.sort_by(|a, b| a.partial_cmp(b).unwrap());
        let warm_median = if warm.is_empty() { first.ms } else { warm[warm.len() / 2] };
        log(format!("cold: {} first={:.0}ms warm={:.0}ms rows={} status={:?}", id.as_str(), first.ms, warm_median, first.rows, first.status));
        out.insert(
            id.as_str().to_string(),
            ColdQuery { intent: id.intent().into(), first_ms: first.ms, warm_median_ms: warm_median, rows: first.rows, status: Some(first.status), error: first.error.clone() },
        );
    }
    Ok(out)
}
