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
