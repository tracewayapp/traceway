import React, { useEffect, useState } from 'react'
import { applyCoupon, checkout, getCart, formatPrice } from './api.js'
import { Button, IconCheck, IconLock } from './ui.jsx'

class ErrorBoundary extends React.Component {
  constructor(props) {
    super(props)
    this.state = { failed: false }
  }
  static getDerivedStateFromError() {
    return { failed: true }
  }
  render() {
    return this.state.failed ? this.props.fallback : this.props.children
  }
}

function CouponBadge({ discount }) {
  return (
    <div className="flex items-center gap-2 rounded-lg bg-emerald-50 px-3 py-2.5 text-sm font-medium text-emerald-700 ring-1 ring-emerald-600/20">
      <IconCheck className="size-4 shrink-0" />
      Coupon applied — saved {discount.percent.toFixed(0)}% ({formatPrice(discount.cents)})
    </div>
  )
}

const inputClass =
  'w-full rounded-lg bg-white px-3.5 py-2.5 text-sm text-neutral-900 ring-1 ring-neutral-950/10 outline-none placeholder:text-neutral-400 focus-visible:ring-2 focus-visible:ring-neutral-900 max-sm:text-base'

export default function CheckoutPage({ onDone }) {
  const [code, setCode] = useState('')
  const [discount, setDiscount] = useState(undefined)
  const [couponError, setCouponError] = useState('')
  const [form, setForm] = useState({ name: '', email: '', card_last4: '' })
  const [result, setResult] = useState(null)
  const [checkoutError, setCheckoutError] = useState('')
  const [busy, setBusy] = useState(false)
  const [cart, setCart] = useState(null)

  useEffect(() => {
    getCart()
      .then(setCart)
      .catch(() => setCart(null))
  }, [])

  async function handleApplyCoupon() {
    setCouponError('')
    try {
      const res = await applyCoupon(code)
      setDiscount({ percent: res.percent_off, cents: res.discount_cents })
    } catch (e) {
      setCouponError(e.message)
      setDiscount(null)
    }
  }

  async function handleCheckout(e) {
    e.preventDefault()
    setCheckoutError('')
    setBusy(true)
    try {
      const res = await checkout(form)
      setResult(res)
    } catch (err) {
      setCheckoutError(err.message)
    } finally {
      setBusy(false)
    }
  }

  function update(field) {
    return (e) => setForm({ ...form, [field]: e.target.value })
  }

  if (result) {
    return (
      <div className="mx-auto max-w-md py-16 text-center">
        <div className="mx-auto flex size-14 items-center justify-center rounded-full bg-emerald-50 ring-1 ring-emerald-600/20">
          <IconCheck className="size-7 text-emerald-600" />
        </div>
        <h1 className="mt-5 text-2xl font-semibold tracking-tight text-neutral-900">Order confirmed</h1>
        <p className="mx-auto mt-2 max-w-[44ch] text-sm text-pretty text-neutral-500">
          Order <strong className="font-medium text-neutral-900">{result.order_id}</strong> placed for{' '}
          <strong className="font-medium text-neutral-900 tabular-nums">{formatPrice(result.total_cents)}</strong>. A
          receipt is on its way to your inbox.
        </p>
        <Button className="mt-6" onClick={onDone}>
          Back to shop
        </Button>
      </div>
    )
  }

  const validDiscount = discount && typeof discount.cents === 'number' ? discount : null
  const subtotal = cart?.total_cents ?? 0
  const total = validDiscount ? Math.max(0, subtotal - validDiscount.cents) : subtotal

  return (
    <div>
      <h1 className="text-2xl font-semibold tracking-tight text-neutral-900">Checkout</h1>

      <div className="mt-8 grid gap-10 lg:grid-cols-[1fr_22rem]">
        <div className="flex flex-col gap-10">
          <section>
            <h2 className="text-sm font-semibold text-neutral-900">Have a coupon?</h2>
            <p className="mt-1 text-sm text-neutral-500">Try {''}<code className="rounded bg-neutral-100 px-1.5 py-0.5 text-xs font-medium text-neutral-700">SAVE10</code>.</p>
            <div className="mt-3 flex gap-2">
              <input
                type="text"
                name="coupon"
                aria-label="Coupon code"
                placeholder="Coupon code"
                value={code}
                onChange={(e) => setCode(e.target.value)}
                className={inputClass}
              />
              <Button variant="secondary" onClick={handleApplyCoupon} className="shrink-0">
                Apply
              </Button>
            </div>
            {couponError && <p className="mt-2 text-sm text-red-600">{couponError}</p>}
            {discount !== undefined && (
              <div className="mt-3">
                <ErrorBoundary
                  fallback={
                    <div className="rounded-lg bg-red-50 px-3 py-2.5 text-sm font-medium text-red-700 ring-1 ring-red-600/20">
                      Could not display coupon.
                    </div>
                  }
                >
                  <CouponBadge discount={discount} />
                </ErrorBoundary>
              </div>
            )}
          </section>

          <section>
            <h2 className="text-sm font-semibold text-neutral-900">Contact &amp; payment</h2>
            <form onSubmit={handleCheckout} className="mt-3 flex flex-col gap-4">
              <div className="flex flex-col gap-1.5">
                <label htmlFor="co-name" className="text-sm font-medium text-neutral-700">Full name</label>
                <input id="co-name" name="name" value={form.name} onChange={update('name')} required placeholder="Ada Lovelace" className={inputClass} />
              </div>
              <div className="flex flex-col gap-1.5">
                <label htmlFor="co-email" className="text-sm font-medium text-neutral-700">Email</label>
                <input id="co-email" name="email" type="email" value={form.email} onChange={update('email')} required placeholder="ada@example.com" className={inputClass} />
              </div>
              <div className="flex flex-col gap-1.5">
                <label htmlFor="co-card" className="text-sm font-medium text-neutral-700">Card last 4</label>
                <input id="co-card" name="card_last4" value={form.card_last4} onChange={update('card_last4')} maxLength={4} required placeholder="4242" inputMode="numeric" className={inputClass} />
              </div>
              <div className="mt-2">
                <Button type="submit" className="w-full" disabled={busy}>
                  {busy ? 'Placing order…' : 'Place order'}
                </Button>
                {checkoutError && <p className="mt-2 text-sm text-red-600">{checkoutError}</p>}
                <p className="mt-3 flex items-center justify-center gap-1.5 text-xs text-neutral-500">
                  <IconLock className="size-3.5" />
                  Payments are encrypted. This is a demo — no card is charged.
                </p>
              </div>
            </form>
          </section>
        </div>

        <aside className="lg:sticky lg:top-28 lg:self-start">
          <div className="rounded-2xl bg-neutral-50 p-6 ring-1 ring-neutral-950/5">
            <h2 className="text-sm font-semibold text-neutral-900">Order summary</h2>
            {cart && cart.items?.length > 0 ? (
              <ul role="list" className="mt-4 space-y-3">
                {cart.items.map((line) => (
                  <li key={line.id} className="flex items-center gap-3">
                    <img src={line.image_url} alt={line.name} className="size-12 shrink-0 rounded-md object-cover ring-1 ring-black/5" />
                    <div className="min-w-0 flex-1">
                      <p className="truncate text-sm font-medium text-neutral-900">{line.name}</p>
                      <p className="text-xs tabular-nums text-neutral-500">Qty {line.qty}</p>
                    </div>
                    <p className="text-sm tabular-nums text-neutral-700">{formatPrice(line.line_total_cents)}</p>
                  </li>
                ))}
              </ul>
            ) : (
              <p className="mt-4 text-sm text-neutral-500">Your order details will appear here.</p>
            )}
            <dl className="mt-5 space-y-3 border-t border-neutral-950/10 pt-5 text-sm">
              <div className="flex items-center justify-between">
                <dt className="text-neutral-500">Subtotal</dt>
                <dd className="font-medium tabular-nums text-neutral-900">{formatPrice(subtotal)}</dd>
              </div>
              {validDiscount && (
                <div className="flex items-center justify-between">
                  <dt className="text-neutral-500">Discount</dt>
                  <dd className="font-medium tabular-nums text-emerald-600">−{formatPrice(validDiscount.cents)}</dd>
                </div>
              )}
              <div className="flex items-center justify-between">
                <dt className="text-neutral-500">Shipping</dt>
                <dd className="font-medium text-emerald-600">Free</dd>
              </div>
              <div className="flex items-center justify-between border-t border-neutral-950/10 pt-3">
                <dt className="font-medium text-neutral-900">Total</dt>
                <dd className="text-base font-semibold tabular-nums text-neutral-900">{formatPrice(total)}</dd>
              </div>
            </dl>
          </div>
        </aside>
      </div>
    </div>
  )
}
