import { useEffect } from 'react'
import { useRouter } from 'next/router'
import '../styles/custom.css'
import { SdkProvider } from '../components/SdkContext'

export default function App({ Component, pageProps }) {
  const router = useRouter()

  useEffect(() => {
    const path = router.pathname.replace(/\//g, '-').replace(/^-/, '') || 'index'
    document.body.setAttribute('data-page', path)
  }, [router.pathname])

  return (
    <SdkProvider>
      <Component {...pageProps} />
    </SdkProvider>
  )
}
