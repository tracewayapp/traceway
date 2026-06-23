import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { tracewayDebugIds } from '@tracewayapp/bundler-plugin/vite'

export default defineConfig({
  build: {
    sourcemap: true
  },
  server: {
    port: 5175,
    proxy: {
      '/api': 'http://localhost:8090'
    }
  },
  plugins: [react(), tracewayDebugIds()]
})
