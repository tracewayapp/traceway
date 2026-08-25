pub mod disk;
pub mod proc;

use serde::Serialize;

#[derive(Clone, Debug, Default, Serialize, PartialEq)]
pub struct ContainerState {
    pub started_at: String,
    pub oom_killed: bool,
    pub status: String,
    pub restart_count: u64,
}

pub async fn container_state(name: &str) -> Option<ContainerState> {
    let out = tokio::process::Command::new("docker")
        .args(["inspect", "-f", "{{.State.StartedAt}}|{{.State.OOMKilled}}|{{.State.Status}}|{{.RestartCount}}", name])
        .output()
        .await
        .ok()?;
    if !out.status.success() {
        return None;
    }
    let s = String::from_utf8_lossy(&out.stdout);
    let mut it = s.trim().split('|');
    Some(ContainerState {
        started_at: it.next()?.to_string(),
        oom_killed: it.next()? == "true",
        status: it.next()?.to_string(),
        restart_count: it.next()?.parse().unwrap_or(0),
    })
}
