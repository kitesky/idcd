import '@testing-library/jest-dom'
import { vi } from 'vitest'

// ---------------------------------------------------------------------------
// Global mock for next-intl — returns zh translation strings so tests
// keep passing without a full next-intl provider setup.
// ---------------------------------------------------------------------------
import zhAuth from '@/i18n/messages/zh/auth.json'
import zhCommon from '@/i18n/messages/zh/common.json'
import zhErrors from '@/i18n/messages/zh/errors.json'
import zhNav from '@/i18n/messages/zh/nav.json'
import zhHome from '@/i18n/messages/zh/home.json'
import zhTools from '@/i18n/messages/zh/tools.json'
import zhLeaderboard from '@/i18n/messages/zh/leaderboard.json'
import zhNodes from '@/i18n/messages/zh/nodes.json'
import zhPricing from '@/i18n/messages/zh/pricing.json'
import zhMonitors from '@/i18n/messages/zh/monitors.json'
import zhAlerts from '@/i18n/messages/zh/alerts.json'
import zhSettings from '@/i18n/messages/zh/settings.json'
import zhBilling from '@/i18n/messages/zh/billing.json'
import zhDashboard from '@/i18n/messages/zh/dashboard.json'
import zhStatus from '@/i18n/messages/zh/status.json'
import zhAdmin from '@/i18n/messages/zh/admin.json'

type NestedRecord = { [key: string]: string | NestedRecord }

function makeTranslator(messages: NestedRecord) {
  return function t(key: string, params?: Record<string, string | number>): string {
    const parts = key.split('.')
    let value: string | NestedRecord | undefined = messages
    for (const part of parts) {
      if (typeof value !== 'object' || value === null) return key
      value = (value as NestedRecord)[part]
    }
    if (typeof value !== 'string') return key
    if (params) {
      return value.replace(/\{(\w+)\}/g, (_, k) => String(params[k] ?? `{${k}}`))
    }
    return value
  }
}

const ALL_MESSAGES: Record<string, NestedRecord> = {
  auth: zhAuth as unknown as NestedRecord,
  common: zhCommon as unknown as NestedRecord,
  errors: zhErrors as unknown as NestedRecord,
  nav: zhNav as unknown as NestedRecord,
  home: zhHome as unknown as NestedRecord,
  tools: zhTools as unknown as NestedRecord,
  leaderboard: zhLeaderboard as unknown as NestedRecord,
  nodes: zhNodes as unknown as NestedRecord,
  pricing: zhPricing as unknown as NestedRecord,
  monitors: zhMonitors as unknown as NestedRecord,
  alerts: zhAlerts as unknown as NestedRecord,
  settings: zhSettings as unknown as NestedRecord,
  billing: zhBilling as unknown as NestedRecord,
  dashboard: zhDashboard as unknown as NestedRecord,
  status: zhStatus as unknown as NestedRecord,
  admin: zhAdmin as unknown as NestedRecord,
}

vi.mock('next-intl', () => ({
  useTranslations: (namespace?: string) => {
    const ns = namespace && ALL_MESSAGES[namespace] ? ALL_MESSAGES[namespace] : {}
    return makeTranslator(ns as NestedRecord)
  },
  useLocale: () => 'zh',
  useNow: () => new Date(),
  useTimeZone: () => 'Asia/Shanghai',
  NextIntlClientProvider: ({ children }: { children: React.ReactNode }) => children,
}))

vi.mock('next-intl/server', () => ({
  getTranslations: async ({ namespace }: { locale?: string; namespace?: string } = {}) => {
    const ns = namespace && ALL_MESSAGES[namespace] ? ALL_MESSAGES[namespace] : {}
    return makeTranslator(ns as NestedRecord)
  },
  getLocale: async () => 'zh',
  getMessages: async () => ALL_MESSAGES,
  getRequestConfig: (fn: unknown) => fn,
}))

vi.mock('@/i18n/locale', () => ({
  getLocale: async () => 'zh',
  getLocaleCookie: async () => 'zh',
}))

// Mock ResizeObserver for jsdom (required by Radix UI Slider and other components)
global.ResizeObserver = class ResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}

// Mock EventSource for jsdom (not available in test environment)
class MockEventSource {
  static CONNECTING = 0
  static OPEN = 1
  static CLOSED = 2
  readyState = MockEventSource.OPEN
  url: string
  withCredentials: boolean
  onopen: (() => void) | null = null
  onerror: (() => void) | null = null
  onmessage: (() => void) | null = null
  constructor(url: string, init?: { withCredentials?: boolean }) {
    this.url = url
    this.withCredentials = init?.withCredentials ?? false
  }
  addEventListener() {}
  removeEventListener() {}
  dispatchEvent() { return true }
  close() {}
}
// @ts-expect-error jsdom does not define EventSource
global.EventSource = MockEventSource

// Mock matchMedia for jsdom
Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: vi.fn().mockImplementation((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: vi.fn(), // deprecated
    removeListener: vi.fn(), // deprecated
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  })),
})
