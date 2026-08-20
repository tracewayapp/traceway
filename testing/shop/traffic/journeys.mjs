import { pick } from './personas.mjs'

const CATEGORIES = ['Apparel', 'Electronics', 'Home', 'Books']

function rand(min, max) {
  return min + Math.floor(Math.random() * (max - min))
}

async function think(page, persona, minMs = 600, maxMs = 3000) {
  await page.waitForTimeout(Math.floor(rand(minMs, maxMs) * persona.thinkScale))
}

async function goToShop(page) {
  await page.getByRole('banner').getByRole('button', { name: 'Shop', exact: true }).click()
}

async function goToCart(page) {
  await page.getByRole('banner').getByRole('button', { name: /^Cart, \d+ items$/ }).click()
}

async function goToCheckout(page) {
  await page.getByRole('banner').getByRole('button', { name: 'Checkout', exact: true }).click()
}

async function waitForProducts(page) {
  await page.getByRole('button', { name: 'Add to cart' }).first().waitFor({ timeout: 20000 })
}

async function browseAround(page, persona, { categoryHops = 1, hovers = 2 } = {}) {
  await waitForProducts(page)
  for (let i = 0; i < categoryHops; i++) {
    await page.getByRole('button', { name: pick(CATEGORIES), exact: true }).click()
    await think(page, persona, 800, 2500)
  }
  const cards = page.locator('div.group')
  const count = await cards.count()
  for (let i = 0; i < hovers && count > 0; i++) {
    await cards.nth(rand(0, count)).hover()
    await think(page, persona, 500, 1500)
  }
  await page.mouse.wheel(0, rand(300, 1200))
  await think(page, persona, 500, 1500)
}

async function addRandomItems(page, persona, n) {
  await page.getByRole('button', { name: 'All', exact: true }).click()
  await waitForProducts(page)
  for (let i = 0; i < n; i++) {
    const buttons = page.getByRole('button', { name: 'Add to cart' })
    const count = await buttons.count()
    const target = buttons.nth(rand(0, count))
    await target.scrollIntoViewIfNeeded()
    await target.click()
    // Add-to-cart is slow-path ~75% of the time; wait for the button to settle.
    await page.getByRole('button', { name: 'Adding…' }).first().waitFor({ timeout: 2000 }).catch(() => {})
    await page.getByRole('button', { name: 'Adding…' }).first().waitFor({ state: 'detached', timeout: 15000 }).catch(() => {})
    await think(page, persona, 700, 2200)
  }
}

async function fillCheckoutForm(page, persona) {
  await page.locator('#co-name').fill(persona.name)
  await page.locator('#co-email').fill(persona.email)
  await page.locator('#co-card').fill(persona.card)
}

async function applyCouponUI(page, persona, code) {
  await page.getByLabel('Coupon code').fill(code)
  await page.getByRole('button', { name: 'Apply', exact: true }).click()
  await think(page, persona, 1200, 2500)
}

async function placeOrder(page, persona, { retryOnDecline = true } = {}) {
  await page.getByRole('button', { name: 'Place order' }).click()
  const confirmed = page.getByRole('heading', { name: 'Order confirmed' })
  try {
    await confirmed.waitFor({ timeout: 20000 })
    await think(page, persona, 1500, 3000)
    await page.getByRole('button', { name: 'Back to shop' }).click()
    return true
  } catch {
    if (retryOnDecline) {
      const errText = await page.locator('text=declined').count().catch(() => 0)
      if (errText > 0) {
        await think(page, persona, 1000, 2500)
        return placeOrder(page, persona, { retryOnDecline: false })
      }
    }
    return false
  }
}

export const journeys = {
  happyPath: {
    weight: 30, cart: true,
    async run(page, persona) {
      await browseAround(page, persona, { categoryHops: rand(1, 3), hovers: rand(1, 4) })
      await addRandomItems(page, persona, rand(1, 4))
      await goToCart(page)
      await think(page, persona, 1200, 3000)
      await page.getByRole('button', { name: 'Proceed to checkout' }).click()
      await think(page, persona, 800, 2000)
      if (Math.random() < 0.5) {
        await applyCouponUI(page, persona, pick(['SAVE10', 'HALF50', 'SAVE10']))
      }
      await fillCheckoutForm(page, persona)
      await think(page, persona, 800, 1800)
      await placeOrder(page, persona)
    }
  },

  couponExpired: {
    weight: 10, cart: true,
    async run(page, persona) {
      await browseAround(page, persona, { categoryHops: 1, hovers: 1 })
      await addRandomItems(page, persona, 1)
      await goToCheckout(page)
      await applyCouponUI(page, persona, 'EXPIRED')
      await applyCouponUI(page, persona, 'SAVE10')
      await fillCheckoutForm(page, persona)
      await placeOrder(page, persona)
    }
  },

  emptyCartCheckout: {
    weight: 8, cart: true,
    async run(page, persona, { base }) {
      // Ensure the (global) cart really is empty before demonstrating the bug.
      const cart = await fetch(base + '/api/cart').then((r) => r.json()).catch(() => null)
      for (const item of cart?.items || []) {
        await fetch(`${base}/api/cart/${item.id}`, { method: 'DELETE' }).catch(() => {})
      }
      await goToCheckout(page)
      await think(page, persona, 800, 1500)
      await fillCheckoutForm(page, persona)
      await page.getByRole('button', { name: 'Place order' }).click()
      await page.locator('text=cart is empty').waitFor({ timeout: 15000 }).catch(() => {})
      await think(page, persona, 1500, 3000)
    }
  },

  cartAbandon: {
    weight: 15, cart: true,
    async run(page, persona) {
      await browseAround(page, persona, { categoryHops: rand(1, 2), hovers: 2 })
      await addRandomItems(page, persona, 2)
      await goToCart(page)
      await think(page, persona, 1500, 3500)
      const removes = page.getByRole('button', { name: 'Remove' })
      if ((await removes.count()) > 0) {
        await removes.first().click()
        await think(page, persona, 1000, 2500)
      }
      await goToShop(page)
      await browseAround(page, persona, { categoryHops: 1, hovers: 1 })
    }
  },

  windowShopper: {
    weight: 20, cart: false,
    async run(page, persona) {
      await waitForProducts(page)
      for (const cat of ['Electronics', 'Home', pick(CATEGORIES)]) {
        await page.getByRole('button', { name: cat, exact: true }).click()
        await think(page, persona, 1000, 3000)
        await page.mouse.wheel(0, rand(200, 900))
        await think(page, persona, 800, 2000)
      }
      // Quick-view a working product (anything but the 4K Monitor).
      await page.getByRole('button', { name: 'Books', exact: true }).click()
      await waitForProducts(page)
      const card = page.locator('div.group').first()
      await card.hover()
      await card.getByRole('button', { name: 'Quick view' }).click()
      await think(page, persona, 1500, 3500)
      await page.getByRole('button', { name: 'Close' }).click()
      await think(page, persona, 800, 2000)
    }
  },

  quickViewCrash: {
    weight: 10, cart: false,
    async run(page, persona) {
      await waitForProducts(page)
      await page.getByRole('button', { name: 'Electronics', exact: true }).click()
      await think(page, persona, 800, 2000)
      const card = page.locator('div.group').filter({ hasText: '4K Monitor' }).first()
      await card.hover()
      await card.getByRole('button', { name: 'Quick view' }).click()
      await page.locator('text=No variants available').waitFor({ timeout: 5000 }).catch(() => {})
      await think(page, persona, 1000, 2500)
      await browseAround(page, persona, { categoryHops: 1, hovers: 2 })
    }
  },

  idler: {
    weight: 7, cart: false,
    async run(page, persona) {
      await waitForProducts(page)
      for (let i = 0; i < rand(4, 8); i++) {
        await page.mouse.wheel(0, rand(-300, 700))
        await page.waitForTimeout(rand(3000, 8000))
      }
    }
  }
}

export function pickJourney() {
  const entries = Object.entries(journeys)
  const total = entries.reduce((n, [, j]) => n + j.weight, 0)
  let roll = Math.random() * total
  for (const [name, j] of entries) {
    roll -= j.weight
    if (roll <= 0) return { name, ...j }
  }
  const [name, j] = entries[entries.length - 1]
  return { name, ...j }
}
