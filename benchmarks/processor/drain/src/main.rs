use axum::body::Bytes;
use axum::extract::State;
use axum::http::HeaderMap;
use axum::routing::{get, post};
use axum::Router;
use flate2::read::GzDecoder;
use std::io::Read;
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::Arc;

const OK_MARKER: &[u8] = b"../src/inventory.js";
const FAIL_MARKER: &[u8] = b".mjs:1:";

#[derive(Default)]
struct Counters {
    requests: AtomicU64,
    symbolicated: AtomicU64,
    unsymbolicated: AtomicU64,
    bytes: AtomicU64,
}

fn contains(haystack: &[u8], needle: &[u8]) -> bool {
    haystack.windows(needle.len()).any(|w| w == needle)
}

async fn ingest(State(c): State<Arc<Counters>>, headers: HeaderMap, body: Bytes) -> &'static str {
    let decoded: Vec<u8>;
    let data: &[u8] = if headers
        .get("content-encoding")
        .map(|v| v.as_bytes() == b"gzip")
        .unwrap_or(false)
    {
        let mut d = GzDecoder::new(body.as_ref());
        let mut out = Vec::with_capacity(body.len() * 4);
        if d.read_to_end(&mut out).is_err() {
            c.unsymbolicated.fetch_add(1, Ordering::Relaxed);
            return "";
        }
        decoded = out;
        &decoded
    } else {
        body.as_ref()
    };
    c.requests.fetch_add(1, Ordering::Relaxed);
    c.bytes.fetch_add(data.len() as u64, Ordering::Relaxed);
    if contains(data, OK_MARKER) && !contains(data, FAIL_MARKER) {
        c.symbolicated.fetch_add(1, Ordering::Relaxed);
    } else {
        c.unsymbolicated.fetch_add(1, Ordering::Relaxed);
    }
    ""
}

async fn stats(State(c): State<Arc<Counters>>) -> String {
    serde_json::json!({
        "requests": c.requests.load(Ordering::Relaxed),
        "symbolicated": c.symbolicated.load(Ordering::Relaxed),
        "unsymbolicated": c.unsymbolicated.load(Ordering::Relaxed),
        "bytes": c.bytes.load(Ordering::Relaxed),
    })
    .to_string()
}

async fn reset(State(c): State<Arc<Counters>>) -> &'static str {
    c.requests.store(0, Ordering::Relaxed);
    c.symbolicated.store(0, Ordering::Relaxed);
    c.unsymbolicated.store(0, Ordering::Relaxed);
    c.bytes.store(0, Ordering::Relaxed);
    "reset"
}

#[tokio::main]
async fn main() {
    let counters = Arc::new(Counters::default());
    let app = Router::new()
        .route("/v1/traces", post(ingest))
        .route("/v1/logs", post(ingest))
        .route("/stats", get(stats))
        .route("/reset", post(reset))
        .with_state(counters);
    let addr = std::env::var("DRAIN_ADDR").unwrap_or_else(|_| "0.0.0.0:9319".to_string());
    let listener = tokio::net::TcpListener::bind(&addr).await.unwrap();
    println!("drain listening on {}", addr);
    axum::serve(listener, app).await.unwrap();
}
