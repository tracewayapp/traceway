import { useEffect } from 'react'
import { useRouter } from 'next/router'
import { JetBrains_Mono, IBM_Plex_Mono, Inter } from 'next/font/google'
import '../styles/custom.css'
import { SdkProvider } from '../components/SdkContext'

const jetbrainsMono = JetBrains_Mono({
  variable: '--font-display',
  subsets: ['latin'],
  weight: ['400', '500', '600', '700'],
  display: 'swap',
})

const ibmPlexMono = IBM_Plex_Mono({
  variable: '--font-mono',
  subsets: ['latin'],
  weight: ['400', '500', '600'],
  display: 'swap',
})

const inter = Inter({
  variable: '--font-body',
  subsets: ['latin'],
  display: 'swap',
})

export default function App({ Component, pageProps }) {
  const router = useRouter()

  useEffect(() => {
    const path = router.pathname.replace(/\//g, '-').replace(/^-/, '') || 'index'
    document.body.setAttribute('data-page', path)
    // lock to dark; Nextra's forcedTheme is respected but belt-and-suspenders
    document.documentElement.classList.add('dark')
    document.documentElement.style.colorScheme = 'dark'
  }, [router.pathname])

  return (
    <div className={`docs-root ${jetbrainsMono.variable} ${ibmPlexMono.variable} ${inter.variable}`}>
      <SdkProvider>
        <Component {...pageProps} />
      </SdkProvider>
    </div>
  )
}
