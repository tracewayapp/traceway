import React, { useState } from 'react'
import { useTraceway, TracewayErrorBoundary } from '@tracewayapp/react'
import { applyCoupon, checkout, formatPrice } from './api.js'

function CouponBadge({ discount }) {
  return (
    <div className="coupon-badge">
      Coupon applied — saved {discount.percent.toFixed(0)}% ({formatPrice(discount.cents)})
    </div>
  )
}

export default function CheckoutPage({ onDone }) {
  const { captureException } = useTraceway()
  const [code, setCode] = useState('')
  const [discount, setDiscount] = useState(undefined)
  const [couponError, setCouponError] = useState('')
  const [form, setForm] = useState({ name: '', email: '', card_last4: '' })
  const [result, setResult] = useState(null)
  const [checkoutError, setCheckoutError] = useState('')
  const [busy, setBusy] = useState(false)

  async function handleApplyCoupon() {
    setCouponError('')
    try {
      const res = await applyCoupon(code)
      setDiscount({ percent: res.percent_off, cents: res.discount_cents })
    } catch (e) {
      captureException(e)
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
      <div className="confirmation">
        <h1>Order confirmed 🎉</h1>
        <p>
          Order <strong>{result.order_id}</strong> placed for{' '}
          <strong>{formatPrice(result.total_cents)}</strong>.
        </p>
        <button className="primary" onClick={onDone}>
          Back to shop
        </button>
      </div>
    )
  }

  return (
    <div className="checkout">
      <h1>Checkout</h1>

      <section className="panel">
        <h2>Coupon</h2>
        <div className="coupon-row">
          <input
            placeholder="Coupon code (try SAVE10)"
            value={code}
            onChange={(e) => setCode(e.target.value)}
          />
          <button className="ghost" onClick={handleApplyCoupon}>
            Apply
          </button>
        </div>
        {couponError && <div className="error-inline">{couponError}</div>}
        {discount !== undefined && (
          <TracewayErrorBoundary fallback={<div className="error-inline">Could not display coupon.</div>}>
            <CouponBadge discount={discount} />
          </TracewayErrorBoundary>
        )}
      </section>

      <section className="panel">
        <h2>Payment</h2>
        <form onSubmit={handleCheckout} className="checkout-form">
          <label>
            Name
            <input value={form.name} onChange={update('name')} required />
          </label>
          <label>
            Email
            <input type="email" value={form.email} onChange={update('email')} required />
          </label>
          <label>
            Card last 4
            <input value={form.card_last4} onChange={update('card_last4')} maxLength={4} required />
          </label>
          <button className="primary" type="submit" disabled={busy}>
            {busy ? 'Placing order…' : 'Place order'}
          </button>
          {checkoutError && <div className="error-inline">{checkoutError}</div>}
        </form>
      </section>
    </div>
  )
}
