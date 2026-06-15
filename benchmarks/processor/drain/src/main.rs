use axum::body::Bytes;
use axum::extract::State;
use axum::http::HeaderMap;
use axum::routing::{get, post};
use axum::Router;
use flate2::read::GzDecoder;
use std::io::Read;
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::Arc;

const DEFAULT_OK_MARKER: &str = "../src/inventory.js";
const DEFAULT_FAIL_MARKER: &str = ".mjs:1:";

#[derive(Default)]
struct Counters {
    requests: AtomicU64,
    symbolicated: AtomicU64,
    unsymbolicated: AtomicU64,
    bytes: AtomicU64,
}

struct App {
    counters: Counters,
    ok: Vec<u8>,
    fail: Vec<u8>,
}

fn contains(haystack: &[u8], needle: &[u8]) -> bool {
    haystack.windows(needle.len()).any(|w| w == needle)
}

async fn ingest(State(c): State<Arc<App>>, headers: HeaderMap, body: Bytes) -> &'static str {
    let decoded: Vec<u8>;
    let data: &[u8] = if headers
        .get("content-encoding")
        .map(|v| v.as_bytes() == b"gzip")
        .unwrap_or(false)
    {
        let mut d = GzDecoder::new(body.as_ref());
        let mut out = Vec::with_capacity(body.len() * 4);
        if d.read_to_end(&mut out).is_err() {
            c.counters.unsymbolicated.fetch_add(1, Ordering::Relaxed);
            return "";
        }
        decoded = out;
        &decoded
    } else {
        body.as_ref()
    };
    c.counters.requests.fetch_add(1, Ordering::Relaxed);
    c.counters.bytes.fetch_add(data.len() as u64, Ordering::Relaxed);
    if contains(data, &c.ok) && !contains(data, &c.fail) {
        c.counters.symbolicated.fetch_add(1, Ordering::Relaxed);
    } else {
        c.counters.unsymbolicated.fetch_add(1, Ordering::Relaxed);
    }
    ""
}

async fn stats(State(c): State<Arc<App>>) -> String {
    serde_json::json!({
        "requests": c.counters.requests.load(Ordering::Relaxed),
        "symbolicated": c.counters.symbolicated.load(Ordering::Relaxed),
        "unsymbolicated": c.counters.unsymbolicated.load(Ordering::Relaxed),
        "bytes": c.counters.bytes.load(Ordering::Relaxed),
    })
    .to_string()
}

async fn reset(State(c): State<Arc<App>>) -> &'static str {
    c.counters.requests.store(0, Ordering::Relaxed);
    c.counters.symbolicated.store(0, Ordering::Relaxed);
    c.counters.unsymbolicated.store(0, Ordering::Relaxed);
    c.counters.bytes.store(0, Ordering::Relaxed);
    "reset"
}

#[tokio::main]
async fn main() {
    let ok = std::env::var("OK_MARKER").unwrap_or_else(|_| DEFAULT_OK_MARKER.to_string());
    let fail = std::env::var("FAIL_MARKER").unwrap_or_else(|_| DEFAULT_FAIL_MARKER.to_string());
    println!("drain markers: ok={:?} fail={:?}", ok, fail);
    let state = Arc::new(App {
        counters: Counters::default(),
        ok: ok.into_bytes(),
        fail: fail.into_bytes(),
    });
    let app = Router::new()
        .route("/v1/traces", post(ingest))
        .route("/v1/logs", post(ingest))
        .route("/stats", get(stats))
        .route("/reset", post(reset))
        .with_state(state);
    let addr = std::env::var("DRAIN_ADDR").unwrap_or_else(|_| "0.0.0.0:9319".to_string());
    let listener = tokio::net::TcpListener::bind(&addr).await.unwrap();
    println!("drain listening on {}", addr);
    axum::serve(listener, app).await.unwrap();
}
