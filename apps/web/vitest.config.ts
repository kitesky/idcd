import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import { resolve } from 'path'

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    pool: 'forks',
    poolOptions: {
      forks: {
        maxForks: 2,
        minForks: 1,
      },
    },
    env: {
      // Set polling timeout to 0 so probe poll loop exits immediately in tests.
      // Tests mock /v1/probe/tasks/ with 404; without this the suite would hang.
      PROBE_POLL_TIMEOUT_MS: '0',
      PROBE_POLL_INTERVAL_MS: '0',
    },
  },
  resolve: {
    alias: {
      '@': resolve(__dirname, './src')
    }
  }
})