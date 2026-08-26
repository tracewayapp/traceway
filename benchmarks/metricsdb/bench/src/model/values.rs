use super::rng::Xoshiro256;
use super::templates::{Precision, ValueModel};

/// Per-series state. `rate` is this series' own steady rate for counters
/// (drawn once, log-normal around the template mean, the way idle and user
/// cpu time differ by an order of magnitude on one host); `hold_p` makes a
/// third of the gauges sticky, holding their last value most scrapes the way
/// memory, filesystem and connection-count gauges do in real fleets.
#[derive(Clone, Debug)]
pub struct ValueState {
    cur: f64,
    rate: f64,
    hold_p: f64,
    rng: Xoshiro256,
}

impl ValueState {
    pub fn new(model: &ValueModel, seed: u64) -> Self {
        let mut rng = Xoshiro256::seeded(seed);
        let mut rate = 0.0;
        let mut hold_p = 0.0;
        let cur = match *model {
            ValueModel::Walk { lo, hi, .. } => {
                if rng.next_f64() < 0.3 {
                    hold_p = 0.8;
                }
                lo + rng.next_f64() * (hi - lo)
            }
            ValueModel::Counter { rate_mean, .. } => {
                rate = rate_mean * (0.8 * rng.next_normal()).exp();
                rng.next_f64() * rate * 86_400.0
            }
            ValueModel::Const { lo, hi } => lo + rng.next_f64() * (hi - lo),
            ValueModel::ZeroMostly { .. } => (rng.next_f64() * 20.0).floor(),
            ValueModel::Spiky { base, .. } => base,
        };
        Self { cur, rate, hold_p, rng }
    }

    #[inline]
    pub fn next(&mut self, model: &ValueModel, interval_s: f64, quiet: bool, prec: Precision) -> f64 {
        let damp = if quiet { 0.1 } else { 1.0 };
        match *model {
            ValueModel::Walk { lo, hi, sigma, revert } => {
                let step = self.rng.next_normal();
                if self.hold_p > 0.0 && self.rng.next_f64() < self.hold_p {
                    return quantize(self.cur, prec);
                }
                let mid = (lo + hi) * 0.5;
                self.cur += sigma * damp * step + revert * (mid - self.cur);
                self.cur = self.cur.clamp(lo, hi);
            }
            ValueModel::Counter { rate_cv, .. } => {
                let inc = self.rate * interval_s * (1.0 + rate_cv * damp * self.rng.next_normal());
                self.cur += inc.max(0.0);
            }
            ValueModel::Const { .. } => {}
            ValueModel::ZeroMostly { p, burst } => {
                if self.rng.next_f64() < p {
                    self.cur += (burst * (1.0 + self.rng.next_f64())).round();
                }
            }
            ValueModel::Spiky { base, sigma, spike_p, spike_mag } => {
                let mut v = base + sigma * damp * self.rng.next_normal();
                if self.rng.next_f64() < spike_p {
                    v += spike_mag * (1.0 + self.rng.next_f64());
                }
                self.cur = v.max(0.0);
            }
        }
        quantize(self.cur, prec)
    }
}

#[inline]
pub fn quantize(v: f64, prec: Precision) -> f64 {
    match prec {
        Precision::Int => v.round(),
        Precision::Dec2 => (v * 100.0).round() / 100.0,
        Precision::Dec4 => (v * 10_000.0).round() / 10_000.0,
    }
}
