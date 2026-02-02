import '../styles/custom.css'
import { SdkProvider } from '../components/SdkContext'

export default function App({ Component, pageProps }) {
  return (
    <SdkProvider>
      <Component {...pageProps} />
    </SdkProvider>
  )
}
