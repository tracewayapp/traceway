use std::collections::VecDeque;

use serde::Serialize;

use crate::queries::QueryStatus;
use crate::report::Window;

#[derive(Clone, Debug, Default, Serialize)]
pub struct Verdict {
    pub fell_behind_at_s: Option<u64>,
    pub fell_behind_reason: Option<String>,
    pub unusable_at_s: Option<u64>,
    pub unusable_reason: Option<String>,
    pub process_restarts: u32,
    pub stopped_reason: String,
}

pub struct VerdictCfg {
    pub lag_threshold_s: f64,
    pub degrade_ratio: f64,
    pub query_slow_ms: f64,
    pub write_err_ratio: f64,
    pub interval_s: f64,
    pub debt_windows: usize,
    pub debt_growth: f64,
    pub debt_min_delta: f64,
    pub debt_metric: String,
}

pub struct VerdictState {
    cfg: VerdictCfg,
    behind_streak: u32,
    unusable_streak: u32,
    behind_reasons: Vec<String>,
    unusable_reasons: Vec<String>,
    recent_pps: VecDeque<f64>,
    post_warmup_windows: u32,
    lags: VecDeque<f64>,
    debts: VecDeque<Option<f64>>,
    debt_streak: u32,
    pub plateau_pps: f64,
    pub verdict: Verdict,
}

impl VerdictState {
    pub fn new(cfg: VerdictCfg) -> Self {
        Self {
            cfg,
            behind_streak: 0,
            unusable_streak: 0,
            behind_reasons: Vec::new(),
            unusable_reasons: Vec::new(),
            recent_pps: VecDeque::new(),
            post_warmup_windows: 0,
            lags: VecDeque::new(),
            debts: VecDeque::new(),
            debt_streak: 0,
            plateau_pps: 0.0,
            verdict: Verdict { stopped_reason: "running".into(), ..Default::default() },
        }
    }

    pub fn observe(&mut self, w: &Window, reachable: bool, restarted: bool) {
        if restarted {
            self.verdict.process_restarts += 1;
        }
        if w.phase != "ingest" {
            return;
        }
        self.post_warmup_windows += 1;
        self.recent_pps.push_back(w.acked_pps);
        if self.recent_pps.len() > 5 {
            self.recent_pps.pop_front();
        }
        if self.recent_pps.len() == 5 {
            let mut v: Vec<f64> = self.recent_pps.iter().copied().collect();
            v.sort_by(|a, b| a.partial_cmp(b).unwrap());
            self.plateau_pps = self.plateau_pps.max(v[2]);
        }
        let lag = w.visibility.as_ref().map(|v| v.lag_sim_s).unwrap_or(0.0);
        self.lags.push_back(lag);
        if self.lags.len() > 3 {
            self.lags.pop_front();
        }

        let mut behind: Option<String> = None;
        let threshold = self.cfg.lag_threshold_s.max(6.0 * self.cfg.interval_s);
        if self.lags.len() == 3 && lag > threshold && self.lags[0] <= self.lags[1] && self.lags[1] <= self.lags[2] {
            behind = Some(format!("visibility lag {lag:.0}s > {threshold:.0}s and growing"));
        }
        if self.post_warmup_windows >= 10 && self.plateau_pps > 0.0 && w.acked_pps < self.cfg.degrade_ratio * self.plateau_pps {
            behind.get_or_insert(format!("throughput {:.0}/s < {:.0}% of plateau {:.0}/s", w.acked_pps, self.cfg.degrade_ratio * 100.0, self.plateau_pps));
        }
        if w.queries.iter().any(|q| q.status == QueryStatus::Timeout || (q.status != QueryStatus::Error && q.ms > self.cfg.query_slow_ms)) {
            behind.get_or_insert("during-ingest query slower than threshold or timed out".into());
        }
        let total = w.batches + w.errors;
        let err_ratio = if total > 0 { w.errors as f64 / total as f64 } else { 0.0 };
        if err_ratio > self.cfg.write_err_ratio || (w.batches > 0 && w.retries > w.batches) {
            behind.get_or_insert(format!("write error ratio {:.1}% (retries {})", err_ratio * 100.0, w.retries));
        }
        // Deferred work that keeps piling up is the LSM way of falling behind:
        // every insert still returns 200 while merges, checkpoints or vacuums
        // slip further behind. Compare the two halves of the trailing window
        // and require the growth to hold for a full minute.
        self.debts.push_back(w.debt);
        if self.debts.len() > self.cfg.debt_windows.max(2) {
            self.debts.pop_front();
        }
        if self.debts.len() == self.cfg.debt_windows.max(2) {
            let half = self.debts.len() / 2;
            let floor = |it: &mut dyn Iterator<Item = &Option<f64>>| {
                let mut v: Vec<f64> = it.filter_map(|d| *d).collect();
                if v.is_empty() {
                    return None;
                }
                v.sort_by(|a, b| a.partial_cmp(b).unwrap());
                Some(v[(v.len() - 1) / 4])
            };
            let m1 = floor(&mut self.debts.iter().take(half));
            let m2 = floor(&mut self.debts.iter().skip(half));
            match (m1, m2) {
                (Some(a), Some(b)) if a > 0.0 && b > self.cfg.debt_growth * a && b - a >= self.cfg.debt_min_delta => {
                    self.debt_streak += 1;
                    if self.debt_streak >= 6 {
                        behind.get_or_insert(format!("merge debt growing: {} floor {a:.0} -> {b:.0} over the trailing window", self.cfg.debt_metric));
                    }
                }
                _ => self.debt_streak = 0,
            }
        }
        if w.throttled > 0 {
            behind.get_or_insert(format!("store throttled {} writes (too many parts)", w.throttled));
        }

        let mut unusable: Option<String> = None;
        if !w.queries.is_empty() && w.queries.iter().all(|q| matches!(q.status, QueryStatus::Timeout | QueryStatus::Error)) {
            unusable = Some("every query in the window timed out or failed".into());
        }
        if w.acked_points == 0 && w.produced_pps > 0.0 {
            unusable.get_or_insert("writers acknowledged nothing while the generator produced".into());
        }
        if err_ratio > 0.5 {
            unusable.get_or_insert(format!("write error ratio {:.0}%", err_ratio * 100.0));
        }
        if !reachable {
            unusable.get_or_insert("database unreachable".into());
        }
        if restarted {
            unusable.get_or_insert("database process restarted".into());
        }

        match behind {
            Some(r) => {
                self.behind_streak += 1;
                self.behind_reasons.push(r);
            }
            None => {
                self.behind_streak = 0;
                self.behind_reasons.clear();
            }
        }
        match unusable {
            Some(r) => {
                self.unusable_streak += 1;
                self.unusable_reasons.push(r);
            }
            None => {
                self.unusable_streak = 0;
                self.unusable_reasons.clear();
            }
        }
        if self.behind_streak >= 3 && self.verdict.fell_behind_at_s.is_none() {
            self.verdict.fell_behind_at_s = Some(w.t_s);
            self.verdict.fell_behind_reason = self.behind_reasons.last().cloned();
        }
        if self.unusable_streak >= 3 && self.verdict.unusable_at_s.is_none() {
            self.verdict.unusable_at_s = Some(w.t_s);
            self.verdict.unusable_reason = self.unusable_reasons.last().cloned();
        }
    }
}
