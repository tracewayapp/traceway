import { chromium } from 'playwright'
import { PERSONAS, VIEWPORTS, pick } from './personas.mjs'
import { pickJourney } from './journeys.mjs'
import { httpBurst, aiConversation, CHAT_SCRIPTS } from './http-load.mjs'

const args = process.argv.slice(2)
const flag = (name) => args.includes(name)
const opt = (name, fallback) => {
  const i = args.indexOf(name)
  return i >= 0 && args[i + 1] ? args[i + 1] : fallback
}

const BASE = opt('--base', 'http://localhost:8090')
const SESSIONS = parseInt(opt('--sessions', '50'), 10)
const PARALLEL = parseInt(opt('--parallel', '3'), 10)
const HEADED = flag('--headed')
const DRIP = flag('--drip')
const NO_BROWSER = flag('--no-browser')

const sleep = (ms) => new Promise((r) => setTimeout(r, ms))
const rand = (min, max) => min + Math.floor(Math.random() * (max - min))

// The backend cart is global (no session scoping), so cart-mutating journeys
// must not overlap or they steal each other's carts mid-checkout.
let cartLock = Promise.resolve()
function withCartLock(fn) {
  const run = cartLock.then(fn, fn)
  cartLock = run.catch(() => {})
  return run
}

let sessionCounter = 0

async function runSession(browser) {
  const id = ++sessionCounter
  const persona = pick(PERSONAS)
  const viewport = pick(VIEWPORTS)
  const journey = pickJourney()

  const context = await browser.newContext({ viewport })
  await context.addInitScript((user) => {
    window.__DEMO_USER = user
  }, { name: persona.name, email: persona.email, plan: persona.plan })

  const page = await context.newPage()
  const label = `#${id} ${journey.name} (${persona.name}, ${viewport.width}x${viewport.height})`
  console.log(`[session] start ${label}`)
  try {
    await page.goto(BASE, { waitUntil: 'domcontentloaded' })
    const exec = () => journey.run(page, persona, { base: BASE })
    if (journey.cart) {
      await withCartLock(exec)
    } else {
      await exec()
    }
    // Idle tail so the session-recording segment covering the journey uploads.
    await page.waitForTimeout(rand(5000, 9000))
    await page.close({ runBeforeUnload: true })
    await sleep(1500)
    console.log(`[session] done  ${label}`)
  } catch (e) {
    console.log(`[session] FAIL  ${label}: ${e.message?.split('\n')[0]}`)
  } finally {
    await context.close().catch(() => {})
  }
}

async function seedMode() {
  console.log(`Seeding ${SESSIONS} sessions against ${BASE} (parallel ${PARALLEL})`)

  const sideWork = (async () => {
    const chats = [...CHAT_SCRIPTS, ...CHAT_SCRIPTS].slice(0, 7)
    for (const script of chats) {
      await aiConversation(BASE, { script })
      await sleep(rand(5000, 20000))
    }
  })()
  const load = (async () => {
    for (let i = 0; i < 16; i++) {
      await httpBurst(BASE, { requests: 25 })
      await sleep(rand(2000, 8000))
    }
  })()

  if (!NO_BROWSER) {
    const browser = await chromium.launch({ headless: !HEADED })
    let launched = 0
    const workers = Array.from({ length: PARALLEL }, async () => {
      while (launched < SESSIONS) {
        launched++
        await runSession(browser)
        await sleep(rand(1000, 4000))
      }
    })
    await Promise.all(workers)
    await browser.close()
  }

  await Promise.all([sideWork, load])
  console.log('Seed run complete.')
}

async function dripMode() {
  console.log(`Drip mode against ${BASE} — Ctrl-C to stop.`)
  const browser = NO_BROWSER ? null : await chromium.launch({ headless: !HEADED })

  const browserLoop = (async () => {
    while (true) {
      if (browser) await runSession(browser)
      await sleep(rand(120000, 240000))
    }
  })()
  const httpLoop = (async () => {
    while (true) {
      await httpBurst(BASE, { requests: rand(8, 18) })
      await sleep(rand(30000, 60000))
    }
  })()
  const aiLoop = (async () => {
    while (true) {
      await aiConversation(BASE)
      await sleep(rand(480000, 780000))
    }
  })()

  await Promise.all([browserLoop, httpLoop, aiLoop])
}

if (DRIP) {
  await dripMode()
} else {
  await seedMode()
}
