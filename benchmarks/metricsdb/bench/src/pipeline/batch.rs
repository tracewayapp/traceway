#[derive(Clone, Debug, Default)]
pub struct Batch {
    pub seq: u64,
    pub series_id: Vec<u64>,
    pub ts_ms: Vec<i64>,
    pub value: Vec<f64>,
    pub max_ts_ms: i64,
}

impl Batch {
    pub fn with_capacity(seq: u64, cap: usize) -> Self {
        Self { seq, series_id: Vec::with_capacity(cap), ts_ms: Vec::with_capacity(cap), value: Vec::with_capacity(cap), max_ts_ms: i64::MIN }
    }

    #[inline]
    pub fn push(&mut self, id: u64, ts: i64, v: f64) {
        self.series_id.push(id);
        self.ts_ms.push(ts);
        self.value.push(v);
        if ts > self.max_ts_ms {
            self.max_ts_ms = ts;
        }
    }

    #[inline]
    pub fn len(&self) -> usize {
        self.series_id.len()
    }

    pub fn is_empty(&self) -> bool {
        self.series_id.is_empty()
    }

    pub fn raw_bytes(&self) -> u64 {
        self.len() as u64 * 16
    }

    pub fn sorted_by_series_ts(&self) -> Batch {
        let mut idx: Vec<u32> = (0..self.len() as u32).collect();
        idx.sort_unstable_by_key(|&i| (self.series_id[i as usize], self.ts_ms[i as usize]));
        let mut out = Batch::with_capacity(self.seq, self.len());
        for i in idx {
            let i = i as usize;
            out.push(self.series_id[i], self.ts_ms[i], self.value[i]);
        }
        out
    }
}
