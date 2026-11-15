// End-to-end check of remote data collection and browser performance metrics, against a local stack:
// examples/devtesting-embedded (Traceway on :8082), the dashboard dev server, and this shop.
//
//   DASHBOARD_URL=http://localhost:5180 node data-collection-e2e.mjs [screenshot dir]
//
// It drives the dashboard the way a user would (the Data Collection page, a dashboard, an alert rule), sends
// real traffic through the storefront in Chromium, and fails on the first thing that is not as described.
import assert from 'node:assert/strict'
import { createServer } from 'node:http'
import { mkdirSync } from 'node:fs'
import path from 'node:path'
import { chromium } from 'playwright'

const DASHBOARD = process.env.DASHBOARD_URL ?? 'http://localhost:5180'
const API = process.env.TRACEWAY_URL ?? 'http://localhost:8082'
const SHOP = process.env.SHOP_URL ?? 'http://localhost:5175'
const PROJECT = process.env.SHOP_PROJECT ?? 'Shoply Storefront'
const BACKEND_PROJECT = process.env.BACKEND_PROJECT ?? 'Backend API'
const EMAIL = process.env.TRACEWAY_EMAIL ?? 'admin@localhost.com'
const PASSWORD = process.env.TRACEWAY_PASSWORD ?? 'admin'
const OUT = path.resolve(process.argv[2] ?? 'screenshots')
const ALERT_WAIT_MS = Number(process.env.ALERT_WAIT_MS ?? 150_000)

const TARGETED = { name: 'Mia Chen', email: 'mia.chen@example.com', plan: 'free' }
const OTHERS = [
  { name: 'Liam Novak', email: 'liam.novak@example.com', plan: 'pro' },
  { name: 'Sofia Ricci', email: 'sofia.ricci@example.com', plan: 'free' }
]

mkdirSync(OUT, { recursive: true })
let step = 0
async function shot(page, name) {
  step++
  const file = `${String(step).padStart(2, '0')}-${name}.png`
  await page.waitForTimeout(500)
  for (let attempt = 1; ; attempt++) {
    try {
      await page.screenshot({ path: path.join(OUT, file) })
      break
    } catch (error) {
      // Chromium occasionally refuses a capture while a dialog is still animating.
      if (attempt === 3) throw error
      await page.waitForTimeout(1000)
    }
  }
  console.log(`  screenshot ${file}`)
}
async function check(name, run) {
  await run()
  console.log(`PASS ${name}`)
}

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

const login = await fetch(`${API}/api/login`, {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ email: EMAIL, password: PASSWORD })
})
assert(login.ok, `login failed: ${login.status}`)
const { token } = await login.json()
const projects = await api(token, 'GET', '/projects')
const project = projects.find((p) => p.name === PROJECT)
const backendProject = projects.find((p) => p.name === BACKEND_PROJECT)
assert(project, `project "${PROJECT}" not found; start examples/devtesting-embedded`)
assert(backendProject, `project "${BACKEND_PROJECT}" not found`)

// Start from a known state, so the run can be repeated against the same instance.
for (const rule of (await api(token, 'GET', '/sdk-config/rules', undefined, project.id)).data)
  await api(token, 'DELETE', `/sdk-config/rules/${rule.id}`, undefined, project.id)
for (const rule of await api(token, 'GET', '/notification-rules', undefined, project.id).then((r) => r.rules))
  await api(token, 'DELETE', `/notification-rules/${rule.id}`, undefined, project.id)
for (const channel of await api(token, 'GET', '/notification-channels', undefined, project.id).then((r) => r.channels))
  await api(token, 'DELETE', `/notification-channels/${channel.id}`, undefined, project.id)

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

const browser = await chromium.launch({ headless: process.env.HEADED !== '1' })
const dashboard = await browser.newContext({ viewport: { width: 1440, height: 1100 }, timezoneId: 'UTC' })
const page = await dashboard.newPage()
const pageErrors = []
page.on('pageerror', (error) => pageErrors.push(error.message))
const open = (route, projectId = project.id, query = '') => page.goto(`${DASHBOARD}${route}?projectId=${projectId}${query}`)
const sidebarLink = (title) => page.locator('[data-sidebar="menu-button"]').filter({ has: page.getByText(title, { exact: true }) })
const dialog = () => page.getByRole('alertdialog').or(page.getByRole('dialog'))

async function chooseSignal(label, mode) {
  const row = dialog()
    .locator('div.flex.items-center.justify-between')
    .filter({ has: page.getByText(label, { exact: true }) })
  await row.locator('button').click()
  await page.getByRole('option', { name: mode, exact: true }).click()
}
async function saveRule() {
  await dialog().getByRole('button', { name: 'New Rule', exact: true }).click()
  await dialog().waitFor({ state: 'detached' })
}

try {
  await check('sign in', async () => {
    await page.goto(`${DASHBOARD}/login`)
    await page.locator('#email').fill(EMAIL)
    await page.locator('#password').fill(PASSWORD)
    await page.getByRole('button', { name: 'Login', exact: true }).click()
    await page.waitForURL((url) => !url.pathname.startsWith('/login'))
  })

  await check('a browser project has Data Collection and Dashboards in the sidebar', async () => {
    await open('/issues')
    await sidebarLink('Data Collection').waitFor()
    await sidebarLink('Dashboards').waitFor()
    assert.equal(await sidebarLink('Data Collection').locator('svg.lucide-upload').count(), 1, 'the upload icon')
    await shot(page, 'sidebar-browser-project')
  })

  await check('the Edit Project panel no longer has a Data Collection tab', async () => {
    await page.getByLabel('Switch organization or project').click()
    await page.getByRole('menuitem', { name: 'Edit Project' }).click()
    await page.getByRole('tab', { name: 'Danger Zone' }).waitFor()
    const tabs = await page.getByRole('tab').allTextContents()
    assert.deepEqual(
      tabs.map((tab) => tab.trim()),
      ['Project', 'Healthchecks', 'Profiles', 'AI', 'Danger Zone']
    )
    await shot(page, 'edit-project-without-data-collection')
    await page.keyboard.press('Escape')
  })

  await check('the Data Collection page opens from the sidebar and starts empty', async () => {
    await sidebarLink('Data Collection').click()
    await page.waitForURL(/\/data-collection/)
    await page.getByRole('heading', { name: 'Data Collection', level: 1 }).waitFor()
    await page.getByText('No rules: errors are sent').waitFor()
    await shot(page, 'data-collection-empty')
  })

  await check('a rule streams route timing for every client', async () => {
    await page.getByRole('button', { name: 'New Rule' }).click()
    await page.locator('#sdk-rule-name').fill('Route timing for everyone')
    await chooseSignal('Route timing', 'Stream')
    await shot(page, 'rule-route-timing-for-everyone')
    await saveRule()
    await page.getByText('Route timing for everyone').waitFor()
  })

  await check('a rule streams long tasks for one user', async () => {
    await page.getByRole('button', { name: 'New Rule' }).click()
    await page.locator('#sdk-rule-name').fill('Long tasks for Mia')
    await dialog().getByRole('button', { name: 'Add condition' }).click()
    await dialog().getByPlaceholder('user.id').fill('user.id')
    await dialog().getByPlaceholder('value', { exact: true }).fill(TARGETED.email)
    await chooseSignal('Long tasks', 'Stream')
    await shot(page, 'rule-long-tasks-for-one-user')
    await saveRule()
    await page.getByText('Long tasks for Mia').waitFor()
  })

  await check('a trigger reacts to a frozen page on any client', async () => {
    await page.getByRole('button', { name: 'New Rule' }).click()
    await page.locator('#sdk-rule-name').fill('Capture frozen pages')
    await dialog().getByRole('button', { name: /Edit as JSON/ }).click()
    await dialog().getByRole('button', { name: 'Frozen page → capture' }).click()
    await dialog().getByText('frozen-page').first().waitFor()
    await shot(page, 'rule-frozen-page-trigger')
    await saveRule()
    await page.getByText('Capture frozen pages').waitFor()
    await shot(page, 'data-collection-rules')
  })

  await check('the server hands each client the settings its rules select', async () => {
    const rules = (await api(token, 'GET', '/sdk-config/rules', undefined, project.id)).data
    assert.equal(rules.length, 3)
    const preview = (attributes) => api(token, 'POST', '/sdk-config/preview', { attributes }, project.id)
    const mia = await preview({ 'user.id': TARGETED.email })
    const liam = await preview({ 'user.id': OTHERS[0].email })
    const settingsOf = (result) => result.settings
    assert.equal(settingsOf(mia).signals.longTasks.mode, 'stream')
    assert.equal(settingsOf(mia).signals.routeTiming.mode, 'stream')
    assert.equal(settingsOf(liam).signals.longTasks, undefined)
    assert.equal(settingsOf(liam).signals.routeTiming.mode, 'stream')
    assert.equal(settingsOf(liam).triggers[0].id, 'frozen-page')
  })

  await check('a backend project has no Data Collection entry', async () => {
    await open('/issues', backendProject.id)
    await sidebarLink('Dashboards').waitFor()
    assert.equal(await sidebarLink('Data Collection').count(), 0)
    await open('/data-collection', backendProject.id)
    await page.getByText('Remote collection settings are available for browser and Flutter projects').waitFor()
    await shot(page, 'data-collection-backend-project')
  })

  const sent = new Map()
  await check('the storefront sends what its rules allow, per user', async () => {
    // The dashboard's tabs stay out of the way: on a small machine the shop's renderer is otherwise killed.
    await page.goto('about:blank')
    const shopBrowser = await chromium.launch({ headless: process.env.HEADED !== '1' })
    const journey = async (persona) => {
      const context = await shopBrowser.newContext({ viewport: { width: 1280, height: 800 } })
      await context.addInitScript((user) => {
        window.__DEMO_USER = user
      }, persona)
      const shop = await context.newPage()
      const events = []
      sent.set(persona.email, events)
      shop.on('request', (request) => {
        if (!request.url().endsWith('/api/otel/v1/logs') || request.method() !== 'POST') return
        try {
          for (const resource of JSON.parse(request.postData() ?? '{}').resourceLogs ?? [])
            for (const scope of resource.scopeLogs ?? [])
              for (const record of scope.logRecords ?? []) events.push(record.eventName)
        } catch {
          // not a JSON body
        }
      })
      // A block inside a task of the page: the browser does not time a script DevTools evaluates.
      const block = (ms) =>
        shop.evaluate(
          (ms) =>
            new Promise((resolve) =>
              setTimeout(() => {
                const until = performance.now() + ms
                while (performance.now() < until);
                resolve()
              }, 0)
            ),
          ms
        )
      const banner = shop.getByRole('banner')
      await shop.goto(SHOP)
      await shop.getByRole('button', { name: 'Add to cart' }).first().waitFor({ timeout: 30_000 })
      await shop.waitForTimeout(1500)
      await shop.getByRole('button', { name: 'Add to cart' }).first().click()
      await block(300)
      await banner.getByRole('button', { name: /^Cart, \d+ items$/ }).click()
      await shop.waitForTimeout(2500)
      await block(persona === TARGETED ? 300 : 1200)
      await banner.getByRole('button', { name: 'Checkout', exact: true }).click()
      await shop.waitForTimeout(2500)
      await banner.getByRole('button', { name: 'Shop', exact: true }).click()
      await shop.waitForTimeout(5000)
      if (persona === TARGETED) await shot(shop, 'storefront')
      // Web vitals such as LCP and INP are final only once the page is hidden.
      await shop.evaluate(() => {
        Object.defineProperty(document, 'visibilityState', { value: 'hidden', configurable: true })
        document.dispatchEvent(new Event('visibilitychange'))
      })
      await shop.waitForTimeout(3000)
      await context.close()
    }
    for (const persona of [TARGETED, ...OTHERS]) {
      try {
        await journey(persona)
      } catch (error) {
        if (!/crashed/i.test(String(error))) throw error
        console.log(`  ${persona.name}: the tab crashed, running the journey again`)
        await journey(persona)
      }
    }
    await shopBrowser.close()
    const count = (email, name) => sent.get(email).filter((event) => event === name).length
    for (const persona of [TARGETED, ...OTHERS]) {
      // The last route change may still be loading when the page is hidden, which discards its measurement.
      assert(count(persona.email, 'browser.route_timing') >= 2, `${persona.name}: route timing is on for everyone`)
      assert(count(persona.email, 'browser.web_vital') >= 1, `${persona.name}: web vitals are on by default`)
    }
    assert(count(TARGETED.email, 'browser.long_task') >= 2, 'the targeted user streams long tasks')
    for (const persona of OTHERS) {
      assert.equal(count(persona.email, 'traceway.trigger.fired'), 1, `${persona.name}: the 1.2 s freeze fires the trigger`)
      assert.equal(count(persona.email, 'traceway.capture.started'), 1, `${persona.name}: and starts a capture`)
    }
    assert.equal(count(TARGETED.email, 'traceway.trigger.fired'), 0, 'no freeze of a second, no trigger')
  })

  await check('the events are stored as metrics', async () => {
    await new Promise((resolve) => setTimeout(resolve, 3000))
    const now = new Date()
    const discovered = await api(
      token,
      'GET',
      `/metrics/discover?from=${new Date(now - 3_600_000).toISOString()}&to=${new Date(now.getTime() + 60_000).toISOString()}`,
      undefined,
      project.id
    )
    const names = discovered.metrics.map((metric) => metric.name)
    for (const name of ['browser.web_vital.lcp', 'browser.web_vital.fcp', 'browser.route_timing.duration', 'browser.long_task.duration'])
      assert(names.includes(name), `${name} is not among ${names.join(', ')}`)
  })

  await check('the Web Performance dashboard is one click away and charts the data', async () => {
    await open('/dashboards', project.id, '&preset=30m')
    const install = page.getByRole('button', { name: 'Add Web Performance Dashboard' })
    const tab = page.getByRole('tab', { name: /Web Performance/ })
    await install.or(tab).first().waitFor()
    if (await install.isVisible()) {
      await shot(page, 'dashboards-empty')
      await install.click()
    }
    await page.getByText('Largest Contentful Paint (ms)').waitFor()
    await page.getByText('Route Change Duration by Page (ms)').scrollIntoViewIfNeeded()
    await page.waitForTimeout(2500)
    await page.getByText('Largest Contentful Paint (ms)').scrollIntoViewIfNeeded()
    await page.waitForTimeout(500)
    await shot(page, 'web-performance-dashboard')
    await page.getByText('Route Change Duration (ms)', { exact: true }).scrollIntoViewIfNeeded()
    await page.waitForTimeout(1500)
    await shot(page, 'web-performance-dashboard-routes-and-long-tasks')
  })

  await check('a metric alert is built from a preset and fires', async () => {
    await api(
      token,
      'POST',
      '/notification-channels',
      { name: 'Local webhook', channelType: 'webhook', config: { url: webhookUrl, method: 'POST' }, enabled: true },
      project.id
    )
    await open('/notifications', project.id, '&tab=rules')
    await page.getByRole('button', { name: 'New Rule' }).click()
    await dialog()
      .locator('div.space-y-2')
      .filter({ has: page.getByText('Channel', { exact: true }) })
      .locator('button')
      .first()
      .click()
    await page.getByRole('option', { name: /Local webhook/ }).click()
    await page.getByRole('option', { name: /Local webhook/ }).waitFor({ state: 'detached' })
    await dialog()
      .locator('div.space-y-2')
      .filter({ has: page.getByText('Rule Type', { exact: true }) })
      .locator('button')
      .first()
      .click()
    await page.getByRole('option', { name: 'Metric Threshold' }).waitFor()
    const offered = await page.getByRole('option').allTextContents()
    assert.deepEqual(
      offered.map((option) => option.trim()),
      ['New Issue', 'Metric Threshold', 'SLO Burn Rate', 'SLO Budget Consumed']
    )
    await page.getByRole('option', { name: 'Metric Threshold' }).click()
    await dialog().getByRole('button', { name: 'Slow route change' }).click()
    assert.equal(await dialog().locator('#cfg-metric').inputValue(), 'browser.route_timing.duration')
    assert.equal(await dialog().locator('#rule-name').inputValue(), 'Slow route change')
    // The preset's threshold is three seconds; the demo shop is faster, so lower it to see the alert fire.
    await dialog().locator('#cfg-value').fill('50')
    await dialog().locator('#cfg-lookback-m').fill('60')
    await shot(page, 'alert-rule-from-preset')
    await dialog().getByRole('button', { name: 'New Rule', exact: true }).click()
    await dialog().waitFor({ state: 'detached' })
    await page.getByText('Slow route change').first().waitFor()
    await shot(page, 'alert-rules')

    const deadline = Date.now() + ALERT_WAIT_MS
    while (webhookCalls.length === 0 && Date.now() < deadline) await new Promise((resolve) => setTimeout(resolve, 2000))
    assert(webhookCalls.length > 0, `no alert reached the webhook within ${ALERT_WAIT_MS / 1000} s`)
    console.log(`  alert payload: ${webhookCalls[0].slice(0, 400)}`)
    assert(/route_timing|Slow route change/.test(webhookCalls.join('\n')))
    await open('/notifications', project.id, '&tab=history')
    await page.getByText('Slow route change').first().waitFor()
    await shot(page, 'alert-history')
  })

  assert.deepEqual(pageErrors, [], 'the dashboard threw no errors')
  console.log('Data collection end-to-end checks passed.')
} catch (error) {
  await page.screenshot({ path: path.join(OUT, 'failure.png') }).catch(() => {})
  throw error
} finally {
  await browser.close()
  await new Promise((resolve) => webhook.close(resolve))
}
