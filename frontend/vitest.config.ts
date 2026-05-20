import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import { resolve } from 'path'

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    // 默认 include 的 *.spec.ts 会把 tests/e2e/* (Playwright) 一起扫进来,
    // 触发 "Playwright Test did not expect test() to be called" 误报。
    include: ['src/**/*.{test,spec}.{ts,tsx,js,jsx}'],
    exclude: ['node_modules', '.next', '.open-next', '.wrangler', 'tests/e2e/**'],
    pool: 'forks',
    // vitest 4 移除 InlineConfig.poolOptions；并发上限改为顶层 maxWorkers。
    maxWorkers: 2,
    env: {
      // Set polling timeout to 0 so probe poll loop exits immediately in tests.
      // Tests mock /v1/probe/tasks/ with 404; without this the suite would hang.
      PROBE_POLL_TIMEOUT_MS: '0',
      PROBE_POLL_INTERVAL_MS: '0',
    },
  },
  resolve: {
    alias: {
      '@': resolve(__dirname, './src'),
    }
  }
})