import { useState, useRef, useEffect } from 'react'
import { useRouter } from 'next/router'
import { useSdk, SDK_OPTIONS } from './SdkContext'

const SDK_QUICK_START = {
  'go-gin': '/client/gin-middleware',
  'go-chi': '/client/chi-middleware',
  'go-fiber': '/client/fiber-middleware',
  'go-fasthttp': '/client/fasthttp-middleware',
  'go-http': '/client/http-middleware',
  'go-generic': '/client/sdk',
  'js-node': '/client/node-sdk',
  'js-nestjs': '/client/nestjs',
  'js-react': '/client/react',
  'js-vue': '/client/vue',
  'js-svelte': '/client/svelte',
  'js-generic': '/client/js-sdk',
}

export default function SdkSelector() {
  const router = useRouter()
  const { sdk, setSdk } = useSdk()
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState('')
  const ref = useRef(null)
  const searchRef = useRef(null)

  const current = SDK_OPTIONS.find((o) => o.value === sdk)
  const filtered = SDK_OPTIONS.filter((o) =>
    o.label.toLowerCase().includes(search.toLowerCase())
  )

  useEffect(() => {
    function handleClick(e) {
      if (ref.current && !ref.current.contains(e.target)) {
        setOpen(false)
        setSearch('')
      }
    }
    document.addEventListener('mousedown', handleClick)
    return () => document.removeEventListener('mousedown', handleClick)
  }, [])

  useEffect(() => {
    if (open && searchRef.current) {
      searchRef.current.focus()
    }
  }, [open])

  return (
    <div className="sdk-selector" ref={ref}>
      <button
        className="sdk-selector-trigger"
        onClick={() => {
          setOpen(!open)
          setSearch('')
        }}
      >
        <span>{current?.label ?? 'Select SDK'}</span>
        <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
          <path
            d="M3 4.5L6 7.5L9 4.5"
            stroke="currentColor"
            strokeWidth="1.5"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
      </button>
      {open && (
        <div className="sdk-selector-dropdown">
          <input
            ref={searchRef}
            className="sdk-selector-search"
            type="text"
            placeholder="Search..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
          <ul className="sdk-selector-list">
            {filtered.map((option) => (
              <li key={option.value}>
                <button
                  className={`sdk-selector-option ${option.value === sdk ? 'active' : ''}`}
                  onClick={() => {
                    setSdk(option.value)
                    setOpen(false)
                    setSearch('')
                    const href = SDK_QUICK_START[option.value]
                    if (href) {
                      router.push(`${href}?sdk=${option.value}`)
                    }
                  }}
                >
                  {option.label}
                </button>
              </li>
            ))}
            {filtered.length === 0 && (
              <li className="sdk-selector-empty">No results</li>
            )}
          </ul>
        </div>
      )}
    </div>
  )
}
