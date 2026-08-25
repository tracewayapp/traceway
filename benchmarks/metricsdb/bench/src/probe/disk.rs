use std::collections::BTreeMap;
use std::path::Path;
use std::time::Instant;

use serde::Serialize;

#[derive(Clone, Debug, Default, Serialize)]
pub struct DiskSample {
    pub total_bytes: u64,
    pub by_class: BTreeMap<String, u64>,
    pub walk_ms: u64,
}

pub fn walk(dir: &Path, classify: &dyn Fn(&str) -> &'static str) -> DiskSample {
    let t0 = Instant::now();
    let mut out = DiskSample::default();
    for entry in walkdir::WalkDir::new(dir).follow_links(false).into_iter().filter_map(Result::ok) {
        let md = match entry.metadata() {
            Ok(m) => m,
            Err(_) => continue,
        };
        if !md.is_file() {
            continue;
        }
        let bytes = allocated(&md);
        let rel = entry.path().strip_prefix(dir).map(|p| p.to_string_lossy().to_string()).unwrap_or_default();
        *out.by_class.entry(classify(&rel).to_string()).or_insert(0) += bytes;
        out.total_bytes += bytes;
    }
    out.walk_ms = t0.elapsed().as_millis() as u64;
    out
}

#[cfg(unix)]
fn allocated(md: &std::fs::Metadata) -> u64 {
    use std::os::unix::fs::MetadataExt;
    md.blocks() * 512
}

#[cfg(not(unix))]
fn allocated(md: &std::fs::Metadata) -> u64 {
    md.len()
}
