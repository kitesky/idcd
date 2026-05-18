import { defineConfig, devices } from '@playwright/test'

/**
 * Playwright E2E config. Tuned for low memory pressure:
 * - single worker, no fullyParallel
 * - chromium only
 * - trace/video only on first retry to keep artifacts small
 * - authenticate once via a setup project; reuse storageState across app tests
 *   to avoid hitting the auth login rate-limit on every test.
 */
export default defineConfig({
  testDir: './tests/e2e',
  timeout: 30_000,
  expect: { timeout: 8_000 },
  fullyParallel: false,
  workers: 1,
  retries: 0,
  reporter: [
    ['list'],
    ['html', { outputFolder: 'playwright-report', open: 'never' }],
    ['json', { outputFile: 'playwright-report/results.json' }],
  ],
  use: {
    baseURL: process.env.BASE_URL ?? 'http://localhost:3000',
    headless: true,
    locale: 'zh-CN',
    timezoneId: 'Asia/Shanghai',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
    actionTimeout: 8_000,
    navigationTimeout: 20_000,
  },
  projects: [
    {
      name: 'setup',
      testMatch: /auth\.setup\.ts/,
    },
    {
      name: 'chromium-public',
      testMatch: /(public|tools|diagnose|probe-ping|diagnose-flow)\.spec\.ts/,
      use: { ...devices['Desktop Chrome'] },
    },
    {
      name: 'chromium-app',
      testMatch: /(app|api-key-create|monitor-list)\.spec\.ts/,
      use: {
        ...devices['Desktop Chrome'],
        storageState: 'tests/e2e/.auth/user.json',
      },
      dependencies: ['setup'],
    },
  ],
  outputDir: 'test-results',
})
