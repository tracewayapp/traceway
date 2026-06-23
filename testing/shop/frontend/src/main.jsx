import React from 'react'
import { createRoot } from 'react-dom/client'
import { TracewayProvider } from '@tracewayapp/react'
import App from './App.jsx'
import './styles.css'

createRoot(document.getElementById('root')).render(
  <TracewayProvider
    connectionString={import.meta.env.VITE_TW_CONNECTION}
    options={{
      version: '0.1.0',
      sessionRecording: true,
      recordAllSessions: true
    }}
  >
    <App />
  </TracewayProvider>
)
