// Seeds Traceway platform entities for the Shoply demo account.
// Auth resolution order: TRACEWAY_DEMO_PAT env, TRACEWAY_DEMO_EMAIL+TRACEWAY_DEMO_PASSWORD env,
// then the traceway CLI's stored "demo" profile token (~/.local/state/traceway/state.json).
// Re-runnable: every create is guarded by a list-lookup first.

import { readFileSync, writeFileSync, existsSync } from 'node:fs'
import { homedir } from 'node:os'
import { join, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'

const BASE = process.env.TRACEWAY_BASE_URL || 'https://cloud.tracewayapp.com'
const REACT_TOKEN = process.env.SHOPLY_REACT_TOKEN || '17b6181efa3b46639519faadd77245c3'
const OTEL_TOKEN = process.env.SHOPLY_OTEL_TOKEN || '74b1342ebb974f6c85530b90ab0828e2'
const CLI_PROFILE = process.env.TRACEWAY_CLI_PROFILE || 'demo'
const MINT_SOURCEMAP_TOKEN = process.env.MINT_SOURCEMAP_TOKEN === '1'

let authToken = null
const summary = []
const note = (kind, name, detail = '') => {
  summary.push({ kind, name, detail })
  console.log(`  [${kind}] ${name}${detail ? ' — ' + detail : ''}`)
}

async function api(method, path, body, { allow = [] } = {}) {
  const res = await fetch(BASE + path, {
    method,
    headers: {
      Authorization: `Bearer ${authToken}`,
      ...(body !== undefined ? { 'Content-Type': 'application/json' } : {})
    },
    body: body !== undefined ? JSON.stringify(body) : undefined
  })
  let data = null
  const text = await res.text()
  try { data = text ? JSON.parse(text) : null } catch { data = text }
  if (!res.ok && !allow.includes(res.status)) {
    throw new Error(`${method} ${path} -> ${res.status}: ${typeof data === 'object' ? JSON.stringify(data) : data}`)
  }
  return { status: res.status, data }
}

function cliToken() {
  const statePath = join(homedir(), '.local', 'state', 'traceway', 'state.json')
  if (!existsSync(statePath)) return null
  try {
    const state = JSON.parse(readFileSync(statePath, 'utf8'))
    return state.profiles?.[CLI_PROFILE]?.jwt || null
  } catch {
    return null
  }
}

async function resolveAuth() {
  if (process.env.TRACEWAY_DEMO_PAT) {
    authToken = process.env.TRACEWAY_DEMO_PAT
    return
  }
  if (process.env.TRACEWAY_DEMO_EMAIL && process.env.TRACEWAY_DEMO_PASSWORD) {
    const res = await fetch(BASE + '/api/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: process.env.TRACEWAY_DEMO_EMAIL, password: process.env.TRACEWAY_DEMO_PASSWORD })
    })
    if (!res.ok) throw new Error(`login failed: ${res.status}`)
    authToken = (await res.json()).token
    return
  }
  const jwt = cliToken()
  if (!jwt) throw new Error(`No auth: set TRACEWAY_DEMO_PAT, or TRACEWAY_DEMO_EMAIL/PASSWORD, or run: traceway login --profile ${CLI_PROFILE}`)
  authToken = jwt
  // Device-flow JWTs expire in 15 minutes; trade it for a durable PAT immediately.
  const { data: pats } = await api('GET', '/api/personal-access-tokens')
  const existing = (Array.isArray(pats) ? pats : pats?.tokens || []).find?.((p) => p.name === 'demo-seed')
  if (!existing) {
    const { data } = await api('POST', '/api/personal-access-tokens', { name: 'demo-seed' })
    if (data?.token) {
      authToken = data.token
      note('auth', 'PAT demo-seed', 'minted (stored only in memory this run)')
      return
    }
  }
  note('auth', 'CLI JWT', 'using short-lived device token (PAT demo-seed already exists; delete it or pass TRACEWAY_DEMO_PAT for long runs)')
}

async function main() {
  console.log(`Seeding demo entities on ${BASE}`)
  await resolveAuth()

  // ---- Context -------------------------------------------------------------
  const { data: bundle } = await api('GET', '/api/me/login-bundle')
  const userId = bundle.user.id
  const { data: projects } = await api('GET', '/api/projects')
  const reactProject = projects.find((p) => p.token === REACT_TOKEN)
  const otelProject = projects.find((p) => p.token === OTEL_TOKEN)
  if (!reactProject || !otelProject) {
    const names = projects.map((p) => `${p.name} (${p.framework})`).join(', ')
    throw new Error(`Could not match demo project tokens. Visible projects: ${names}`)
  }
  const orgId = otelProject.organizationId
  const REACT_ID = reactProject.id
  const OTEL_ID = otelProject.id
  const org = bundle.organizations.find((o) => o.id === orgId)
  if (org && !['owner', 'admin'].includes(org.role)) {
    throw new Error(`Org role is '${org.role}' — owner/admin needed for on-call/status-page seeding`)
  }
  note('context', 'projects', `react=${REACT_ID} otel=${OTEL_ID} org=${orgId} user=${userId}`)
  const demoEmail = bundle.user.email

  const pq = (id) => `?projectId=${id}`

  // ---- Healthy monitors first (they need cycles for uptime history) --------
  const { data: checksResp } = await api('GET', `/api/synthetics/checks${pq(OTEL_ID)}`)
  const existingChecks = checksResp?.checks || []
  const checkByName = (n) => existingChecks.find((c) => c.name === n)

  async function ensureCheck(body) {
    const existing = checkByName(body.name)
    if (existing) {
      note('monitor', body.name, `exists (id ${existing.id})`)
      return existing.id
    }
    const { status, data } = await api('POST', `/api/synthetics/checks${pq(OTEL_ID)}`, body, { allow: [422] })
    if (status === 422) {
      note('monitor', body.name, `SKIPPED 422: ${data?.error}`)
      return null
    }
    const id = data?.check?.id ?? data?.id
    note('monitor', body.name, `created (id ${id})`)
    await api('POST', `/api/synthetics/checks/${id}/run${pq(OTEL_ID)}`, {}, { allow: [422] })
    return id
  }

  const httpCheck = (name, url, intervalSeconds, assertions, extra = {}) => ({
    name, checkType: 'http', enabled: true, intervalSeconds, timeoutSeconds: 10, failureThreshold: 2,
    config: {
      url, method: 'GET', headers: {}, auth: { type: 'none' },
      followRedirects: true, skipTlsVerify: false,
      assertions: { statusCodes: [200], bodyContains: '', bodyRegex: '', maxResponseTimeMs: 0, tlsMinDaysRemaining: 0, ...assertions },
      ...extra
    }
  })

  const apiCheckId = await ensureCheck(httpCheck('Traceway API', `${BASE}/api/health`, 60, { maxResponseTimeMs: 2000, tlsMinDaysRemaining: 14 }))
  const mktCheckId = await ensureCheck(httpCheck('Marketing Site', 'https://example.com', 120, {}))
  const tcpCheckId = await ensureCheck({
    name: 'Edge TLS', checkType: 'tcp', enabled: true, intervalSeconds: 120, timeoutSeconds: 10, failureThreshold: 2,
    config: { host: 'cloud.tracewayapp.com', port: 443, useTls: true, sendPayload: '', expectContains: '', tlsMinDaysRemaining: 14 }
  })

  // ---- On-call: team -> schedule -> escalation policy ----------------------
  const { data: teams } = await api('GET', `/api/organizations/${orgId}/teams`)
  let team = (teams || []).find((t) => t.name === 'Shoply Engineering')
  if (!team) {
    const { data } = await api('POST', `/api/organizations/${orgId}/teams`, {
      name: 'Shoply Engineering',
      description: 'Owns the Shoply storefront and API',
      memberUserIds: [userId],
      projectIds: [REACT_ID, OTEL_ID]
    })
    team = data?.team ?? data
    note('team', 'Shoply Engineering', `created (id ${team.id})`)
  } else {
    note('team', 'Shoply Engineering', `exists (id ${team.id})`)
  }

  const { data: schedules } = await api('GET', `/api/organizations/${orgId}/schedules`)
  let schedule = (schedules || []).find((s) => s.name === 'Shoply Primary On-Call')
  if (!schedule) {
    const { data } = await api('POST', `/api/organizations/${orgId}/schedules`, {
      teamId: team.id,
      name: 'Shoply Primary On-Call',
      description: 'Weekly rotation for the Shoply services',
      timezone: 'UTC',
      definition: {
        schemaVersion: 1,
        layers: [{
          name: 'Primary',
          rotationType: 'weekly',
          handoffTime: '09:00',
          handoffDay: 1,
          intervalDays: 0,
          rotationStart: '2026-08-18',
          userIds: [userId],
          restrictions: []
        }]
      }
    })
    schedule = data?.schedule ?? data
    note('schedule', 'Shoply Primary On-Call', `created (id ${schedule.id})`)
  } else {
    note('schedule', 'Shoply Primary On-Call', `exists (id ${schedule.id})`)
  }

  const { data: policies } = await api('GET', `/api/organizations/${orgId}/escalation-policies`)
  let policy = (policies || []).find((p) => p.name === 'Shoply Critical')
  if (!policy) {
    const { data } = await api('POST', `/api/organizations/${orgId}/escalation-policies`, {
      name: 'Shoply Critical',
      definition: {
        schemaVersion: 1,
        urgency: 'auto',
        repeatCount: 1,
        steps: [
          { delayMinutes: 5, targets: [{ type: 'schedule', id: schedule.id }] },
          { delayMinutes: 10, targets: [{ type: 'user', id: userId }] }
        ]
      }
    })
    policy = data?.policy ?? data
    note('policy', 'Shoply Critical', `created (id ${policy.id})`)
  } else {
    note('policy', 'Shoply Critical', `exists (id ${policy.id})`)
  }

  // ---- Notification channels & rules ---------------------------------------
  async function ensureChannel(projectId, body) {
    const { data } = await api('GET', `/api/notification-channels${pq(projectId)}`)
    const list = Array.isArray(data) ? data : data?.channels || []
    const existing = list.find((c) => c.name === body.name)
    if (existing) {
      note('channel', body.name, `exists (id ${existing.id})`)
      return existing.id
    }
    const { data: created } = await api('POST', `/api/notification-channels${pq(projectId)}`, body)
    const id = created?.channel?.id ?? created?.id
    note('channel', body.name, `created (id ${id})`)
    return id
  }

  async function ensureRule(projectId, body) {
    const { data } = await api('GET', `/api/notification-rules${pq(projectId)}`)
    const list = Array.isArray(data) ? data : data?.rules || []
    const existing = list.find((r) => r.name === body.name)
    if (existing) {
      note('rule', body.name, `exists (id ${existing.id})`)
      return existing.id
    }
    const { data: created } = await api('POST', `/api/notification-rules${pq(projectId)}`, body)
    const id = created?.rule?.id ?? created?.id
    note('rule', body.name, `created (id ${id})`)
    return id
  }

  const emailChId = await ensureChannel(OTEL_ID, {
    name: 'Shoply Alerts (email)', channelType: 'email', config: { recipients: [demoEmail] }
  })
  const escChId = await ensureChannel(OTEL_ID, {
    name: 'Page Shoply On-Call', channelType: 'escalation', config: { policyId: policy.id }
  })
  const reactEmailChId = await ensureChannel(REACT_ID, {
    name: 'Shoply Alerts (email)', channelType: 'email', config: { recipients: [demoEmail] }
  })

  await ensureRule(OTEL_ID, { channelId: escChId, name: 'Monitor down → page on-call', ruleType: 'check_down', config: {}, cooldownMinutes: 15, severity: 'critical' })
  await ensureRule(OTEL_ID, { channelId: emailChId, name: 'New error', ruleType: 'new_error', config: { ignorePatterns: [] }, cooldownMinutes: 15, severity: 'warning' })
  await ensureRule(OTEL_ID, { channelId: emailChId, name: 'GET /api/products p95 > 400ms', ruleType: 'endpoint_p95_threshold', config: { endpoint: 'GET /api/products', thresholdMs: 400, lookbackMinutes: 15 }, cooldownMinutes: 30, severity: 'warning' })
  await ensureRule(OTEL_ID, { channelId: emailChId, name: 'Error rate > 5%', ruleType: 'error_rate_threshold', config: { thresholdPercent: 5, lookbackMinutes: 15, minRequests: 20 }, cooldownMinutes: 30, severity: 'critical' })
  await ensureRule(REACT_ID, { channelId: reactEmailChId, name: 'New error', ruleType: 'new_error', config: { ignorePatterns: [] }, cooldownMinutes: 15, severity: 'warning' })

  // ---- The intentionally failing check (after check_down exists) -----------
  const failCheckId = await ensureCheck(httpCheck('Payments Provider', `${BASE}/api/health`, 60, { bodyContains: 'payments-provider-ok' }))

  // ---- Status page + backdated incident + post-mortem ----------------------
  const checkIds = [apiCheckId, mktCheckId, tcpCheckId, failCheckId].filter(Boolean)
  const { data: pages } = await api('GET', `/api/organizations/${orgId}/status-pages`)
  let statusPage = (pages || []).find((p) => p.name === 'Shoply Status')
  if (!statusPage) {
    let slug = 'shoply-status'
    let res = await api('POST', `/api/organizations/${orgId}/status-pages`, {
      name: 'Shoply Status', slug, isPublic: true, checkIds,
      description: 'Live status for the Shoply storefront and API'
    }, { allow: [422] })
    if (res.status === 422) {
      slug = `shoply-status-${Math.random().toString(16).slice(2, 6)}`
      res = await api('POST', `/api/organizations/${orgId}/status-pages`, {
        name: 'Shoply Status', slug, isPublic: true, checkIds,
        description: 'Live status for the Shoply storefront and API'
      })
    }
    statusPage = res.data?.statusPage ?? res.data
    note('status-page', 'Shoply Status', `created (/status/${statusPage.slug})`)
  } else {
    note('status-page', 'Shoply Status', `exists (/status/${statusPage.slug})`)
  }

  const { data: incResp } = await api('GET', `/api/organizations/${orgId}/status-pages/${statusPage.id}/incidents?page=1&pageSize=50`, undefined, { allow: [404] })
  const incidents = incResp?.data || incResp?.incidents || []
  let incident = incidents.find((i) => i.title === 'Elevated checkout error rate')
  if (!incident) {
    const startedAt = new Date(Date.now() - 105 * 60 * 1000).toISOString()
    const { data } = await api('POST', `/api/organizations/${orgId}/status-pages/${statusPage.id}/incidents`, {
      title: 'Elevated checkout error rate',
      startedAt,
      message: 'We are investigating elevated error rates on checkout.'
    })
    incident = data?.incident ?? data
    for (const upd of [
      { status: 'identified', message: 'A bad deploy of the coupon cache is causing checkout requests with coupons to fail. Rolling back now.' },
      { status: 'monitoring', message: 'Rollback complete. Checkout error rate is recovering; we are monitoring.' },
      { status: 'resolved', message: 'Error rate has been back to baseline for 30 minutes. Incident resolved.' }
    ]) {
      await api('POST', `/api/organizations/${orgId}/incidents/${incident.id}/updates`, upd)
    }
    note('incident', 'Elevated checkout error rate', `created (id ${incident.id}, resolved with timeline)`)
  } else {
    note('incident', 'Elevated checkout error rate', `exists (id ${incident.id})`)
  }

  const { data: pmResp } = await api('GET', `/api/post-mortems${pq(OTEL_ID)}&page=1&pageSize=50`)
  const postMortems = pmResp?.data || []
  let pm = postMortems.find((p) => p.title === '2026-08-20 — Checkout coupon outage')
  if (!pm) {
    const { status, data } = await api('POST', `/api/post-mortems${pq(OTEL_ID)}`, {
      title: '2026-08-20 — Checkout coupon outage',
      contentMd: POST_MORTEM_MD,
      tags: ['checkout', 'coupon', 'panic'],
      incidentId: incident?.id
    }, { allow: [422] })
    if (status === 422) {
      note('post-mortem', 'Checkout coupon outage', `SKIPPED 422: ${data?.error}`)
    } else {
      pm = data?.postMortem ?? data
      note('post-mortem', 'Checkout coupon outage', `created (id ${pm?.id})`)
    }
  } else {
    note('post-mortem', 'Checkout coupon outage', `exists (id ${pm.id})`)
  }

  // ---- Dashboards ----------------------------------------------------------
  const { data: dashResp } = await api('GET', `/api/dashboards${pq(OTEL_ID)}`)
  const dashboards = dashResp?.dashboards || (Array.isArray(dashResp) ? dashResp : [])
  const hasTemplate = (key) => dashboards.some((d) => d.templateKey === key)

  const pdRes = await api('POST', `/api/dashboards/populate-defaults${pq(OTEL_ID)}`, {}, { allow: [422] })
  note('dashboards', 'populate-defaults', pdRes.status === 422 ? `skipped: ${pdRes.data?.error}` : 'installed')

  if (!hasTemplate('golang')) {
    const res = await api('POST', `/api/dashboard-templates/golang/install`, { organizationId: orgId, projectIds: [OTEL_ID] }, { allow: [404, 422] })
    note('dashboards', 'golang template', res.status >= 400 ? `skipped ${res.status}` : 'installed')
  } else {
    note('dashboards', 'golang template', 'exists')
  }

  if (!dashboards.some((d) => d.name === 'Shoply Business')) {
    const res = await api('POST', '/api/dashboards', {
      organizationId: orgId,
      name: 'Shoply Business',
      description: 'Orders, revenue and coupon health',
      applyToProjectIds: [OTEL_ID],
      definition: {
        schemaVersion: 1,
        widgets: [
          { title: 'Orders Placed', widgetType: 'bar_chart', config: { sources: [{ type: 'metric', name: 'shop.orders.placed', aggregation: 'sum' }], unit: 'orders' } },
          { title: 'Revenue (USD)', widgetType: 'area_chart', config: { sources: [{ type: 'metric', name: 'shop.revenue', aggregation: 'sum' }], unit: '$' } },
          { title: 'Avg Order Value', widgetType: 'single_value', config: { sources: [{ type: 'metric', name: 'shop.checkout.value.avg', aggregation: 'avg' }], showSparkline: true, unit: '$' } },
          { title: 'Coupon Failures', widgetType: 'line_chart', config: { sources: [{ type: 'metric', name: 'shop.coupon.failures', aggregation: 'sum' }] } },
          { title: 'Payments Declined', widgetType: 'line_chart', config: { sources: [{ type: 'metric', name: 'shop.payments.declined', aggregation: 'sum' }] } },
          { title: 'Items In Carts', widgetType: 'line_chart', config: { sources: [{ type: 'metric', name: 'shop.cart.items', aggregation: 'avg' }] } }
        ]
      }
    }, { allow: [422] })
    note('dashboards', 'Shoply Business', res.status === 422 ? `skipped: ${res.data?.error}` : 'created')
  } else {
    note('dashboards', 'Shoply Business', 'exists')
  }

  // ---- Source map token (rotates on every mint — opt-in) -------------------
  if (MINT_SOURCEMAP_TOKEN) {
    const { data } = await api('POST', `/api/projects/source-map-token${pq(REACT_ID)}`)
    const token = data?.sourceMapToken ?? data?.token
    if (token) {
      const envPath = join(dirname(fileURLToPath(import.meta.url)), '..', 'frontend', '.env')
      const lines = [
        'VITE_API_BASE=/api',
        `VITE_TW_CONNECTION=${REACT_TOKEN}@${BASE}/api/report`,
        `TRACEWAY_URL=${BASE}`,
        `TRACEWAY_SOURCEMAP_TOKEN=${token}`,
        ''
      ]
      writeFileSync(envPath, lines.join('\n'))
      note('sourcemap', 'react project token', `minted and written to frontend/.env (previous token rotated out)`)
    }
  } else {
    note('sourcemap', 'react project token', 'skipped (set MINT_SOURCEMAP_TOKEN=1 to mint + write frontend/.env)')
  }

  // ---- Summary -------------------------------------------------------------
  console.log('\n=== Seed complete ===')
  console.log(`Status page: ${BASE.replace('/api', '')}/status/${statusPage.slug}`)
  console.log(`Projects: react=${REACT_ID}  otel=${OTEL_ID}  org=${orgId}`)
  console.log('The "Payments Provider" monitor is intentionally failing; expect an incident + page within ~3 minutes.')
}

const POST_MORTEM_MD = `## Summary

Between 13:05 and 14:50 UTC, checkout requests that applied a coupon failed with HTTP 500. Roughly 60% of checkout attempts in that window were affected. The root cause was a nil coupon-hit cache map introduced in the 13:00 deploy: the first coupon application after boot panicked the handler, and every subsequent coupon apply followed the same path.

## Impact

- Checkout conversion dropped ~40% during the window.
- 214 coupon applications returned "internal error".
- No orders were lost permanently; affected customers retried after the rollback.

## Timeline (UTC)

- **13:00** — Deploy of coupon cache refactor goes out.
- **13:05** — First \`assignment to entry in nil map\` panic hits the issues feed; error-rate alert fires.
- **13:12** — On-call acknowledges the page; correlates the panic with the deploy via the release tag.
- **13:41** — Root cause identified: the cache map is declared but never initialized after the refactor.
- **13:55** — Rollback deployed; error rate begins recovering.
- **14:50** — Error rate back at baseline for 30 minutes; incident resolved.

## Root cause

The refactor moved coupon-hit tracking to a package-level \`map[string]int\` and removed the \`make()\` call from the initializer. Writes to a nil map panic in Go. Unit tests only exercised the read path, which returns the zero value on a nil map and passes.

## What went well

- Alerting fired within 5 minutes of the first failure.
- The distributed trace on the frontend exception pointed directly at the failing backend span.

## Action items

- [ ] Add a write-path unit test for the coupon cache.
- [ ] Add a lint rule for package-level map declarations without initialization.
- [ ] Add a canary stage for checkout-path deploys.
`

main().catch((e) => {
  console.error('SEED FAILED:', e.message)
  process.exit(1)
})
