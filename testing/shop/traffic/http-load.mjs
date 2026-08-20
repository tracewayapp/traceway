import { randomUUID } from 'node:crypto'
import { PERSONAS, pick } from './personas.mjs'

function rand(min, max) {
  return min + Math.floor(Math.random() * (max - min))
}

const sleep = (ms) => new Promise((r) => setTimeout(r, ms))

async function req(base, path, options) {
  try {
    const res = await fetch(base + path, options)
    await res.arrayBuffer()
    return res.status
  } catch {
    return 0
  }
}

const jsonPost = (body) => ({
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify(body)
})

// One pass of mixed API traffic — endpoint volume without browser sessions.
export async function httpBurst(base, { requests = 25 } = {}) {
  for (let i = 0; i < requests; i++) {
    const roll = Math.random()
    if (roll < 0.3) {
      await req(base, '/api/products')
    } else if (roll < 0.55) {
      await req(base, `/api/products/${rand(1, 13)}`)
    } else if (roll < 0.65) {
      await req(base, '/api/cart')
    } else if (roll < 0.75) {
      await req(base, '/api/cart', jsonPost({ product_id: rand(1, 13), qty: rand(1, 3) }))
    } else if (roll < 0.85) {
      await req(base, '/api/coupon', jsonPost({ code: pick(['SAVE10', 'HALF50', 'EXPIRED', 'WELCOME5', 'NOPE']) }))
    } else if (roll < 0.95) {
      const p = pick(PERSONAS)
      await req(base, '/api/checkout', jsonPost({ name: p.name, email: p.email, card_last4: p.card }))
    } else {
      await req(base, `/api/cart/${rand(1, 50)}`, { method: 'DELETE' })
    }
    await sleep(rand(200, 1500))
  }
}

const CHAT_SCRIPTS = [
  {
    label: 'order-status',
    messages: [
      'Hi! Can you check on my order ORD-483920? It has been a week.',
      'Great, thanks. Will I get a notification when it is out for delivery?',
      'Perfect, thank you!'
    ]
  },
  {
    label: 'product-question',
    messages: [
      'What does the scented candle smell like? How long does it burn?',
      'Nice. What is your return policy if I do not like the scent?'
    ]
  },
  {
    label: 'refund-complaint',
    messages: [
      'My order arrived broken AGAIN. Honestly this is bullshit, third time in a row.',
      'I want a refund, not a replacement.',
      'Fine. How long until the money is back on my card?'
    ]
  },
  {
    label: 'shipping-question',
    messages: [
      'How fast is shipping to Berlin?',
      'Is express shipping available for order ORD-771203 that I placed an hour ago?'
    ]
  }
]

export async function aiConversation(base, { script, persona, turnDelayMs = [3000, 9000] } = {}) {
  const chosen = script || pick(CHAT_SCRIPTS)
  const user = persona || pick(PERSONAS)
  const conversationId = randomUUID()
  for (const message of chosen.messages) {
    await req(base, '/api/support/chat', jsonPost({
      conversation_id: conversationId,
      user_id: user.email,
      message
    }))
    await sleep(rand(turnDelayMs[0], turnDelayMs[1]))
  }
  return { conversationId, label: chosen.label, user: user.email }
}

export { CHAT_SCRIPTS }
