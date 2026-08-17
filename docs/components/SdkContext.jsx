import { createContext, useContext, useState, useEffect, useCallback } from 'react'
import { useRouter } from 'next/router'

// One entry per integration path. Every backend (Go, Node, Python, PHP, Java,
// .NET, Ruby) is the single `otel` option, and the per-framework guides live
// inside /client/otel.
export const SDK_OPTIONS = [
  { value: 'otel', label: 'OpenTelemetry (any backend)' },
  { value: 'js-react', label: 'React' },
  { value: 'js-vue', label: 'Vue.js' },
  { value: 'js-svelte', label: 'Svelte' },
  { value: 'js-jquery', label: 'jQuery' },
  { value: 'js-generic', label: 'JS Generic' },
  { value: 'flutter', label: 'Flutter' },
  { value: 'android', label: 'Android' },
  { value: 'ios', label: 'iOS' },
  { value: 'react-native', label: 'React Native' },
  { value: 'openrouter', label: 'OpenRouter' },
]

const STORAGE_KEY = 'traceway-docs-sdk'
const VALID_VALUES = new Set(SDK_OPTIONS.map((o) => o.value))

// First path segment after /client/ → exact SDK value. These folders map 1:1
// to a single framework, so the path alone is authoritative.
const FOLDER_SDK = {
  'otel': 'otel',
  'react': 'js-react',
  'react-native': 'react-native',
  'vue': 'js-vue',
  'svelte': 'js-svelte',
  'jquery': 'js-jquery',
  'openrouter': 'openrouter',
  'flutter': 'flutter',
  'android': 'android',
  'ios': 'ios',
}

// Shared reference folders. The SDK cannot be derived from the path alone, so
// it is disambiguated by the remembered framework (lastSdk) when that belongs
// to the folder's group, otherwise by a generic fallback.
const SHARED_FOLDERS = {
  'js-sdk': {
    members: new Set(['js-react', 'js-vue', 'js-svelte', 'js-jquery', 'js-generic']),
    fallback: 'js-generic',
  },
}

function clientSegment(pathname) {
  if (!pathname || !pathname.startsWith('/client')) return undefined
  return pathname.split('/').filter(Boolean)[1]
}

// The path is the single source of truth for the active SDK. `concrete` is true
// when the path unambiguously implies the SDK (so it should be remembered).
export function resolveSdkFromPath(pathname, lastSdk) {
  const seg = clientSegment(pathname)
  if (seg === undefined) return { sdk: null, concrete: false }
  if (FOLDER_SDK[seg]) return { sdk: FOLDER_SDK[seg], concrete: true }
  const shared = SHARED_FOLDERS[seg]
  if (shared) {
    if (lastSdk && shared.members.has(lastSdk)) return { sdk: lastSdk, concrete: false }
    return { sdk: shared.fallback, concrete: false }
  }
  return { sdk: null, concrete: false }
}

const SdkContext = createContext({
  sdk: null,
  setSdk: () => {},
})

export function SdkProvider({ children }) {
  const router = useRouter()
  const [sdk, setSdkState] = useState(null)

  useEffect(() => {
    const asPath = router.asPath
    const hashIdx = asPath.indexOf('#')
    const hash = hashIdx >= 0 ? asPath.slice(hashIdx) : ''
    const noHash = hashIdx >= 0 ? asPath.slice(0, hashIdx) : asPath
    const qIdx = noHash.indexOf('?')
    const pathname = qIdx >= 0 ? noHash.slice(0, qIdx) : noHash
    const params = new URLSearchParams(qIdx >= 0 ? noHash.slice(qIdx + 1) : '')

    const lastSdk = localStorage.getItem(STORAGE_KEY)
    const { sdk: resolved, concrete } = resolveSdkFromPath(pathname, lastSdk)
    if (resolved) {
      setSdkState(resolved)
      if (concrete && VALID_VALUES.has(resolved)) {
        localStorage.setItem(STORAGE_KEY, resolved)
      }
    }

    // Self-heal: strip any stray ?sdk= so /client URLs stay clean and old
    // bookmarks that pinned the wrong SDK correct themselves.
    if (pathname.startsWith('/client') && params.has('sdk')) {
      params.delete('sdk')
      const qs = params.toString()
      router.replace(pathname + (qs ? '?' + qs : '') + hash, undefined, { shallow: true })
    }
  }, [router.asPath])

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
