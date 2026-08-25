pub fn splitmix64(state: &mut u64) -> u64 {
    *state = state.wrapping_add(0x9E37_79B9_7F4A_7C15);
    let mut z = *state;
    z = (z ^ (z >> 30)).wrapping_mul(0xBF58_476D_1CE4_E5B9);
    z = (z ^ (z >> 27)).wrapping_mul(0x94D0_49BB_1331_11EB);
    z ^ (z >> 31)
}

#[derive(Clone, Debug)]
pub struct Xoshiro256 {
    s: [u64; 4],
}

impl Xoshiro256 {
    pub fn seeded(seed: u64) -> Self {
        let mut st = seed;
        Self {
            s: [
                splitmix64(&mut st),
                splitmix64(&mut st),
                splitmix64(&mut st),
                splitmix64(&mut st),
            ],
        }
    }

    #[inline]
    pub fn next_u64(&mut self) -> u64 {
        let result = self.s[0]
            .wrapping_add(self.s[3])
            .rotate_left(23)
            .wrapping_add(self.s[0]);
        let t = self.s[1] << 17;
        self.s[2] ^= self.s[0];
        self.s[3] ^= self.s[1];
        self.s[1] ^= self.s[2];
        self.s[0] ^= self.s[3];
        self.s[2] ^= t;
        self.s[3] = self.s[3].rotate_left(45);
        result
    }

    #[inline]
    pub fn next_f64(&mut self) -> f64 {
        (self.next_u64() >> 11) as f64 * (1.0 / (1u64 << 53) as f64)
    }

    /// Approximately normal (Irwin-Hall with 6 uniforms), mean 0, sd 1.
    #[inline]
    pub fn next_normal(&mut self) -> f64 {
        let mut acc = 0.0;
        for _ in 0..6 {
            acc += self.next_f64();
        }
        (acc - 3.0) * 1.414_213_6
    }
}
