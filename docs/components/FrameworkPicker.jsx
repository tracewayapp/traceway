import { useRouter } from 'next/router'
import { useSdk } from './SdkContext'

const FRAMEWORKS = [
  {
    value: 'go-gin',
    label: 'Go Gin',
    description: 'Gin Gonic web framework with automatic request tracing and panic recovery.',
    icon: '/gin.png',
    href: '/client/gin-middleware',
  },
  {
    value: 'go-chi',
    label: 'Go Chi',
    description: 'Lightweight Chi router with automatic request tracing and panic recovery.',
    icon: '/chi.png',
    href: '/client/chi-middleware',
  },
  {
    value: 'go-fiber',
    label: 'Go Fiber',
    description: 'Express-inspired Fiber framework with request tracing and error capture.',
    icon: '/fiber.svg',
    href: '/client/fiber-middleware',
  },
  {
    value: 'go-fasthttp',
    label: 'Go FastHTTP',
    description: 'High-performance FastHTTP server with request tracing and panic recovery.',
    icon: '/fasthttp.png',
    href: '/client/fasthttp-middleware',
  },
  {
    value: 'go-http',
    label: 'Go net/http',
    description: 'Standard library HTTP middleware for request tracing and error capture.',
    icon: '/stdlib.png',
    href: '/client/http-middleware',
  },
  {
    value: 'go-generic',
    label: 'Go Generic',
    description: 'Framework-agnostic SDK for manual instrumentation of any Go application.',
    icon: '/custom.png',
    href: '/client/sdk',
  },
  {
    value: 'js-react',
    label: 'React',
    description: 'React applications with error boundaries and hooks.',
    icon: '/react.png',
    href: '/client/react',
  },
  {
    value: 'js-vue',
    label: 'Vue.js',
    description: 'Vue 3 applications with plugin and composables.',
    icon: '/vue.png',
    href: '/client/vue',
  },
  {
    value: 'js-svelte',
    label: 'Svelte',
    description: 'Svelte/SvelteKit applications with context API.',
    icon: '/svelte.png',
    href: '/client/svelte',
  },
  {
    value: 'js-generic',
    label: 'JS Generic',
    description: 'Framework-agnostic JavaScript SDK for browsers.',
    icon: '/javascript.png',
    href: '/client/js-sdk',
  },
]

export default function FrameworkPicker() {
  const router = useRouter()
  const { setSdk } = useSdk()

  function handleSelect(fw) {
    setSdk(fw.value)
    router.push(`${fw.href}?sdk=${fw.value}`)
  }

  return (
    <div className="framework-picker">
      <h2 className="framework-picker-heading">Choose your framework</h2>
      <p className="framework-picker-subheading">
        Select the framework you're using to get started with Traceway.
      </p>
      <div className="framework-picker-grid">
        {FRAMEWORKS.map((fw) => (
          <button
            key={fw.value}
            className="framework-picker-card"
            onClick={() => handleSelect(fw)}
          >
            <img src={fw.icon} alt={fw.label} className="framework-picker-icon" />
            <span className="framework-picker-label">{fw.label}</span>
            <span className="framework-picker-desc">{fw.description}</span>
          </button>
        ))}
      </div>
    </div>
  )
}
