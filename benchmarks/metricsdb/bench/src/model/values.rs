use super::rng::Xoshiro256;
use super::templates::{Precision, ValueModel};

#[derive(Clone, Debug)]
pub struct ValueState {
    cur: f64,
    rng: Xoshiro256,
}

impl ValueState {
    pub fn new(model: &ValueModel, seed: u64) -> Self {
        let mut rng = Xoshiro256::seeded(seed);
        let cur = match *model {
            ValueModel::Walk { lo, hi, .. } => lo + rng.next_f64() * (hi - lo),
            ValueModel::Counter { rate_mean, .. } => rng.next_f64() * rate_mean * 86_400.0,
            ValueModel::Const { lo, hi } => lo + rng.next_f64() * (hi - lo),
            ValueModel::ZeroMostly { .. } => (rng.next_f64() * 20.0).floor(),
            ValueModel::Spiky { base, .. } => base,
        };
        Self { cur, rng }
    }

    #[inline]
    pub fn next(&mut self, model: &ValueModel, interval_s: f64, quiet: bool, prec: Precision) -> f64 {
        let damp = if quiet { 0.1 } else { 1.0 };
        match *model {
            ValueModel::Walk { lo, hi, sigma, revert } => {
                let mid = (lo + hi) * 0.5;
                self.cur += sigma * damp * self.rng.next_normal() + revert * (mid - self.cur);
                self.cur = self.cur.clamp(lo, hi);
            }
            ValueModel::Counter { rate_mean, rate_cv } => {
                let inc = rate_mean * interval_s * (1.0 + rate_cv * damp * self.rng.next_normal());
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
