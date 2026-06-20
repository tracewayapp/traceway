import React from 'react'
import { createRoot } from 'react-dom/client'
import { TracewayProvider } from '@tracewayapp/react'
import App from './App.jsx'
import './styles.css'

const connectionString =
  import.meta.env.VITE_TW_CONNECTION ||
  'default_token_change_me@http://localhost:8082/api/report'

createRoot(document.getElementById('root')).render(
  <TracewayProvider connectionString={connectionString} options={{ debug: true, version: '0.1.0' }}>
    <App />
  </TracewayProvider>
)
