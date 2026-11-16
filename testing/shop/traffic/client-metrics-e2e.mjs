// End-to-end check of client metrics (performance histograms aggregated on the device), against a local stack:
// examples/devtesting-embedded (Traceway on :8082), a built dashboard, and this shop with the 2.1 browser SDK.
//
//   HEALTH_DEEP_TOKEN=... DASHBOARD_URL=http://localhost:5180 node client-metrics-e2e.mjs [screenshot dir]
//
// Start the backend with HEALTH_DEEP_TOKEN, GEOIP_COUNTRY_HEADER=X-Test-Country and, to keep the run short,
// NOTIFICATION_POLL_SECONDS=10 and SLO_POLL_SECONDS=10. The storefront is visited as two releases on desktops and
// phones from two countries; release 0.2.0 is made slow on purpose. The script then reads what was uploaded on the
// wire, what the server stored, and what the dashboard, an alert rule and an SLO make of it.
import assert from 'node:assert/strict'
import { createServer } from 'node:http'
import { mkdirSync } from 'node:fs'
import path from 'node:path'
import { chromium, devices } from 'playwright'

const DASHBOARD = process.env.DASHBOARD_URL ?? 'http://localhost:5180'
const API = process.env.TRACEWAY_URL ?? 'http://localhost:8082'
const SHOP = process.env.SHOP_URL ?? 'http://localhost:5175'
const PROJECT = process.env.SHOP_PROJECT ?? 'Shoply Storefront'
const MOBILE_PROJECT = process.env.MOBILE_PROJECT ?? 'Flutter App'
const MOBILE_TOKEN = process.env.MOBILE_TOKEN ?? 'flutter-dev-token'
const EMAIL = process.env.TRACEWAY_EMAIL ?? 'admin@localhost.com'
const PASSWORD = process.env.TRACEWAY_PASSWORD ?? 'admin'
const HEALTH_TOKEN = process.env.HEALTH_DEEP_TOKEN ?? ''
const COUNTRY_HEADER = process.env.COUNTRY_HEADER ?? 'X-Test-Country'
const OUT = path.resolve(process.argv[2] ?? 'screenshots-client-metrics')
const ALERT_WAIT_MS = Number(process.env.ALERT_WAIT_MS ?? 240_000)
const SKIP_TRAFFIC = process.env.SKIP_TRAFFIC === '1'
const HEADLESS = process.env.HEADED !== '1'

const OLD = '0.1.0'
const NEW = '0.2.0'
const SLOW_CLICK_MS = 650
const VISITS = [
  { name: 'Mia Chen', email: 'mia.chen@example.com', plan: 'free', release: OLD, country: 'DE', phone: false },
  { name: 'Liam Novak', email: 'liam.novak@example.com', plan: 'pro', release: OLD, country: 'US', phone: false },
  { name: 'Sofia Ricci', email: 'sofia.ricci@example.com', plan: 'free', release: OLD, country: 'US', phone: true },
  { name: 'Noah Kim', email: 'noah.kim@example.com', plan: 'pro', release: OLD, country: 'DE', phone: false },
  { name: 'Emma Stone', email: 'emma.stone@example.com', plan: 'pro', release: NEW, country: 'DE', phone: false },
  { name: 'Lucas Meyer', email: 'lucas.meyer@example.com', plan: 'free', release: NEW, country: 'US', phone: true },
  { name: 'Ava Rossi', email: 'ava.rossi@example.com', plan: 'free', release: NEW, country: 'US', phone: false },
  { name: 'Ben Okafor', email: 'ben.okafor@example.com', plan: 'pro', release: NEW, country: 'DE', phone: false }
]
// Visitors who read the landing page and leave without touching it.
const BOUNCES = 3
const RESEND_VISITOR = { name: 'Zoe Park', email: 'zoe.park@example.com', plan: 'pro', release: OLD, country: 'FR', phone: false }

mkdirSync(OUT, { recursive: true })
let step = 0
async function shot(page, name, options = {}) {
  step++
  const file = `${String(step).padStart(2, '0')}-${name}.png`
  await page.waitForTimeout(500)
  for (let attempt = 1; ; attempt++) {
    try {
      await page.screenshot({ path: path.join(OUT, file), ...options })
      break
    } catch (error) {
      if (attempt === 3) throw error
      await page.waitForTimeout(1000)
    }
  }
  console.log(`  screenshot ${file}`)
}
const failures = []
async function check(name, run) {
  try {
    await run()
    console.log(`PASS ${name}`)
  } catch (error) {
    failures.push(name)
    console.log(`FAIL ${name}\n     ${String(error.message ?? error).split('\n').join('\n     ')}`)
    if (process.env.FAIL_FAST === '1') throw error
  }
}
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms))

async function api(token, method, route, body, projectId) {
  const url = new URL(`${API}/api${route}`)
  if (projectId) url.searchParams.set('projectId', projectId)
  const response = await fetch(url, {
    method,
    headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body)
  })
  const text = await response.text()
  assert(response.ok, `${method} ${route}: ${response.status} ${text}`)
  return text ? JSON.parse(text) : null
}
async function health() {
  const response = await fetch(`${API}/api/health/deep`, { headers: { Authorization: `Bearer ${HEALTH_TOKEN}` } })
  assert(response.ok, `/api/health/deep answered ${response.status}; start the backend with HEALTH_DEEP_TOKEN and pass it here`)
  return (await response.json()).clientMetrics
}

const login = await fetch(`${API}/api/login`, {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ email: EMAIL, password: PASSWORD })
})
assert(login.ok, `login failed: ${login.status}`)
const { token } = await login.json()
const projects = await api(token, 'GET', '/projects')
const project = projects.find((p) => p.name === PROJECT)
const mobileProject = projects.find((p) => p.name === MOBILE_PROJECT)
assert(project, `project "${PROJECT}" not found; start examples/devtesting-embedded`)
assert(mobileProject, `project "${MOBILE_PROJECT}" not found`)

for (const rule of await api(token, 'GET', '/notification-rules', undefined, project.id).then((r) => r.rules))
  await api(token, 'DELETE', `/notification-rules/${rule.id}`, undefined, project.id)
for (const channel of await api(token, 'GET', '/notification-channels', undefined, project.id).then((r) => r.channels))
  await api(token, 'DELETE', `/notification-channels/${channel.id}`, undefined, project.id)
for (const rule of (await api(token, 'GET', '/sdk-config/rules', undefined, project.id)).data)
  await api(token, 'DELETE', `/sdk-config/rules/${rule.id}`, undefined, project.id)
for (const slo of (await api(token, 'GET', '/slos', undefined, project.id)).slos ?? [])
  await api(token, 'DELETE', `/slos/${slo.id}`, undefined, project.id)

const webhookCalls = []
const webhook = createServer(async (req, res) => {
  const chunks = []
  for await (const chunk of req) chunks.push(chunk)
  const body = Buffer.concat(chunks).toString()
  if (body) webhookCalls.push(body)
  res.end('ok')
})
await new Promise((resolve) => webhook.listen(0, '127.0.0.1', resolve))
const webhookUrl = `http://127.0.0.1:${webhook.address().port}/alerts`

const startedAt = new Date()
const range = () => ({ from: new Date(startedAt.getTime() - 120_000).toISOString(), to: new Date(Date.now() + 120_000).toISOString() })
async function query(item, wholeRange = true) {
  const result = await api(token, 'POST', '/metrics/query', { queries: [item], ...range(), wholeRange }, project.id)
  return result.results[0]
}
const groupValues = (result) => Object.fromEntries(Object.entries(result.series).map(([key, points]) => [key, points.reduce((sum, p) => sum + p.value, 0)]))

// ---------------------------------------------------------------------------------------------------------------
// Traffic
// ---------------------------------------------------------------------------------------------------------------

const attributesOf = (list) => Object.fromEntries((list ?? []).map((a) => [a.key, Object.values(a.value ?? {})[0]]))
function readPayload(request) {
  const body = JSON.parse(request.postData() ?? '{}')
  const out = { resource: {}, metrics: [], authorization: request.headers().authorization ?? '' }
  for (const resourceMetrics of body.resourceMetrics ?? []) {
    out.resource = attributesOf(resourceMetrics.resource?.attributes)
    for (const scope of resourceMetrics.scopeMetrics ?? []) for (const metric of scope.metrics ?? []) out.metrics.push(metric)
  }
  return out
}
const pointsOf = (metric) => (metric.exponentialHistogram ?? metric.sum ?? metric.histogram ?? metric.gauge)?.dataPoints ?? []

const uploads = new Map() // visitor email -> payloads
const eventNames = new Map() // visitor email -> log event names
const storageKeys = new Map()
const before = HEALTH_TOKEN ? await health() : null

async function visit(shopBrowser, visitor, { resend = false } = {}) {
  const context = await shopBrowser.newContext({
    ...(visitor.phone ? devices['Pixel 7'] : { viewport: { width: 1280, height: 800 } })
  })
  await context.addInitScript(
    ({ user, release, slow, slowMs }) => {
      window.__DEMO_USER = user
      window.__DEMO_VERSION = release
      if (!slow) return
      // A release whose click handlers block the main thread. Registered on window, so it runs after the app's own
      // handler has changed the route.
      window.addEventListener('click', () => {
        const until = performance.now() + slowMs
        while (performance.now() < until);
      })
    },
    { user: { name: visitor.name, email: visitor.email, plan: visitor.plan }, release: visitor.release, slow: visitor.release === NEW, slowMs: SLOW_CLICK_MS }
  )
  const shop = await context.newPage()
  const payloads = []
  const events = []
  uploads.set(visitor.email, payloads)
  eventNames.set(visitor.email, events)
  shop.on('request', (request) => {
    if (request.method() !== 'POST') return
    try {
      if (request.url().endsWith('/api/otel/v1/metrics')) payloads.push(readPayload(request))
      if (request.url().endsWith('/api/otel/v1/logs'))
        for (const resource of JSON.parse(request.postData() ?? '{}').resourceLogs ?? [])
          for (const scope of resource.scopeLogs ?? []) for (const record of scope.logRecords ?? []) events.push(record.eventName)
    } catch (error) {
      payloads.push({ unreadable: String(error) })
    }
  })
  // What a CDN in front of Traceway does: it names the visitor's country in a header. Set here and not as an extra
  // header of the browser, which would turn it into a header the page itself sends and fail the CORS preflight.
  let interrupted = 0
  await context.route(`${API}/**`, async (route) => {
    const request = route.request()
    const headers = { ...request.headers(), [COUNTRY_HEADER]: visitor.country }
    if (!resend || interrupted > 0 || request.method() !== 'POST' || !request.url().endsWith('/api/otel/v1/metrics'))
      return route.continue({ headers })
    // The upload reaches the server, but the page never sees the answer: what a page that dies mid-request looks like.
    interrupted++
    await route.fetch({ headers }).catch(() => {})
    await route.abort('connectionreset')
  })
  const hide = async () => {
    await shop.evaluate(() => {
      Object.defineProperty(document, 'visibilityState', { value: 'hidden', configurable: true })
      document.dispatchEvent(new Event('visibilitychange'))
    })
    await shop.waitForTimeout(2500)
  }
  const banner = shop.getByRole('banner')
  const tap = (locator) => (visitor.phone ? locator.tap() : locator.click())
  await shop.goto(SHOP)
  await shop.getByRole('button', { name: 'Add to cart' }).first().waitFor({ timeout: 30_000 })
  await shop.waitForTimeout(1500)
  if (visitor.release === NEW) {
    // A banner that pushes the page down a second after it rendered: a layout shift without user input.
    await shop.evaluate(() => {
      const late = document.createElement('div')
      late.id = 'late-promo-banner'
      late.style.cssText = 'height:420px;background:#fde68a'
      late.textContent = 'Late promo banner'
      document.body.prepend(late)
    })
    await shop.waitForTimeout(1200)
  }
  await tap(shop.getByRole('button', { name: 'Add to cart' }).first())
  await shop.waitForTimeout(800)
  await tap(banner.getByRole('button', { name: /^Cart, \d+ items$/ }))
  await shop.waitForTimeout(2000)
  await tap(banner.getByRole('button', { name: 'Checkout', exact: true }))
  await shop.waitForTimeout(2000)
  await tap(banner.getByRole('button', { name: 'Shop', exact: true }))
  await shop.waitForTimeout(2500)
  if (visitor === VISITS[0]) await shot(shop, 'storefront-desktop')
  if (visitor.phone && visitor.release === NEW) await shot(shop, 'storefront-phone-slow-release')
  await hide()
  if (resend) {
    assert.equal(interrupted, 1, 'the first upload was interrupted')
    storageKeys.set('after-failure', await shop.evaluate(() => Object.keys(localStorage).filter((key) => /traceway|tw[_:.-]/i.test(key))))
    await shop.reload()
    await shop.getByRole('button', { name: 'Add to cart' }).first().waitFor({ timeout: 30_000 })
    await shop.waitForTimeout(2500)
    // LCP is final at the first interaction or when the page is really hidden; the hide here is simulated.
    await tap(shop.getByRole('button', { name: 'Add to cart' }).first())
    await shop.waitForTimeout(1500)
    await hide()
  }
  storageKeys.set(visitor.email, await shop.evaluate(() => Object.keys(localStorage)))
  await context.close()
}

if (!SKIP_TRAFFIC) {
  await check('the storefront is visited as two releases, on desktops and phones, from two countries', async () => {
    const shopBrowser = await chromium.launch({ headless: HEADLESS })
    for (const visitor of VISITS) {
      try {
        await visit(shopBrowser, visitor)
      } catch (error) {
        if (!/crashed/i.test(String(error))) throw error
        console.log(`  ${visitor.name}: the tab crashed, visiting again`)
        await visit(shopBrowser, visitor)
      }
    }
    await visit(shopBrowser, RESEND_VISITOR, { resend: true })
    // A real departure, with nothing between the page and Traceway: the last upload leaves while the page unloads,
    // which is also the only moment an untouched page knows its LCP.
    for (let i = 0; i < BOUNCES; i++) {
      const context = await shopBrowser.newContext({ viewport: { width: 1280, height: 800 } })
      await context.addInitScript(
        ({ user, release }) => {
          window.__DEMO_USER = user
          window.__DEMO_VERSION = release
        },
        { user: { name: `Bounce ${i}`, email: `bounce${i}@example.com`, plan: 'bounce' }, release: OLD }
      )
      const shop = await context.newPage()
      await shop.goto(SHOP)
      await shop.getByRole('button', { name: 'Add to cart' }).first().waitFor({ timeout: 30_000 })
      await shop.waitForTimeout(2500)
      if (i % 2) await shop.goto('about:blank')
      else await shop.close({ runBeforeUnload: true })
      await sleep(2500)
      await context.close()
    }
    await shopBrowser.close()
  })

  await check('each visit uploads a few small aggregated payloads, not one request per measurement', async () => {
    for (const visitor of VISITS) {
      const payloads = uploads.get(visitor.email)
      assert(payloads.length >= 1 && payloads.length <= 3, `${visitor.name}: ${payloads.length} metric uploads`)
      for (const payload of payloads) {
        assert(!payload.unreadable, `${visitor.name}: unreadable upload ${payload.unreadable}`)
        assert.equal(payload.resource['traceway.metrics'], 'aggregated')
        assert.equal(payload.resource['service.version'], visitor.release)
        assert.match(String(payload.resource['traceway.batch_id']), /^[\w-]{8,}$/)
        assert.equal(payload.resource['device.form_factor'], visitor.phone ? 'phone' : 'desktop')
        assert.equal(payload.resource['custom.plan'], visitor.plan, 'the dimension the app set')
        assert.equal(payload.resource.country, undefined, 'the client never names its country')
      }
      const names = new Set(payloads.flatMap((payload) => payload.metrics.map((metric) => metric.name)))
      for (const name of ['browser.web_vital.lcp', 'browser.web_vital.fcp', 'browser.route_timing.duration', 'browser.page_view', 'browser.session', 'browser.user'])
        assert(names.has(name), `${visitor.name}: ${name} is not among ${[...names].join(', ')}`)
      const lcp = payloads.flatMap((payload) => payload.metrics).find((metric) => metric.name === 'browser.web_vital.lcp')
      assert(lcp.exponentialHistogram, 'a web vital is an exponential histogram')
      assert.equal(lcp.exponentialHistogram.aggregationTemporality, 1, 'delta temporality')
      assert.equal(lcp.exponentialHistogram.dataPoints[0].scale, 3)
    }
    const sample = uploads.get(VISITS[0].email)[0]
    console.log(`  resource of an upload: ${JSON.stringify(sample.resource)}`)
    console.log(`  metrics of an upload: ${sample.metrics.map((metric) => metric.name).join(', ')}`)
  })

  await check('no upload carries a session, user or device id as a dimension, nor a full URL', async () => {
    const identifying = /^(session\.id|user\.|enduser\.|service\.instance\.id|client\.|url\.full|url\.query)/
    // A script is named by origin and path; nothing else may look like a URL, and nothing may carry a query.
    const urlAllowed = new Set(['script'])
    for (const [email, payloads] of uploads) {
      for (const payload of payloads) {
        for (const key of Object.keys(payload.resource)) assert(!identifying.test(key), `${email}: resource attribute ${key}`)
        for (const metric of payload.metrics)
          for (const point of pointsOf(metric)) {
            for (const [key, value] of Object.entries(attributesOf(point.attributes))) {
              assert(!identifying.test(key), `${email}: ${metric.name} has dimension ${key}`)
              assert(!String(value).includes(email), `${email}: ${metric.name} ${key}=${value}`)
              assert(urlAllowed.has(key) || !/^https?:/.test(String(value)), `${email}: ${metric.name} ${key}=${value}`)
              if (/^https?:/.test(String(value))) assert(!/[?#]/.test(String(value)), `${email}: ${metric.name} ${key}=${value}`)
            }
          }
      }
    }
  })

  await check('only values rated poor carry a session, as an exemplar with the element behind it', async () => {
    const exemplars = []
    for (const visitor of VISITS)
      for (const payload of uploads.get(visitor.email))
        for (const metric of payload.metrics)
          for (const point of pointsOf(metric))
            for (const exemplar of point.exemplars ?? [])
              exemplars.push({ release: visitor.release, metric: metric.name, rating: attributesOf(point.attributes).rating, value: exemplar.asDouble, ...attributesOf(exemplar.filteredAttributes) })
    console.log(`  exemplars sent: ${JSON.stringify(exemplars.slice(0, 4))}${exemplars.length > 4 ? ` and ${exemplars.length - 4} more` : ''}`)
    assert(exemplars.length > 0, 'the slow release sends exemplars')
    for (const exemplar of exemplars) {
      assert.equal(exemplar.release, NEW, `${exemplar.metric} of the fast release has an exemplar`)
      assert.match(String(exemplar['session.id']), /^[0-9a-f-]{36}$/)
      if (exemplar.rating !== undefined) assert.equal(exemplar.rating, 'poor')
    }
    assert(exemplars.some((exemplar) => exemplar.metric.endsWith('.inp') && exemplar['traceway.target']), 'a poor INP names its element')
  })

  await check('an SDK that aggregates does not also upload one event per web vital', async () => {
    for (const visitor of VISITS) {
      const vitals = eventNames.get(visitor.email).filter((name) => name === 'browser.web_vital')
      assert.equal(vitals.length, 0, `${visitor.name} streamed ${vitals.length} web vital events`)
    }
  })

  await check('the SDK keeps two entries in the browser per project token', async () => {
    const keys = storageKeys.get(VISITS[0].email)
    console.log(`  localStorage of a visitor: ${JSON.stringify(keys)}`)
    console.log(`  localStorage after a failed upload: ${JSON.stringify(storageKeys.get('after-failure'))}`)
    const own = keys.filter((key) => /traceway/i.test(key))
    assert(own.length >= 1, 'the daily user marker is stored')
    for (const key of keys) assert(!/@|example\.com/.test(key), `storage key ${key} names a person`)
  })

  await check('an upload whose answer never arrived is sent once more and counted once', async () => {
    const payloads = uploads.get(RESEND_VISITOR.email)
    const ids = payloads.map((payload) => payload.resource['traceway.batch_id'])
    const retried = payloads.filter((payload) => payload.resource['traceway.retry'])
    console.log(`  batch ids: ${ids.join(', ')}; resent: ${retried.map((payload) => payload.resource['traceway.batch_id']).join(', ')}`)
    assert.equal(ids[0], ids[1], 'the upload after the reload is the interrupted one')
    assert.equal(payloads[0].resource['traceway.retry'], undefined)
    assert(payloads[1].resource['traceway.retry'], 'and it says it is a resend')
    // The reload itself ends a page whose last upload may not have been answered: that one is resent too.
    for (const payload of retried) assert.equal(ids.filter((id) => id === payload.resource['traceway.batch_id']).length <= 2, true, 'no payload is sent a third time')
    if (!before) return console.log('  (no HEALTH_DEEP_TOKEN: the duplicate counter was not read)')
    const after = await health()
    const recognized = after.duplicatePayloads - before.duplicatePayloads
    assert(recognized >= 1 && recognized <= retried.length, `the server recognized ${recognized} of ${retried.length} resends`)
  })
}

// ---------------------------------------------------------------------------------------------------------------
// Storage, through the query API
// ---------------------------------------------------------------------------------------------------------------

const storageCheck = SKIP_TRAFFIC ? async (name) => console.log(`SKIP ${name}`) : check
await sleep(SKIP_TRAFFIC ? 0 : 13_000)

await storageCheck('nothing was dropped at ingest', async () => {
  if (!before) return console.log('  (no HEALTH_DEEP_TOKEN: skipped)')
  const after = await health()
  console.log(`  ingest: ${JSON.stringify({ requests: after.requests - before.requests, observations: after.observations - before.observations, flushes: after.flushes - before.flushes, lastFlushRows: after.lastFlushRows, dropped: after.dropped })}`)
  for (const [reason, count] of Object.entries(after.dropped)) {
    if (reason === 'duplicate') continue
    assert.equal(count - (before.dropped[reason] ?? 0), 0, `dropped for ${reason}`)
  }
  assert(after.flushes > before.flushes, 'the cells were written')
})

await storageCheck('the server stores the metrics as distributions and answers percentiles', async () => {
  const discovered = await api(token, 'GET', `/metrics/discover?from=${range().from}&to=${range().to}`, undefined, project.id)
  const byName = new Map(discovered.metrics.map((metric) => [metric.name, metric]))
  for (const name of [
    'browser.web_vital.lcp',
    'browser.web_vital.inp',
    'browser.web_vital.cls',
    'browser.web_vital.fcp',
    'browser.web_vital.ttfb',
    'browser.route_timing.duration',
    'browser.long_task.duration',
    'browser.long_task.blocking_time',
    'browser.page_load.duration',
    'browser.request.duration',
    'browser.view.inp',
    'browser.view.cls',
    'browser.page_view',
    'browser.session',
    'browser.user',
    'browser.user.affected',
    'browser.web_vital.attribution'
  ])
    assert(byName.has(name), `${name} is not among ${[...byName.keys()].join(', ')}`)
  const lcp = await query({ name: 'browser.web_vital.lcp', aggregation: 'p75' })
  const p75 = lcp.series.__all__[0].value
  assert(p75 > 20 && p75 < 20_000, `p75 LCP = ${p75}`)
  const count = (await query({ name: 'browser.web_vital.lcp', aggregation: 'count' })).series.__all__[0].value
  console.log(`  p75 LCP ${p75.toFixed(0)} ms over ${count} page loads`)
  assert.equal(count, VISITS.length + 2 + BOUNCES, 'one LCP per page load: every visit, two loads of the resend visitor, and the visitors who left without a click')
  const bounced = groupValues(await query({ name: 'browser.web_vital.lcp', aggregation: 'count', groupBy: 'custom.plan' })).bounce
  assert.equal(bounced, BOUNCES, 'a page nobody touched still reports its LCP when it is left')
})

await storageCheck('the slow release is visible by release, and only there', async () => {
  const inp = groupValues(await query({ name: 'browser.web_vital.inp', aggregation: 'p75', groupBy: 'service.version' }))
  const route = groupValues(await query({ name: 'browser.route_timing.duration', aggregation: 'p75', groupBy: 'service.version' }))
  const good = groupValues(await query({ name: 'browser.web_vital.inp', aggregation: 'good_ratio', groupBy: 'service.version' }))
  console.log(`  p75 INP by release ${JSON.stringify(inp)}, p75 route change ${JSON.stringify(route)}, good INP share ${JSON.stringify(good)}`)
  assert(inp[NEW] >= SLOW_CLICK_MS * 0.9, `p75 INP of ${NEW} is ${inp[NEW]}`)
  assert(inp[OLD] < 200, `p75 INP of ${OLD} is ${inp[OLD]}`)
  assert(good[OLD] > 0.9 && good[NEW] < 0.1, 'good_ratio follows')
})

await storageCheck('devices, countries and the app-defined dimension slice the same data', async () => {
  const formFactor = groupValues(await query({ name: 'browser.web_vital.lcp', aggregation: 'count', groupBy: 'device.form_factor' }))
  const country = groupValues(await query({ name: 'browser.web_vital.lcp', aggregation: 'count', groupBy: 'country' }))
  const plan = groupValues(await query({ name: 'browser.web_vital.lcp', aggregation: 'count', groupBy: 'custom.plan' }))
  const os = groupValues(await query({ name: 'browser.web_vital.lcp', aggregation: 'count', groupBy: 'os.name' }))
  const phoneNew = groupValues(await query({ name: 'browser.web_vital.inp', aggregation: 'p75', groupBy: 'device.form_factor', tagFilters: { 'service.version': NEW } }))
  console.log(`  page loads by form factor ${JSON.stringify(formFactor)}, country ${JSON.stringify(country)}, plan ${JSON.stringify(plan)}, OS ${JSON.stringify(os)}`)
  console.log(`  p75 INP of ${NEW} by form factor ${JSON.stringify(phoneNew)}`)
  assert.deepEqual(formFactor, { desktop: 8 + BOUNCES, phone: 2 })
  assert.deepEqual({ DE: country.DE, US: country.US, FR: country.FR }, { DE: 4, US: 4, FR: 2 }, 'the country comes from the request header the operator configured')
  assert.deepEqual(plan, { free: 4, pro: 6, bounce: BOUNCES })
  assert(phoneNew.phone > 0 && phoneNew.desktop > 0, 'a release can be read per device')
})

await storageCheck('users are counted once a day per browser profile, and by what went wrong for them', async () => {
  const users = (await query({ name: 'browser.user', aggregation: 'sum' })).series.__all__[0].value
  const affected = groupValues(await query({ name: 'browser.user.affected', aggregation: 'sum', groupBy: 'issue' }))
  const sessions = (await query({ name: 'browser.session', aggregation: 'sum' })).series.__all__[0].value
  console.log(`  users ${users}, sessions ${sessions}, affected ${JSON.stringify(affected)}`)
  assert.equal(users, VISITS.length + 1 + BOUNCES, 'the visitor who loaded the page twice is one user')
  assert.equal(affected.poor_inp, VISITS.filter((visitor) => visitor.release === NEW).length, 'every user of the slow release had a poor INP')
})

await storageCheck('slow values name the element and the page behind them', async () => {
  const targets = groupValues(await query({ name: 'browser.web_vital.attribution', aggregation: 'sum', groupBy: 'target' }))
  const vitals = groupValues(await query({ name: 'browser.web_vital.attribution', aggregation: 'sum', groupBy: 'vital' }))
  const viewInp = groupValues(await query({ name: 'browser.view.inp', aggregation: 'p75', groupBy: 'url.path' }))
  const scripts = groupValues(await query({ name: 'browser.long_task.script_time', aggregation: 'sum', groupBy: 'script' }))
  console.log(`  attribution by element ${JSON.stringify(targets)}\n  by vital ${JSON.stringify(vitals)}\n  p75 INP per view ${JSON.stringify(viewInp)}\n  script time ${JSON.stringify(scripts)}`)
  assert(Object.keys(targets).length > 0, 'at least one element is named')
  assert(vitals.inp > 0, 'INP is attributed')
  assert(Object.keys(viewInp).some((route) => route !== '/'), 'INP is recorded on the routes reached by a route change, not only on the landing page')
  const exemplars = await api(token, 'POST', '/metrics/exemplars', { name: 'browser.web_vital.inp', ...range() }, project.id)
  const list = exemplars.exemplars ?? exemplars.data ?? exemplars
  console.log(`  slow sessions: ${JSON.stringify(list.slice(0, 2))}`)
  assert(list.length > 0, 'slow sessions are listed')
})

const browser = await chromium.launch({ headless: HEADLESS })
const dashboard = await browser.newContext({ viewport: { width: 1440, height: 1100 }, timezoneId: 'UTC' })
const page = await dashboard.newPage()
const pageErrors = []
const apiErrors = []
page.on('pageerror', (error) => pageErrors.push(error.message))
page.on('response', (response) => {
  if (response.status() >= 400 && response.url().includes('/api/')) apiErrors.push(`${response.status()} ${response.request().method()} ${response.url()}`)
})
const open = (route, projectId = project.id, queryString = '') => page.goto(`${DASHBOARD}${route}?projectId=${projectId}${queryString}`)
const sidebarLink = (title) => page.locator('[data-sidebar="menu-button"]').filter({ has: page.getByText(title, { exact: true }) })
const dialog = () => page.getByRole('alertdialog').or(page.getByRole('dialog'))
// A widget card, found by its title.
const escaped = (text) => text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
const widget = (title) =>
  page
    .getByRole('listitem')
    .filter({ has: page.locator('[data-slot="card-title"]', { hasText: new RegExp(`^\\s*${escaped(title)}\\s*(\\(|$)`) }) })
    .first()
// Only this run's visits: an earlier run against the same instance is outside the range.
const sinceRun = () => `&from=${encodeURIComponent(range().from)}&to=${encodeURIComponent(range().to)}`
const widgetText = async (title) => (await widget(title).innerText()).replace(/\s+/g, ' ')
async function pick(label, option) {
  await page.getByRole('button', { name: new RegExp(`^${label}\\b`) }).first().click()
  await page.getByRole('option', { name: option, exact: true }).click()
  await page.waitForTimeout(2500)
}
async function selectIn(scope, labelText, option) {
  await scope.locator('div.space-y-2').filter({ has: page.getByText(labelText, { exact: true }) }).locator('button').first().click()
  await page.getByRole('option', { name: option }).first().click()
  await page.getByRole('option', { name: option }).first().waitFor({ state: 'detached' })
}

try {
  await check('sign in', async () => {
    await page.goto(`${DASHBOARD}/login`)
    await page.locator('#email').fill(EMAIL)
    await page.locator('#password').fill(PASSWORD)
    await page.getByRole('button', { name: 'Login', exact: true }).click()
    await page.waitForURL((url) => !url.pathname.startsWith('/login'))
  })

  await check('the Web Performance dashboard charts percentiles, ratings and breakdowns of the visits', async () => {
    await open('/dashboards', project.id, sinceRun())
    await sidebarLink('Dashboards').waitFor()
    await page.getByRole('tab', { name: /Web Performance/ }).waitFor()
    await widget('Largest Contentful Paint').waitFor()
    await page.waitForTimeout(5000)
    await shot(page, 'web-performance-dashboard', { fullPage: true })
    const inp = await widgetText('INP Ratings')
    assert.match(inp, /Good 60% 6/, `INP ratings: ${inp}`)
    assert.match(inp, /Poor 40% 4/, `INP ratings: ${inp}`)
    const cls = await widgetText('CLS Ratings')
    assert.match(cls, /Poor \d+% 4/, `CLS ratings: ${cls}`)
    const country = await widgetText('LCP by Country')
    for (const code of ['DE', 'US', 'FR']) assert(country.includes(code), `LCP by Country: ${country}`)
    const perView = await widgetText('INP by Page')
    for (const route of ['/cart', '/checkout']) assert(perView.includes(route), `INP by Page: ${perView}`)
    assert.match(await widgetText('Users Affected'), /poor_inp 4/)
    assert.match(await widgetText('Elements Behind Slow Vitals'), /button\./)
    assert.match(await widgetText('Scripts Blocking the Main Thread'), /\d+ ms/)
    assert.match(await widgetText('LCP by Release'), /0\.1\.0.*0\.2\.0/)
    for (const title of ['Elements Behind Slow Vitals', 'Scripts Blocking the Main Thread', 'Users Affected']) {
      const overflow = await widget(title).evaluate((card) => {
        const scroller = card.querySelector('.overflow-auto')
        return scroller ? scroller.scrollWidth - scroller.clientWidth : 0
      })
      assert(overflow <= 1, `${title} scrolls sideways by ${overflow}px`)
    }
  })

  await check('the dashboard is narrowed by release, OS and country', async () => {
    await pick('Release', NEW)
    let inp = await widgetText('INP Ratings')
    assert.match(inp, /Good 0% 0/, `INP ratings of ${NEW}: ${inp}`)
    assert.match(inp, /Poor 100% 4/, `INP ratings of ${NEW}: ${inp}`)
    await shot(page, 'dashboard-release-0.2.0')
    await pick('OS', 'Android')
    inp = await widgetText('INP Ratings')
    assert.match(inp, /Poor 100% 1/, `INP ratings of ${NEW} on Android: ${inp}`)
    await shot(page, 'dashboard-release-0.2.0-android')
    await pick('OS', 'All')
    await pick('Release', 'All')
    await pick('Country', 'FR')
    const lcp = await widgetText('LCP Ratings')
    assert.match(lcp, /(\D)2(\D|$)/, `LCP ratings in FR: ${lcp}`)
    assert.match(await widgetText('INP Ratings'), /Good 100%/)
    await shot(page, 'dashboard-country-fr')
    await pick('Country', 'All')
  })

  await check('a slow value leads to the session behind it', async () => {
    await widget('Interaction to Next Paint').getByText('Slow sessions').click()
    const link = page.locator('a[href*="/sessions/"]').first()
    await link.waitFor()
    await shot(page, 'slow-sessions-of-inp')
    const listed = await page.locator('a[href*="/sessions/"]').first().locator('xpath=ancestor::*[self::li or self::div][1]').innerText()
    console.log(`  first slow session: ${listed.replace(/\s+/g, ' ').slice(0, 200)}`)
    await link.click()
    await page.waitForURL(/\/sessions\/[0-9a-f-]{36}/)
    await page.waitForTimeout(4000)
    const body = await page.locator('main').last().innerText()
    assert(!/not found/i.test(body), 'the session exists')
    await shot(page, 'session-behind-a-slow-inp')
  })

  await check('a mobile project gets Mobile Performance one click away and charts app metrics sent over OTLP', async () => {
    const at = Date.now()
    const bucket = (value) => Math.ceil(Math.log2(value) * 8) - 1
    const point = (value, attributes) => ({
      attributes: Object.entries(attributes).map(([key, v]) => ({ key, value: { stringValue: v } })),
      startTimeUnixNano: `${at - 1000}000000`,
      timeUnixNano: `${at}000000`,
      count: 1,
      sum: value,
      scale: 3,
      zeroCount: 0,
      min: value,
      max: value,
      positive: { offset: bucket(value), bucketCounts: [1] }
    })
    const upload = (release, values) =>
      fetch(`${API}/api/otel/v1/metrics`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${MOBILE_TOKEN}`, 'Content-Type': 'application/json', [COUNTRY_HEADER]: 'JP' },
        body: JSON.stringify({
          resourceMetrics: [
            {
              resource: {
                attributes: Object.entries({ 'service.name': 'app', 'service.version': release, 'os.name': 'Android', 'device.form_factor': 'phone', 'traceway.metrics': 'aggregated' })
                  .map(([key, v]) => ({ key, value: { stringValue: v } }))
                  .concat([{ key: 'traceway.sent_at', value: { doubleValue: at } }])
              },
              scopeMetrics: [
                {
                  metrics: [
                    { name: 'app.start.duration', unit: 'ms', exponentialHistogram: { aggregationTemporality: 1, dataPoints: values.map((value) => point(value, { start_type: 'cold' })) } },
                    { name: 'app.screen.ttid', unit: 'ms', exponentialHistogram: { aggregationTemporality: 1, dataPoints: values.map((value) => point(value / 4, { screen: 'Checkout' })) } }
                  ]
                }
              ]
            }
          ]
        })
      })
    for (const [release, values] of [['3.0.0', [700, 800, 900]], ['3.1.0', [1900, 2100, 2300]]]) {
      const response = await upload(release, values)
      assert.equal(response.status, 200, `mobile upload answered ${response.status} ${await response.text()}`)
    }
    await sleep(12_000)
    await open('/dashboards', mobileProject.id, sinceRun())
    await sidebarLink('Dashboards').waitFor()
    const install = page.getByRole('button', { name: 'Add Mobile Performance Dashboard' })
    const tab = page.getByRole('tab', { name: /Mobile Performance/ })
    await install.or(tab).first().waitFor()
    if (await install.isVisible()) {
      await shot(page, 'mobile-dashboards-empty')
      await install.click()
    }
    await widget('App Start').waitFor()
    await page.waitForTimeout(5000)
    await shot(page, 'mobile-performance-dashboard', { fullPage: true })
    assert.match(await widgetText('App Start by Release'), /3\.0\.0.*3\.1\.0/)
    assert.match(await widgetText('App Start by Country'), /JP/)
    assert.match(await widgetText('Slowest Screens'), /Checkout/)
  })

  await check('alert rules on a percentile and on a release are built in the dialog', async () => {
    await api(token, 'POST', '/notification-channels', { name: 'Local webhook', channelType: 'webhook', config: { url: webhookUrl, method: 'POST' }, enabled: true }, project.id)
    const newRule = async (type) => {
      await open('/notifications', project.id, '&tab=rules')
      await page.getByRole('button', { name: 'New Rule' }).click()
      await selectIn(dialog(), 'Channel', /Local webhook/)
      await dialog().locator('div.space-y-2').filter({ has: page.getByText('Rule Type', { exact: true }) }).locator('button').first().click()
      await page.getByRole('option', { name: 'Metric Threshold' }).waitFor()
      const offered = (await page.getByRole('option').allTextContents()).map((option) => option.trim())
      assert.deepEqual(offered, ['New Issue', 'Metric Threshold', 'Release Regression', 'SLO Burn Rate', 'SLO Budget Consumed'])
      await page.getByRole('option', { name: type }).click()
    }
    const save = async (name) => {
      await dialog().getByRole('button', { name: 'New Rule', exact: true }).click()
      await dialog().waitFor({ state: 'detached' })
      await page.getByText(name).first().waitFor()
    }
    await newRule('Metric Threshold')
    await dialog().getByRole('button', { name: 'Slow interactions' }).click()
    assert.equal(await dialog().locator('#cfg-metric').inputValue(), 'browser.web_vital.inp')
    await dialog().locator('#cfg-lookback-m').fill('60')
    await dialog().locator('#cfg-min-count').fill('5')
    await shot(page, 'alert-rule-p75-inp')
    await save('Slow interactions (INP)')

    await newRule('Release Regression')
    await dialog().locator('#rule-name').fill('INP regressed in a release')
    await dialog().locator('#cfg-regression-metric').fill('browser.web_vital.inp')
    await dialog().locator('#cfg-regression-min').fill('3')
    await dialog().locator('#cfg-regression-lookback').fill('60')
    await shot(page, 'alert-rule-release-regression')
    await save('INP regressed in a release')
    await shot(page, 'alert-rules')
  })

  await check('both alerts fire: the percentile is over its limit and the new release is worse than the old', async () => {
    const deadline = Date.now() + ALERT_WAIT_MS
    const fired = () => ({ threshold: webhookCalls.some((call) => /Slow interactions/.test(call)), regression: webhookCalls.some((call) => /INP regressed in a release|0\.2\.0/.test(call) && !/Slow interactions/.test(call)) })
    while (!(fired().threshold && fired().regression) && Date.now() < deadline) await sleep(2000)
    for (const call of webhookCalls) console.log(`  alert: ${call.slice(0, 500)}`)
    assert(fired().threshold, `the p75 INP rule did not reach the webhook within ${ALERT_WAIT_MS / 1000} s`)
    assert(fired().regression, `the release regression rule did not reach the webhook within ${ALERT_WAIT_MS / 1000} s`)
    assert(webhookCalls.some((call) => call.includes(NEW) && call.includes(OLD)), 'the regression names both releases')
    await open('/notifications', project.id, '&tab=history')
    await page.getByText('INP regressed in a release').first().waitFor()
    await shot(page, 'alert-history')
  })

  await check('an SLO per release counts good interactions from the buckets', async () => {
    await open('/slos')
    await page.getByRole('button', { name: 'New SLO' }).first().click()
    const sheet = dialog()
    await sheet.locator('#slo-name').fill('Fast interactions')
    await selectIn(sheet, 'Indicator', 'Metric')
    await sheet.locator('#slo-metric').waitFor()
    await sheet.locator('#slo-metric').fill('browser.web_vital.inp')
    await sheet.locator('#slo-target').fill('90')
    await page.waitForTimeout(1500)
    await selectIn(sheet, 'One SLO per', 'service.version')
    const counts = sheet.locator('div.space-y-2').filter({ has: page.getByText(/^(Counts|Aggregate per slice)$/) }).locator('button').first()
    if (!/Every observation/.test(await counts.innerText())) {
      await counts.click()
      await page.getByRole('option', { name: 'Every observation' }).click()
    }
    await sheet.locator('#slo-metric-threshold').fill('200')
    await shot(page, 'slo-sheet-one-per-release')
    await sheet.getByRole('button', { name: 'New SLO', exact: true }).click()
    await sheet.waitFor({ state: 'detached' })
    let slos = []
    const of = (release) => slos.find((slo) => JSON.stringify(slo).includes(`"${release}"`))
    const deadline = Date.now() + 90_000
    while (Date.now() < deadline) {
      slos = (await api(token, 'GET', '/slos', undefined, project.id)).slos ?? []
      if (slos.length >= 2 && slos.every((slo) => slo.currentSli !== null && slo.currentSli !== undefined && slo.lastBucketAt)) break
      await sleep(3000)
    }
    console.log(`  SLOs: ${JSON.stringify(slos.map((slo) => ({ name: slo.name, sli: slo.currentSli, budgetRemaining: slo.budgetRemaining, status: slo.status, config: slo.sliConfig })))}`)
    assert(of(OLD) && of(NEW), 'one SLO per release')
    assert(of(OLD).currentSli > 99, `every interaction of ${OLD} is good`)
    assert(of(NEW).currentSli < 1, `no interaction of ${NEW} is good`)
    await page.reload()
    await page.getByText('Fast interactions').first().waitFor()
    await page.waitForTimeout(1500)
    await shot(page, 'slos-per-release')
    await page.getByText(new RegExp(`Fast interactions.*${NEW.replaceAll('.', '\\.')}`)).first().click()
    await page.waitForURL(/\/slos\/\d+|\/slos\/[0-9a-f-]+/)
    await page.waitForTimeout(3000)
    await shot(page, 'slo-detail-slow-release')
  })

  await check('the dashboard made no failed API call and threw no error', async () => {
    assert.deepEqual(pageErrors, [])
    assert.deepEqual(apiErrors, [])
  })
} catch (error) {
  await page.screenshot({ path: path.join(OUT, 'failure.png') }).catch(() => {})
  throw error
} finally {
  await browser.close()
  await new Promise((resolve) => webhook.close(resolve))
}

if (failures.length) {
  console.log(`\n${failures.length} check(s) failed:\n  ${failures.join('\n  ')}`)
  process.exit(1)
}
console.log('Client metrics end-to-end checks passed.')
