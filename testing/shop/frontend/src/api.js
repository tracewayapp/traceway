import { captureExceptionWithAttributes } from '@tracewayapp/react'

const BASE = import.meta.env.VITE_API_BASE || '/api'

async function request(path, options) {
  const res = await fetch(BASE + path, options)
  if (!res.ok) {
    let detail = ''
    try {
      const body = await res.json()
      detail = body.error || ''
    } catch {
      detail = ''
    }
    const err = new Error(`Request to ${path} failed (${res.status})${detail ? ': ' + detail : ''}`)
    const distributedTraceId = res.headers.get('traceway-trace-id') || undefined
    captureExceptionWithAttributes(
      err,
      {
        path,
        method: options?.method || 'GET',
        status: String(res.status)
      },
      distributedTraceId ? { distributedTraceId } : undefined
    )
    throw err
  }
  if (res.status === 204) return null
  return res.json()
}

export function getProducts() {
  return request('/products')
}

export function getProduct(id) {
  return request(`/products/${id}`)
}

export function getCart() {
  return request('/cart')
}

export function addToCart(productId, qty = 1) {
  return request('/cart', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ product_id: productId, qty })
  })
}

export function removeFromCart(id) {
  return request(`/cart/${id}`, { method: 'DELETE' })
}

export function applyCoupon(code) {
  return request('/coupon', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ code })
  })
}

export function checkout(payload) {
  return request('/checkout', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload)
  })
}

export function formatPrice(cents) {
  return '$' + (cents / 100).toFixed(2)
}
