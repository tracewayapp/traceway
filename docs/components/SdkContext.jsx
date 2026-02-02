import { createContext, useContext, useState, useEffect, useCallback } from 'react'
import { useRouter } from 'next/router'

export const SDK_OPTIONS = [
  { value: 'go-gin', label: 'Go Gin' },
  { value: 'go-chi', label: 'Go Chi' },
  { value: 'go-fiber', label: 'Go Fiber' },
  { value: 'go-fasthttp', label: 'Go FastHTTP' },
  { value: 'go-http', label: 'Go Http' },
  { value: 'go-generic', label: 'Go Generic' },
]

const STORAGE_KEY = 'traceway-docs-sdk'
const VALID_VALUES = new Set(SDK_OPTIONS.map((o) => o.value))

const SdkContext = createContext({
  sdk: 'go-gin',
  setSdk: () => {},
})

export function SdkProvider({ children }) {
  const router = useRouter()
  const [sdk, setSdkState] = useState('go-gin')

  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    const urlSdk = params.get('sdk')
    if (urlSdk && VALID_VALUES.has(urlSdk)) {
      setSdkState(urlSdk)
      localStorage.setItem(STORAGE_KEY, urlSdk)
      return
    }

    const stored = localStorage.getItem(STORAGE_KEY)
    if (stored && VALID_VALUES.has(stored)) {
      setSdkState(stored)
    }
  }, [])

  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    const urlSdk = params.get('sdk')
    if (urlSdk && VALID_VALUES.has(urlSdk) && urlSdk !== sdk) {
      setSdkState(urlSdk)
      localStorage.setItem(STORAGE_KEY, urlSdk)
    }
  }, [router.asPath])

  // Keep sdk param sticky on all /client pages
  useEffect(() => {
    function handleRouteChange(url) {
      if (!url.startsWith('/client')) return

      const [pathname, search] = url.split('?')
      const params = new URLSearchParams(search || '')

      if (pathname === '/client') {
        if (params.has('sdk')) {
          params.delete('sdk')
          const qs = params.toString()
          router.replace(pathname + (qs ? '?' + qs : ''), undefined, { shallow: true })
        }
        localStorage.removeItem(STORAGE_KEY)
        setSdkState('go-gin')
        return
      }

      if (!params.get('sdk')) {
        const currentSdk = localStorage.getItem(STORAGE_KEY) || sdk
        if (currentSdk && VALID_VALUES.has(currentSdk)) {
          params.set('sdk', currentSdk)
          router.replace(pathname + '?' + params.toString(), undefined, { shallow: true })
        }
      }
    }

    router.events.on('routeChangeComplete', handleRouteChange)
    return () => router.events.off('routeChangeComplete', handleRouteChange)
  }, [router, sdk])

  const setSdk = useCallback((value) => {
    if (!VALID_VALUES.has(value)) return
    setSdkState(value)
    localStorage.setItem(STORAGE_KEY, value)

    const url = new URL(window.location.href)
    url.searchParams.set('sdk', value)
    router.replace(url.pathname + url.search, undefined, { shallow: true })
  }, [router])

  return (
    <SdkContext.Provider value={{ sdk, setSdk }}>
      {children}
    </SdkContext.Provider>
  )
}

export function useSdk() {
  return useContext(SdkContext)
}
