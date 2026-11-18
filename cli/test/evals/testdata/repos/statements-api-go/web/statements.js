// Statement generation in the billing dashboard.

const POLL_INTERVAL_MS = 2000;

export async function generateStatement(accountId, periodStart, periodEnd, onProgress) {
  const res = await fetch('/api/statements', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ accountId, periodStart, periodEnd }),
  });
  if (!res.ok) throw new Error(`statement request failed: ${res.status}`);
  const { id } = await res.json();
  onProgress?.('rendering');
  return waitUntilReady(id, onProgress);
}

async function waitUntilReady(id, onProgress) {
  for (;;) {
    const res = await fetch(`/api/statements/${id}`);
    if (!res.ok) throw new Error(`statement status failed: ${res.status}`);
    const st = await res.json();
    if (st.status === 'ready') {
      onProgress?.('ready');
      return `/api/statements/${id}/download`;
    }
    if (st.status === 'failed') throw new Error('statement rendering failed');
    await new Promise((r) => setTimeout(r, POLL_INTERVAL_MS));
  }
}
