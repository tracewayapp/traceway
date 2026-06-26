import React, { useEffect, useState } from 'react'
import { getProducts, addToCart, formatPrice } from './api.js'
import { Button, IconButton, Stars, IconClose, IconCheck, IconEye, cx } from './ui.jsx'

export default function ProductsPage({ onCartChanged }) {
  const [products, setProducts] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [adding, setAdding] = useState(null)
  const [quickView, setQuickView] = useState(null)
  const [notice, setNotice] = useState('')
  const [category, setCategory] = useState('All')

  async function load() {
    setLoading(true)
    setError('')
    try {
      const data = await getProducts()
      setProducts(data)
    } catch (e) {
      setError(e.message)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  useEffect(() => {
    if (!notice) return
    const t = setTimeout(() => setNotice(''), 2200)
    return () => clearTimeout(t)
  }, [notice])

  async function handleAdd(productId) {
    setAdding(productId)
    setNotice('')
    await addToCart(productId, 1)
    setNotice('Added to cart')
    setAdding(null)
    onCartChanged?.()
  }

  function handleQuickView(p) {
    setQuickView(p)
  }

  const categories = ['All', ...Array.from(new Set(products.map((p) => p.category).filter(Boolean)))]
  const visible = category === 'All' ? products : products.filter((p) => p.category === category)

  return (
    <div>
      <section className="border-b border-neutral-950/5 pb-10">
        <p className="text-sm font-medium text-neutral-500">New season</p>
        <h1 className="mt-2 max-w-[18ch] text-4xl font-semibold tracking-tight text-balance text-neutral-900 sm:text-5xl">
          Modern goods for everyday life.
        </h1>
        <p className="mt-4 max-w-[60ch] text-base text-pretty text-neutral-500">
          A small, considered catalog of apparel, electronics, and home essentials — shipped fast and
          backed by a 30-day return policy.
        </p>
      </section>

      <div className="sticky top-16 z-30 -mx-4 mt-2 bg-white/80 px-4 py-4 backdrop-blur-md sm:-mx-6 sm:px-6">
        <div className="no-scrollbar flex gap-2 overflow-x-auto">
          {categories.map((c) => (
            <button
              key={c}
              type="button"
              onClick={() => setCategory(c)}
              className={cx(
                'shrink-0 rounded-full px-4 py-1.5 text-sm font-medium transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-neutral-900',
                category === c
                  ? 'bg-neutral-900 text-white'
                  : 'bg-neutral-100 text-neutral-600 hover:bg-neutral-200 hover:text-neutral-900'
              )}
            >
              {c}
            </button>
          ))}
        </div>
      </div>

      {loading ? (
        <ProductGridSkeleton />
      ) : error ? (
        <div className="mx-auto mt-16 max-w-md rounded-2xl border border-neutral-950/10 p-8 text-center">
          <p className="text-sm text-neutral-600 text-pretty">{error}</p>
          <Button variant="secondary" size="sm" className="mt-4" onClick={load}>
            Try again
          </Button>
        </div>
      ) : (
        <div className="mt-6 grid grid-cols-1 gap-x-6 gap-y-10 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
          {visible.map((p) => (
            <ProductCard
              key={p.id}
              product={p}
              adding={adding === p.id}
              onAdd={() => handleAdd(p.id)}
              onQuickView={() => handleQuickView(p)}
            />
          ))}
        </div>
      )}

      {quickView && (
        <QuickViewModal
          product={quickView}
          adding={adding === quickView.id}
          onAdd={() => handleAdd(quickView.id)}
          onClose={() => setQuickView(null)}
        />
      )}

      <div
        aria-live="polite"
        className="pointer-events-none fixed inset-x-0 bottom-6 z-50 flex justify-center px-4"
      >
        {notice && (
          <div className="pointer-events-auto inline-flex items-center gap-2 rounded-full bg-neutral-900 py-2.5 pr-4 pl-3 text-sm font-medium text-white shadow-lg ring-1 ring-black/5">
            <span className="inline-flex size-5 items-center justify-center rounded-full bg-emerald-500/20">
              <IconCheck className="size-3.5 text-emerald-300" />
            </span>
            {notice}
          </div>
        )}
      </div>
    </div>
  )
}

function ProductCard({ product: p, adding, onAdd, onQuickView }) {
  return (
    <div className="group flex flex-col">
      <div className="relative aspect-4/3 overflow-hidden rounded-xl bg-neutral-100">
        <img
          src={p.image_url}
          alt={p.name}
          loading="lazy"
          className="size-full object-cover transition duration-500 group-hover:scale-105"
        />
        {p.stock <= 20 && (
          <span className="absolute top-3 left-3 rounded-full bg-white/90 px-2.5 py-1 text-xs font-medium text-neutral-700 ring-1 ring-black/5 backdrop-blur">
            Only {p.stock} left
          </span>
        )}
        <div className="absolute inset-x-3 bottom-3 flex translate-y-2 justify-center opacity-0 transition duration-300 group-hover:translate-y-0 group-hover:opacity-100">
          <Button
            variant="secondary"
            size="sm"
            onClick={onQuickView}
            className="w-full shadow-sm backdrop-blur"
          >
            <IconEye className="size-4" />
            Quick view
          </Button>
        </div>
      </div>

      <div className="mt-4 flex flex-1 flex-col">
        <div className="flex items-start justify-between gap-3">
          <h3 className="text-sm font-medium text-neutral-900">{p.name}</h3>
          <p className="text-sm font-medium tabular-nums text-neutral-900">{formatPrice(p.price_cents)}</p>
        </div>
        <div className="mt-1 flex items-center gap-2 text-sm text-neutral-500">
          <span>{p.category}</span>
          <span aria-hidden="true">·</span>
          <Stars count={p.review_count} />
        </div>
        <p className="mt-2 line-clamp-2 text-sm text-pretty text-neutral-500">{p.description}</p>
        <div className="mt-4 flex items-end">
          <Button className="w-full" disabled={adding} onClick={onAdd}>
            {adding ? 'Adding…' : 'Add to cart'}
          </Button>
        </div>
      </div>
    </div>
  )
}

function QuickViewModal({ product: p, adding, onAdd, onClose }) {
  useEffect(() => {
    function onKey(e) {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  return (
    <div
      className="fixed inset-0 z-50 flex items-end justify-center bg-neutral-950/40 p-4 backdrop-blur-sm sm:items-center"
      role="dialog"
      aria-modal="true"
      aria-label={`${p.name} quick view`}
      onClick={onClose}
    >
      <div
        className="relative w-full max-w-2xl overflow-hidden rounded-2xl bg-white shadow-xl ring-1 ring-black/5"
        onClick={(e) => e.stopPropagation()}
      >
        <IconButton label="Close" onClick={onClose} className="absolute top-3 right-3 z-10 bg-white/70 backdrop-blur">
          <IconClose className="size-5" />
        </IconButton>
        <div className="grid sm:grid-cols-2">
          <div className="aspect-4/3 bg-neutral-100 sm:aspect-auto">
            <img src={p.image_url} alt={p.name} className="size-full object-cover" />
          </div>
          <div className="flex flex-col gap-3 p-6">
            <p className="text-sm text-neutral-500">{p.category}</p>
            <h2 className="text-xl font-semibold tracking-tight text-balance text-neutral-900">{p.name}</h2>
            <div className="flex items-center gap-3">
              <p className="text-lg font-semibold tabular-nums text-neutral-900">{formatPrice(p.price_cents)}</p>
              <Stars count={p.review_count} className="text-sm" />
            </div>
            <p className="text-sm text-pretty text-neutral-500">{p.description}</p>
            <p className="text-sm text-neutral-500">
              {p.stock > 20 ? 'In stock — ships in 1–2 days.' : `Only ${p.stock} left in stock.`}
            </p>
            <div className="mt-auto pt-2">
              <Button className="w-full" disabled={adding} onClick={onAdd}>
                {adding ? 'Adding…' : 'Add to cart'}
              </Button>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

function ProductGridSkeleton() {
  return (
    <div className="mt-6 grid grid-cols-1 gap-x-6 gap-y-10 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
      {Array.from({ length: 8 }).map((_, i) => (
        <div key={i} className="flex flex-col">
          <div className="aspect-4/3 animate-pulse rounded-xl bg-neutral-100" />
          <div className="mt-4 h-4 w-2/3 animate-pulse rounded bg-neutral-100" />
          <div className="mt-2 h-3 w-1/3 animate-pulse rounded bg-neutral-100" />
          <div className="mt-4 h-9 w-full animate-pulse rounded-lg bg-neutral-100" />
        </div>
      ))}
    </div>
  )
}
