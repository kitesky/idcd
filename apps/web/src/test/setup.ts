import '@testing-library/jest-dom'
import { vi } from 'vitest'

// ---------------------------------------------------------------------------
// Global mock for next-intl — returns cn translation strings so tests
// keep passing without a full next-intl provider setup.
// ---------------------------------------------------------------------------
import cnAuth from '@/i18n/messages/cn/auth.json'
import cnCommon from '@/i18n/messages/cn/common.json'
import cnErrors from '@/i18n/messages/cn/errors.json'
import cnNav from '@/i18n/messages/cn/nav.json'
import cnHome from '@/i18n/messages/cn/home.json'
import cnTools from '@/i18n/messages/cn/tools.json'
import cnLeaderboard from '@/i18n/messages/cn/leaderboard.json'
import cnNodes from '@/i18n/messages/cn/nodes.json'
import cnPricing from '@/i18n/messages/cn/pricing.json'
import cnMonitors from '@/i18n/messages/cn/monitors.json'
import cnAlerts from '@/i18n/messages/cn/alerts.json'
import cnSettings from '@/i18n/messages/cn/settings.json'
import cnBilling from '@/i18n/messages/cn/billing.json'
import cnDashboard from '@/i18n/messages/cn/dashboard.json'
import cnStatus from '@/i18n/messages/cn/status.json'
import cnAdmin from '@/i18n/messages/cn/admin.json'

type NestedValue = string | NestedRecord | NestedValue[]
type NestedRecord = { [key: string]: NestedValue }

function lookupPath(messages: NestedRecord, key: string): NestedValue | undefined {
  const parts = key.split('.')
  let value: NestedValue | undefined = messages
  for (const part of parts) {
    if (value === null || value === undefined) return undefined
    if (Array.isArray(value)) {
      const idx = Number(part)
      if (!Number.isInteger(idx)) return undefined
      value = value[idx]
      continue
    }
    if (typeof value !== 'object') return undefined
    value = (value as NestedRecord)[part]
  }
  return value
}

interface TranslatorFn {
  (key: string, params?: Record<string, string | number>): string
  raw: (key: string) => NestedValue | undefined
}

function makeTranslator(messages: NestedRecord): TranslatorFn {
  const t = function (key: string, params?: Record<string, string | number>): string {
    const value = lookupPath(messages, key)
    if (typeof value !== 'string') return key
    if (params) {
      return value.replace(/\{(\w+)\}/g, (_, k) => String(params[k] ?? `{${k}}`))
    }
    return value
  } as TranslatorFn
  t.raw = (key: string) => lookupPath(messages, key)
  return t
}

function resolveNamespace(namespace?: string): NestedRecord {
  if (!namespace) return {}
  const [top, ...rest] = namespace.split('.')
  const root = top && ALL_MESSAGES[top]
  if (!root) return {}
  if (rest.length === 0) return root
  const sub = lookupPath(root, rest.join('.'))
  return sub && typeof sub === 'object' && !Array.isArray(sub) ? (sub as NestedRecord) : {}
}

const ALL_MESSAGES: Record<string, NestedRecord> = {
  auth: cnAuth as unknown as NestedRecord,
  common: cnCommon as unknown as NestedRecord,
  errors: cnErrors as unknown as NestedRecord,
  nav: cnNav as unknown as NestedRecord,
  home: cnHome as unknown as NestedRecord,
  tools: cnTools as unknown as NestedRecord,
  leaderboard: cnLeaderboard as unknown as NestedRecord,
  nodes: cnNodes as unknown as NestedRecord,
  pricing: cnPricing as unknown as NestedRecord,
  monitors: cnMonitors as unknown as NestedRecord,
  alerts: cnAlerts as unknown as NestedRecord,
  settings: cnSettings as unknown as NestedRecord,
  billing: cnBilling as unknown as NestedRecord,
  dashboard: cnDashboard as unknown as NestedRecord,
  status: cnStatus as unknown as NestedRecord,
  admin: cnAdmin as unknown as NestedRecord,
}

vi.mock('next-intl', () => ({
  useTranslations: (namespace?: string) => makeTranslator(resolveNamespace(namespace)),
  useLocale: () => 'cn',
  useNow: () => new Date(),
  useTimeZone: () => 'Asia/Shanghai',
  NextIntlClientProvider: ({ children }: { children: React.ReactNode }) => children,
}))

vi.mock('next-intl/server', () => ({
  getTranslations: async ({ namespace }: { locale?: string; namespace?: string } = {}) =>
    makeTranslator(resolveNamespace(namespace)),
  getLocale: async () => 'cn',
  getMessages: async () => ALL_MESSAGES,
  getRequestConfig: (fn: unknown) => fn,
}))

vi.mock('@/i18n/locale', () => ({
  getLocale: async () => 'cn',
  getLocaleCookie: async () => 'cn',
  LOCALE_COOKIE_NAME: 'idcd_locale',
  LEGACY_LOCALE_COOKIE_NAME: 'locale',
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
