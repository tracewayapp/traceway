// Load generator for client metrics: many devices, each uploading the small aggregated payload the browser SDK
// sends when its page is hidden. It checks what the design promises: a request costs no storage write, the rows a
// flush writes depend on the number of dimension combinations and not on the number of devices, and nothing is
// dropped that should not be.
//
//   TRACEWAY_URL=http://localhost:8082 PROJECT_TOKEN=<browser project token> HEALTH_DEEP_TOKEN=<token> \
//     node client-metrics-load.mjs [devices=100000] [concurrency=200]
//
// Without HEALTH_DEEP_TOKEN it only reports request latency and status codes. The server must trust the
// X-Forwarded-For of this machine (loopback is trusted by default), or every device counts as one address.
import assert from 'node:assert/strict'

const API = (process.env.TRACEWAY_URL ?? 'http://localhost:8082').replace(/\/+$/, '')
const TOKEN = process.env.PROJECT_TOKEN
const HEALTH_TOKEN = process.env.HEALTH_DEEP_TOKEN
const DEVICES = Number(process.argv[2] ?? 100_000)
const CONCURRENCY = Number(process.argv[3] ?? 200)
assert(TOKEN, 'PROJECT_TOKEN is required: the token of a browser project')

const RELEASES = ['4.0.0', '4.1.0', '4.2.0']
const BROWSERS = [['Chrome', '142'], ['Safari', '19'], ['Firefox', '144'], ['Edge', '142']]
const SYSTEMS = ['Android', 'iOS', 'Windows', 'macOS']
const ROUTES = ['/', '/products', '/products/:id', '/cart', '/checkout', '/orders/:id', '/account', '/search']
const NETWORKS = ['4g', '3g', 'wifi']
const HOSTS = ['api.shop.example', 'cdn.shop.example']

const pick = (list, n) => list[n % list.length]
const deviceAddress = (n) => `10.${(n >> 16) & 255}.${(n >> 8) & 255}.${n & 255}`
// A log-normal: what page timings look like.
const sample = (median, spread) => median * Math.exp(spread * gaussian())
function gaussian() {
  let u = 0
  while (u === 0) u = Math.random()
  return Math.sqrt(-2 * Math.log(u)) * Math.cos(2 * Math.PI * Math.random())
}

const SCALE = 3
const bucketIndex = (value) => Math.ceil(Math.log2(value) * 2 ** SCALE) - 1
const attrs = (object) =>
  Object.entries(object).map(([key, value]) => ({ key, value: { stringValue: String(value) } }))

function histogram(name, unit, dims, values, times) {
  const counts = new Map()
  for (const value of values) counts.set(bucketIndex(value), (counts.get(bucketIndex(value)) ?? 0) + 1)
  const low = Math.min(...counts.keys())
  const bucketCounts = new Array(Math.max(...counts.keys()) - low + 1).fill(0)
  for (const [index, count] of counts) bucketCounts[index - low] = count
  return {
    name,
    unit,
    exponentialHistogram: {
      aggregationTemporality: 1,
      dataPoints: [{
        attributes: attrs(dims),
        ...times,
        count: values.length,
        sum: values.reduce((a, b) => a + b, 0),
        scale: SCALE,
        zeroCount: 0,
        min: Math.min(...values),
        max: Math.max(...values),
        positive: { offset: low, bucketCounts }
      }]
    }
  }
}

const sum = (name, dims, value, times) => ({
  name,
  sum: { aggregationTemporality: 1, isMonotonic: true, dataPoints: [{ attributes: attrs(dims), ...times, asDouble: value }] }
})

function rating(value, good, poor) {
  return value <= good ? 'good' : value <= poor ? 'needs-improvement' : 'poor'
}

// One session's upload: the web vitals of a page view, a route change, some long tasks, requests and counters.
function payload(device) {
  const now = Date.now()
  const times = { startTimeUnixNano: `${now - 30_000}000000`, timeUnixNano: `${now}000000` }
  const [browser, major] = pick(BROWSERS, device)
  const os = pick(SYSTEMS, device >> 2)
  const view = { 'url.path': pick(ROUTES, device >> 1), 'network.type': pick(NETWORKS, device >> 3) }
  const lcp = sample(1900, 0.5)
  const inp = sample(140, 0.6)
  const cls = Math.abs(sample(0.04, 0.9))
  const vital = (value, good, poor) => ({ ...view, rating: rating(value, good, poor), navigation_type: 'navigate' })
  return JSON.stringify({
    resourceMetrics: [{
      resource: {
        attributes: [
          ...attrs({
            'service.name': 'shop',
            'service.version': pick(RELEASES, device >> 4),
            'os.name': os,
            'browser.name': browser,
            'browser.major': major,
            'device.form_factor': os === 'Android' || os === 'iOS' ? 'phone' : 'desktop',
            'device.class': pick(['low', 'mid', 'high'], device >> 5),
            'traceway.metrics': 'aggregated'
          }),
          { key: 'traceway.sample_rate', value: { doubleValue: 1 } },
          { key: 'traceway.sent_at', value: { doubleValue: now } }
        ]
      },
      scopeMetrics: [{
        scope: { name: '@tracewayapp/frontend', version: '2.1.0' },
        metrics: [
          histogram('browser.web_vital.lcp', 'ms', vital(lcp, 2500, 4000), [lcp], times),
          histogram('browser.web_vital.inp', 'ms', vital(inp, 200, 500), [inp], times),
          histogram('browser.web_vital.cls', '', vital(cls, 0.1, 0.25), [cls], times),
          histogram('browser.web_vital.fcp', 'ms', vital(lcp * 0.6, 1800, 3000), [lcp * 0.6], times),
          histogram('browser.web_vital.ttfb', 'ms', vital(lcp * 0.2, 800, 1800), [lcp * 0.2], times),
          histogram('browser.route_timing.duration', 'ms', { ...view, outcome: 'settled', type: 'push' }, [sample(450, 0.5)], times),
          histogram('browser.long_task.duration', 'ms', { ...view, type: 'long-animation-frame' }, [sample(90, 0.4), sample(90, 0.4)], times),
          histogram('browser.request.duration', 'ms', { host: pick(HOSTS, device) }, [sample(180, 0.6), sample(180, 0.6), sample(180, 0.6)], times),
          sum('browser.request.failed.total', { host: pick(HOSTS, device) }, 3, times),
          sum('browser.page_view', view, 1, times),
          sum('browser.session', view, 1, times)
        ]
      }]
    }]
  })
}

async function health() {
  if (!HEALTH_TOKEN) return null
  const response = await fetch(`${API}/api/health/deep`, { headers: { Authorization: `Bearer ${HEALTH_TOKEN}` } })
  assert(response.ok, `/api/health/deep answered ${response.status}`)
  return (await response.json()).clientMetrics
}

const before = await health()
const statuses = new Map()
const latencies = []
let next = 0
let bytes = 0
const started = performance.now()

async function worker() {
  while (next < DEVICES) {
    const device = next++
    const body = payload(device)
    bytes += body.length
    const sent = performance.now()
    try {
      const response = await fetch(`${API}/api/otel/v1/metrics`, {
        method: 'POST',
        // One address per device: the server caps what a single address may add to a project per minute.
        headers: { Authorization: `Bearer ${TOKEN}`, 'Content-Type': 'application/json', 'X-Forwarded-For': deviceAddress(device) },
        body
      })
      await response.arrayBuffer()
      statuses.set(response.status, (statuses.get(response.status) ?? 0) + 1)
    } catch {
      statuses.set('error', (statuses.get('error') ?? 0) + 1)
    }
    latencies.push(performance.now() - sent)
  }
}

await Promise.all(Array.from({ length: CONCURRENCY }, worker))
const seconds = (performance.now() - started) / 1000
latencies.sort((a, b) => a - b)
const percentile = (p) => latencies[Math.min(latencies.length - 1, Math.floor(p * latencies.length))].toFixed(1)

console.log(`${DEVICES} devices in ${seconds.toFixed(1)}s: ${(DEVICES / seconds).toFixed(0)} requests/s, ${(bytes / DEVICES / 1024).toFixed(1)} KB per payload`)
console.log(`latency ms: p50 ${percentile(0.5)}, p95 ${percentile(0.95)}, p99 ${percentile(0.99)}`)
console.log('status codes:', Object.fromEntries(statuses))

if (before) {
  // Let the last flush interval pass.
  await new Promise((resolve) => setTimeout(resolve, Number(process.env.FLUSH_WAIT_MS ?? 12_000)))
  const after = await health()
  const requests = after.requests - before.requests
  const observations = after.observations - before.observations
  const dropped = after.droppedTotal - before.droppedTotal
  const flushes = after.flushes - before.flushes
  console.log(`aggregator: ${requests} requests, ${observations} data points, ${flushes} flushes, last flush ${after.lastFlushRows} rows in ${after.lastFlushMs} ms, ${after.cells} cells held, ${dropped} dropped`)
  console.log('dropped by reason:', after.dropped)
  const accepted = statuses.get(200) ?? 0
  assert.equal(requests, accepted, 'every accepted request reached the aggregator')
  assert.equal(observations, accepted * 11, 'every data point of every accepted payload was aggregated')
  assert.equal(dropped, 0, 'nothing was dropped')
  assert(after.cells < after.maxCells / 10, 'the cells held stay far below the cap')
  console.log('ok: storage saw', flushes, 'flushes for', accepted, 'requests')
}
