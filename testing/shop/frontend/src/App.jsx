import React, { useState, useEffect } from 'react'
import { setAttributes } from '@tracewayapp/react'
import { getCart } from './api.js'
import { getDemoUser } from './demoUser.js'
import ProductsPage from './ProductsPage.jsx'
import CartPage from './CartPage.jsx'
import CheckoutPage from './CheckoutPage.jsx'
import { cx, Logo, IconBag } from './ui.jsx'

const NAV = [
  { key: 'products', label: 'Shop' },
  { key: 'cart', label: 'Cart' },
  { key: 'checkout', label: 'Checkout' }
]

export default function App() {
  const [page, setPage] = useState('products')
  const [cartCount, setCartCount] = useState(0)

  async function refreshCartCount() {
    try {
      const cart = await getCart()
      const count = (cart.items || []).reduce((n, l) => n + l.qty, 0)
      setCartCount(count)
    } catch {
      setCartCount(0)
    }
  }

  useEffect(() => {
    const user = getDemoUser()
    setAttributes({ 'user.email': user.email, 'user.name': user.name, plan: user.plan })
  }, [])

  useEffect(() => {
    refreshCartCount()
  }, [page])

  return (
    <div className="isolate flex min-h-dvh flex-col">
      <div className="bg-neutral-900 px-4 py-2 text-center text-xs font-medium text-neutral-300 sm:text-sm">
        Free shipping on orders over $50 · 30-day returns, no questions asked.
      </div>

      <header className="sticky top-0 z-40 border-b border-neutral-950/5 bg-white/80 backdrop-blur-md">
        <div className="mx-auto flex h-16 max-w-6xl items-center justify-between gap-4 px-4 sm:px-6">
          <button
            type="button"
            onClick={() => setPage('products')}
            className="group flex items-center gap-2.5 rounded-lg focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-neutral-900"
          >
            <Logo className="size-8" />
            <span className="text-lg font-semibold tracking-tight text-neutral-900">Shoply</span>
          </button>

          <nav className="flex items-center gap-1">
            {NAV.map((item) => {
              const active = page === item.key
              if (item.key === 'cart') {
                return (
                  <button
                    key={item.key}
                    type="button"
                    onClick={() => setPage('cart')}
                    aria-label={`Cart, ${cartCount} items`}
                    className={cx(
                      'relative inline-flex items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-neutral-900',
                      active ? 'text-neutral-900' : 'text-neutral-500 hover:text-neutral-900'
                    )}
                  >
                    <span className="relative inline-flex">
                      <IconBag className="size-5" />
                      {cartCount > 0 && (
                        <span className="absolute -top-2 -right-2 inline-flex h-4 min-w-4 items-center justify-center rounded-full bg-neutral-900 px-1 text-[0.625rem] font-semibold tabular-nums text-white">
                          {cartCount}
                        </span>
                      )}
                    </span>
                    <span className="max-sm:hidden">Cart</span>
                  </button>
                )
              }
              return (
                <button
                  key={item.key}
                  type="button"
                  onClick={() => setPage(item.key)}
                  className={cx(
                    'rounded-lg px-3 py-2 text-sm font-medium transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-neutral-900',
                    active ? 'text-neutral-900' : 'text-neutral-500 hover:text-neutral-900'
                  )}
                >
                  {item.label}
                </button>
              )
            })}
          </nav>
        </div>
      </header>

      <main className="mx-auto w-full max-w-6xl flex-1 px-4 py-10 sm:px-6 sm:py-12">
        {page === 'products' && <ProductsPage onCartChanged={refreshCartCount} />}
        {page === 'cart' && (
          <CartPage
            onCartChanged={refreshCartCount}
            goCheckout={() => setPage('checkout')}
            goShop={() => setPage('products')}
          />
        )}
        {page === 'checkout' && (
          <CheckoutPage
            onDone={() => {
              refreshCartCount()
              setPage('products')
            }}
          />
        )}
      </main>

      <footer className="border-t border-neutral-950/5">
        <div className="mx-auto flex max-w-6xl flex-col gap-6 px-4 py-10 sm:flex-row sm:items-center sm:justify-between sm:px-6">
          <div className="flex items-center gap-2.5">
            <Logo className="size-7" />
            <p className="text-sm text-neutral-500 text-pretty">
              Shoply — modern goods for everyday life.
            </p>
          </div>
          <nav className="flex flex-wrap gap-x-6 gap-y-2 text-sm text-neutral-500">
            <a href="#" className="hover:text-neutral-900">About</a>
            <a href="#" className="hover:text-neutral-900">Shipping</a>
            <a href="#" className="hover:text-neutral-900">Returns</a>
            <a href="#" className="hover:text-neutral-900">Powered by Traceway</a>
          </nav>
        </div>
      </footer>
    </div>
  )
}
