import tailwindcss from '@tailwindcss/vite'
import { tanstackRouter } from '@tanstack/router-plugin/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vitest/config'

const DEV_PORT = 5173
const API_PREFIX = '/api'
const API_TARGET = 'http://127.0.0.1:8080'
// The Go binary embeds the built app from here (see api/internal/webui).
const OUT_DIR = '../api/internal/webui/dist'
// Tests render the whole app; with every file running in parallel a long form test can take
// several times its solo run, so the 5s default is too tight.
const TEST_TIMEOUT_MS = 15_000

export default defineConfig({
  plugins: [tanstackRouter({ target: 'react', autoCodeSplitting: true }), react(), tailwindcss()],
  server: {
    port: DEV_PORT,
    strictPort: true,
    // changeOrigin stays false so the API sees the browser's Host header,
    // which its cross-origin (CSRF) protection compares against Origin.
    proxy: { [API_PREFIX]: { target: API_TARGET, changeOrigin: false } },
  },
  build: { outDir: OUT_DIR, emptyOutDir: true },
  test: {
    environment: 'jsdom',
    testTimeout: TEST_TIMEOUT_MS,
    setupFiles: ['./src/test/setup.ts'],
    css: false,
  },
})
