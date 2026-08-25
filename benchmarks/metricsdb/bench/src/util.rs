use std::time::{SystemTime, UNIX_EPOCH};

pub fn civil_from_days(z: i64) -> (i64, u32, u32) {
    let z = z + 719_468;
    let era = if z >= 0 { z } else { z - 146_096 } / 146_097;
    let doe = z - era * 146_097;
    let yoe = (doe - doe / 1460 + doe / 36_524 - doe / 146_096) / 365;
    let y = yoe + era * 400;
    let doy = doe - (365 * yoe + yoe / 4 - yoe / 100);
    let mp = (5 * doy + 2) / 153;
    let d = (doy - (153 * mp + 2) / 5 + 1) as u32;
    let m = if mp < 10 { mp + 3 } else { mp - 9 } as u32;
    (if m <= 2 { y + 1 } else { y }, m, d)
}

pub fn days_from_civil(y: i64, m: u32, d: u32) -> i64 {
    let y = if m <= 2 { y - 1 } else { y };
    let era = if y >= 0 { y } else { y - 399 } / 400;
    let yoe = y - era * 400;
    let mp = if m > 2 { m - 3 } else { m + 9 } as i64;
    let doy = (153 * mp + 2) / 5 + d as i64 - 1;
    let doe = yoe * 365 + yoe / 4 - yoe / 100 + doy;
    era * 146_097 + doe - 719_468
}

/// "YYYY-MM-DD HH:MM:SS.mmm" in UTC, the literal form every SQL dialect here accepts.
pub fn fmt_ts_ms(ms: i64) -> String {
    let days = ms.div_euclid(86_400_000);
    let rem = ms.rem_euclid(86_400_000);
    let (y, mo, d) = civil_from_days(days);
    let h = rem / 3_600_000;
    let mi = (rem / 60_000) % 60;
    let s = (rem / 1000) % 60;
    let f = rem % 1000;
    format!("{y:04}-{mo:02}-{d:02} {h:02}:{mi:02}:{s:02}.{f:03}")
}

pub fn fmt_rfc3339_ms(ms: i64) -> String {
    let s = fmt_ts_ms(ms);
    format!("{}T{}Z", &s[..10], &s[11..])
}

/// Accepts "YYYY-MM-DD HH:MM:SS[.frac]" and "YYYY-MM-DDTHH:MM:SS[.frac]Z".
pub fn parse_ts_ms(s: &str) -> Option<i64> {
    let s = s.trim().trim_end_matches('Z');
    if s.len() < 19 {
        return None;
    }
    let y: i64 = s[0..4].parse().ok()?;
    let mo: u32 = s[5..7].parse().ok()?;
    let d: u32 = s[8..10].parse().ok()?;
    let h: i64 = s[11..13].parse().ok()?;
    let mi: i64 = s[14..16].parse().ok()?;
    let sec: i64 = s[17..19].parse().ok()?;
    let mut frac_ms = 0i64;
    if s.len() > 20 && &s[19..20] == "." {
        let frac = &s[20..];
        let digits: String = frac.chars().take_while(|c| c.is_ascii_digit()).take(3).collect();
        let mut v: i64 = digits.parse().ok()?;
        for _ in digits.len()..3 {
            v *= 10;
        }
        frac_ms = v;
    }
    Some(days_from_civil(y, mo, d) * 86_400_000 + h * 3_600_000 + mi * 60_000 + sec * 1000 + frac_ms)
}

pub fn now_ms() -> i64 {
    SystemTime::now().duration_since(UNIX_EPOCH).map(|d| d.as_millis() as i64).unwrap_or(0)
}

pub fn now_rfc3339() -> String {
    fmt_rfc3339_ms(now_ms())
}

pub fn fmt_count(v: f64) -> String {
    let a = v.abs();
    if a >= 1e9 {
        format!("{:.2}G", v / 1e9)
    } else if a >= 1e6 {
        format!("{:.2}M", v / 1e6)
    } else if a >= 1e3 {
        format!("{:.1}k", v / 1e3)
    } else {
        format!("{v:.0}")
    }
}

pub fn fmt_bytes(v: u64) -> String {
    let f = v as f64;
    if f >= 1e12 {
        format!("{:.2}T", f / 1e12)
    } else if f >= 1e9 {
        format!("{:.2}G", f / 1e9)
    } else if f >= 1e6 {
        format!("{:.1}M", f / 1e6)
    } else if f >= 1e3 {
        format!("{:.0}k", f / 1e3)
    } else {
        format!("{v}")
    }
}

#[allow(dead_code)]
pub fn parse_bytes(s: &str) -> Option<u64> {
    let s = s.trim().to_ascii_uppercase();
    let (num, mult) = if let Some(n) = s.strip_suffix("GB").or_else(|| s.strip_suffix("GIB")).or_else(|| s.strip_suffix('G')) {
        (n, 1u64 << 30)
    } else if let Some(n) = s.strip_suffix("MB").or_else(|| s.strip_suffix("MIB")).or_else(|| s.strip_suffix('M')) {
        (n, 1u64 << 20)
    } else if let Some(n) = s.strip_suffix("KB").or_else(|| s.strip_suffix("KIB")).or_else(|| s.strip_suffix('K')) {
        (n, 1u64 << 10)
    } else if let Some(n) = s.strip_suffix('B') {
        (n, 1)
    } else {
        (s.as_str(), 1)
    };
    let f: f64 = num.trim().parse().ok()?;
    Some((f * mult as f64) as u64)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn ts_roundtrip() {
        for ms in [0i64, 1_767_225_600_000, 1_767_225_600_123, 4_102_444_800_999] {
            assert_eq!(parse_ts_ms(&fmt_ts_ms(ms)), Some(ms));
            assert_eq!(parse_ts_ms(&fmt_rfc3339_ms(ms)), Some(ms));
        }
        assert_eq!(fmt_ts_ms(1_767_225_600_000), "2026-01-01 00:00:00.000");
        assert_eq!(parse_ts_ms("2026-01-01 00:00:00"), Some(1_767_225_600_000));
        assert_eq!(parse_ts_ms("2026-01-01 00:00:00.5"), Some(1_767_225_600_500));
    }

    #[test]
    fn bytes_parse() {
        assert_eq!(parse_bytes("12GB"), Some(12 << 30));
        assert_eq!(parse_bytes("1.5 GB"), Some((1.5 * (1u64 << 30) as f64) as u64));
        assert_eq!(parse_bytes("256MB"), Some(256 << 20));
        assert_eq!(parse_bytes("1024"), Some(1024));
    }
}
