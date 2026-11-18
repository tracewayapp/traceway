import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import { TracewayProvider } from '@tracewayapp/react'
import { App } from './App'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <TracewayProvider
    connectionString={import.meta.env.VITE_TRACEWAY_CONNECTION_STRING}
    options={{
      version: '2026.10.14',
      // Keep the defaults except timeouts: a search that times out is a
      // shopper who gave up, and we want to see those.
      ignoreErrors: ['ResizeObserver loop', 'Failed to fetch', 'Load failed', 'NetworkError when attempting to fetch resource'],
    }}
  >
    <BrowserRouter>
      <App />
    </BrowserRouter>
  </TracewayProvider>,
)
