import React, { useEffect, useState } from 'react'
import { getCart, removeFromCart, formatPrice } from './api.js'
import { Button, IconBag, IconArrowRight, IconLock } from './ui.jsx'

export default function CartPage({ onCartChanged, goCheckout, goShop }) {
  const [cart, setCart] = useState({ items: [], total_cents: 0 })
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  async function load() {
    setLoading(true)
    setError('')
    try {
      const data = await getCart()
      setCart(data)
    } catch (e) {
      setError(e.message)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  async function handleRemove(id) {
    try {
      await removeFromCart(id)
      await load()
      onCartChanged?.()
    } catch (e) {
      setError(e.message)
    }
  }

  if (loading) return <p className="py-16 text-center text-sm text-neutral-500">Loading your cart…</p>
  if (error)
    return (
      <div className="mx-auto max-w-md rounded-2xl border border-neutral-950/10 p-8 text-center">
        <p className="text-sm text-neutral-600 text-pretty">{error}</p>
        <Button variant="secondary" size="sm" className="mt-4" onClick={load}>
          Try again
        </Button>
      </div>
    )

  if (cart.items.length === 0) {
    return (
      <div className="mx-auto max-w-md py-16 text-center">
        <div className="mx-auto flex size-14 items-center justify-center rounded-full bg-neutral-100">
          <IconBag className="size-7 text-neutral-400" />
        </div>
        <h1 className="mt-5 text-xl font-semibold tracking-tight text-neutral-900">Your cart is empty</h1>
        <p className="mx-auto mt-2 max-w-[40ch] text-sm text-pretty text-neutral-500">
          Looks like you haven’t added anything yet. Browse the catalog to get started.
        </p>
        <Button className="mt-6" onClick={goShop}>
          Continue shopping
          <IconArrowRight className="size-4" />
        </Button>
      </div>
    )
  }

  return (
    <div>
      <h1 className="text-2xl font-semibold tracking-tight text-neutral-900">Your cart</h1>
      <p className="mt-1 text-sm text-neutral-500">
        {cart.items.length} {cart.items.length === 1 ? 'item' : 'items'}
      </p>

      <div className="mt-8 grid gap-10 lg:grid-cols-[1fr_22rem]">
        <ul role="list" className="divide-y divide-neutral-950/5 border-y border-neutral-950/5">
          {cart.items.map((line) => (
            <li key={line.id} className="flex items-center gap-4 py-5">
              <img
                src={line.image_url}
                alt={line.name}
                className="size-20 shrink-0 rounded-lg object-cover ring-1 ring-black/5"
              />
              <div className="min-w-0 flex-1">
                <p className="text-sm font-medium text-neutral-900">{line.name}</p>
                <p className="mt-1 text-sm tabular-nums text-neutral-500">
                  {formatPrice(line.price_cents)} × {line.qty}
                </p>
                <button
                  type="button"
                  onClick={() => handleRemove(line.id)}
                  className="mt-2 text-sm font-medium text-neutral-500 underline-offset-4 transition hover:text-neutral-900 hover:underline"
                >
                  Remove
                </button>
              </div>
              <p className="text-sm font-medium tabular-nums text-neutral-900">
                {formatPrice(line.line_total_cents)}
              </p>
            </li>
          ))}
        </ul>

        <aside className="lg:sticky lg:top-28 lg:self-start">
          <div className="rounded-2xl bg-neutral-50 p-6 ring-1 ring-neutral-950/5">
            <h2 className="text-sm font-semibold text-neutral-900">Order summary</h2>
            <dl className="mt-4 space-y-3 text-sm">
              <div className="flex items-center justify-between">
                <dt className="text-neutral-500">Subtotal</dt>
                <dd className="font-medium tabular-nums text-neutral-900">{formatPrice(cart.total_cents)}</dd>
              </div>
              <div className="flex items-center justify-between">
                <dt className="text-neutral-500">Shipping</dt>
                <dd className="font-medium text-emerald-600">Free</dd>
              </div>
              <div className="flex items-center justify-between border-t border-neutral-950/10 pt-3">
                <dt className="font-medium text-neutral-900">Total</dt>
                <dd className="text-base font-semibold tabular-nums text-neutral-900">
                  {formatPrice(cart.total_cents)}
                </dd>
              </div>
            </dl>
            <Button className="mt-6 w-full" onClick={goCheckout}>
              Proceed to checkout
              <IconArrowRight className="size-4" />
            </Button>
            <p className="mt-3 flex items-center justify-center gap-1.5 text-xs text-neutral-500">
              <IconLock className="size-3.5" />
              Secure checkout
            </p>
          </div>
        </aside>
      </div>
    </div>
  )
}
