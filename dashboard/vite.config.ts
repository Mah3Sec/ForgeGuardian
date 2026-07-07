import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { fileURLToPath, URL } from 'node:url'

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    port: 3000,
    proxy: {
      // Prefix-matched by Vite — must be '/api/' (trailing slash), not '/api',
      // or it also catches client-side routes like /api-docs and proxies
      // them to the backend instead of serving the SPA.
      '/api/': {
        target: process.env.VITE_API_URL || 'http://localhost:8080',
        changeOrigin: true,
        proxyTimeout: 10000,
        configure: (proxy) => {
          proxy.on('error', (err, _req, _res) => {
            console.warn('[vite proxy] API unreachable:', err.message);
          });
        },
      },
    },
  },
})
