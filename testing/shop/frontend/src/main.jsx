import React from 'react'
import { createRoot } from 'react-dom/client'
import { TracewayProvider } from '@tracewayapp/react'
import App from './App.jsx'
import './styles.css'

createRoot(document.getElementById('root')).render(
  <TracewayProvider
    connectionString={import.meta.env.VITE_TW_CONNECTION}
    options={{
      // The end-to-end checks ship a second release to compare against the first.
      version: window.__DEMO_VERSION ?? '0.1.0',
      sessionRecording: true,
      recordAllSessions: true
    }}
  >
    <App />
  </TracewayProvider>
)
