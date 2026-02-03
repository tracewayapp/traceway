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

const PATH_SDK_MAP = {
  'gin-middleware': 'go-gin',
  'chi-middleware': 'go-chi',
  'fiber-middleware': 'go-fiber',
  'fasthttp-middleware': 'go-fasthttp',
  'http-middleware': 'go-http',
}

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

      const hashIdx = url.indexOf('#')
      const questionIdx = url.indexOf('?')

      let pathname, search, hash
      if (hashIdx === -1 && questionIdx === -1) {
        pathname = url; search = ''; hash = ''
      } else if (questionIdx !== -1 && (hashIdx === -1 || questionIdx < hashIdx)) {
        pathname = url.substring(0, questionIdx)
        const rest = url.substring(questionIdx + 1)
        const restHash = rest.indexOf('#')
        if (restHash !== -1) {
          search = rest.substring(0, restHash)
          hash = rest.substring(restHash)
        } else {
          search = rest; hash = ''
        }
      } else {
        pathname = url.substring(0, hashIdx)
        const rest = url.substring(hashIdx)
        const restQ = rest.indexOf('?')
        if (restQ !== -1) {
          hash = rest.substring(0, restQ)
          search = rest.substring(restQ + 1)
        } else {
          hash = rest; search = ''
        }
      }

      const params = new URLSearchParams(search)

      if (pathname === '/client') {
        if (params.has('sdk')) {
          params.delete('sdk')
          const qs = params.toString()
          router.replace(pathname + (qs ? '?' + qs : '') + hash, undefined, { shallow: true })
        }
        localStorage.removeItem(STORAGE_KEY)
        setSdkState('go-gin')
        return
      }

      if (!params.get('sdk')) {
        let detectedSdk = null
        for (const [folder, sdkValue] of Object.entries(PATH_SDK_MAP)) {
          if (pathname.includes('/' + folder)) {
            detectedSdk = sdkValue
            break
          }
        }
        const sdkToUse = detectedSdk || localStorage.getItem(STORAGE_KEY) || sdk
        if (sdkToUse && VALID_VALUES.has(sdkToUse)) {
          params.set('sdk', sdkToUse)
          if (detectedSdk) {
            setSdkState(detectedSdk)
            localStorage.setItem(STORAGE_KEY, detectedSdk)
          }
          const qs = params.toString()
          router.replace(pathname + (qs ? '?' + qs : '') + hash, undefined, { shallow: true })
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
  }, [])

  return (
    <SdkContext.Provider value={{ sdk, setSdk }}>
      {children}
    </SdkContext.Provider>
  )
}

export function useSdk() {
  return useContext(SdkContext)
}
