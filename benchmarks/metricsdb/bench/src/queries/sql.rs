use crate::util::fmt_ts_ms;

use super::{QueryId, QueryInstance};

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Dialect {
    ClickHouse,
    DuckDb,
    Firebolt,
}

impl Dialect {
    fn ts(self, ms: i64) -> String {
        let s = fmt_ts_ms(ms);
        match self {
            Dialect::ClickHouse => format!("toDateTime64('{s}', 3, 'UTC')"),
            Dialect::DuckDb => format!("TIMESTAMP '{s}'"),
            Dialect::Firebolt => format!("'{s}'::TIMESTAMPNTZ"),
        }
    }

    fn bucket(self, secs: i64) -> String {
        match self {
            Dialect::ClickHouse => format!("toStartOfInterval(p.ts, INTERVAL {secs} SECOND)"),
            Dialect::DuckDb => format!("time_bucket(INTERVAL '{secs} seconds', p.ts)"),
            Dialect::Firebolt => {
                if secs == 60 {
                    "DATE_TRUNC('minute', p.ts)".to_string()
                } else {
                    let m = secs / 60;
                    format!("DATE_TRUNC('minute', p.ts) - (EXTRACT(MINUTE FROM p.ts)::INT % {m}) * INTERVAL '1 minute'")
                }
            }
        }
    }

    pub fn tag(self, key: &str) -> String {
        match self {
            Dialect::ClickHouse => format!("tags['{key}']"),
            Dialect::DuckDb => format!("tags['{key}']"),
            Dialect::Firebolt => format!("JSON_POINTER_EXTRACT_TEXT(tags, '/{key}')"),
        }
    }

    fn arg_max(self) -> &'static str {
        match self {
            Dialect::ClickHouse => "argMax",
            Dialect::DuckDb => "arg_max",
            Dialect::Firebolt => "MAX_BY",
        }
    }

    fn p95(self, col: &str) -> String {
        match self {
            Dialect::ClickHouse => format!("quantile(0.95)({col})"),
            Dialect::DuckDb => format!("quantile_cont({col}, 0.95)"),
            Dialect::Firebolt => format!("PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY {col})"),
        }
    }

    fn count_distinct(self, col: &str) -> String {
        match self {
            Dialect::ClickHouse => format!("uniqExact({col})"),
            _ => format!("count(DISTINCT {col})"),
        }
    }
}

pub fn render(d: Dialect, q: &QueryInstance) -> String {
    let p = &q.params;
    let from = d.ts(q.from_ms());
    let to = d.ts(q.t_ms);
    let (_, bucket) = q.id.window();
    match q.id {
        QueryId::A => format!(
            "SELECT {b} AS b, s.host AS host, avg(p.value) AS v \
             FROM points p \
             INNER JOIN (SELECT series_id, {host} AS host FROM series WHERE name = '{name}' AND {state} = 'user') s ON p.series_id = s.series_id \
             WHERE p.series_id >= {lo} AND p.series_id < {hi} AND p.ts >= {from} AND p.ts < {to} \
             GROUP BY b, host ORDER BY b, host",
            b = d.bucket(bucket),
            host = d.tag("host.name"),
            state = d.tag("state"),
            name = p.cpu.name,
            lo = p.cpu.lo,
            hi = p.cpu.hi,
        ),
        QueryId::B => format!(
            "SELECT {b} AS b, avg(p.value) AS v FROM points p \
             WHERE p.series_id = {sid} AND p.ts >= {from} AND p.ts < {to} GROUP BY b ORDER BY b",
            b = d.bucket(bucket),
            sid = p.one_series,
        ),
        QueryId::C => format!(
            "SELECT s.host AS host, {am}(p.value, p.ts) AS v \
             FROM points p \
             INNER JOIN (SELECT series_id, {host} AS host FROM series WHERE name = '{name}' AND {state} = 'used') s ON p.series_id = s.series_id \
             WHERE p.series_id >= {lo} AND p.series_id < {hi} AND p.ts >= {from} AND p.ts < {to} \
             GROUP BY host ORDER BY v DESC LIMIT 20",
            am = d.arg_max(),
            host = d.tag("host.name"),
            state = d.tag("state"),
            name = p.mem.name,
            lo = p.mem.lo,
            hi = p.mem.hi,
        ),
        QueryId::D => format!(
            "SELECT {b} AS b, s.cluster AS cluster, avg(p.value) AS v \
             FROM points p \
             INNER JOIN (SELECT series_id, {cluster} AS cluster FROM series WHERE name = '{name}') s ON p.series_id = s.series_id \
             WHERE p.series_id >= {lo} AND p.series_id < {hi} AND p.ts >= {from} AND p.ts < {to} \
             GROUP BY b, cluster ORDER BY b, cluster",
            b = d.bucket(bucket),
            cluster = d.tag("k8s.cluster.name"),
            name = p.pod.name,
            lo = p.pod.lo,
            hi = p.pod.hi,
        ),
        QueryId::E1 => format!(
            "SELECT s.name AS name, {cd} AS n \
             FROM points p INNER JOIN series s ON p.series_id = s.series_id \
             WHERE p.ts >= {from} AND p.ts < {to} GROUP BY name ORDER BY name",
            cd = d.count_distinct("p.series_id"),
        ),
        QueryId::E2 => format!(
            "SELECT DISTINCT {host} AS host FROM series \
             WHERE series_id IN (SELECT DISTINCT p.series_id FROM points p WHERE p.series_id >= {lo} AND p.series_id < {hi} AND p.ts >= {from} AND p.ts < {to}) \
             ORDER BY host",
            host = d.tag("host.name"),
            lo = p.cpu.lo,
            hi = p.cpu.hi,
        ),
        QueryId::F => format!(
            "SELECT count(*) AS n, {p95} AS p95 FROM \
             (SELECT p.series_id AS series_id, {am}(p.value, p.ts) AS v FROM points p \
              WHERE p.series_id >= {lo} AND p.series_id < {hi} AND p.ts >= {from} AND p.ts < {to} GROUP BY series_id) t",
            p95 = d.p95("v"),
            am = d.arg_max(),
            lo = p.http.lo,
            hi = p.http.hi,
        ),
    }
}
