use std::time::Duration;

pub fn client(timeout: Duration) -> reqwest::Client {
    reqwest::Client::builder()
        .no_proxy()
        .timeout(timeout)
        .pool_max_idle_per_host(64)
        .tcp_nodelay(true)
        .build()
        .expect("reqwest client")
}

pub async fn wait_until<F, Fut>(timeout: Duration, mut probe: F) -> anyhow::Result<()>
where
    F: FnMut() -> Fut,
    Fut: std::future::Future<Output = bool>,
{
    let deadline = std::time::Instant::now() + timeout;
    loop {
        if probe().await {
            return Ok(());
        }
        if std::time::Instant::now() > deadline {
            anyhow::bail!("database not healthy after {:?}", timeout);
        }
        tokio::time::sleep(Duration::from_secs(2)).await;
    }
}
