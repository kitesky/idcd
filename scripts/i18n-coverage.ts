#!/usr/bin/env tsx
/**
 * i18n 翻译覆盖率（Phase 5.3）。
 *
 * 输出 `locale × namespace` 的 key 覆盖率矩阵，作为 CI 软门禁：
 *   - default locale 缺 key → error（exit 1）
 *   - 其他 locale 缺 key   → warning（stderr，不退出非零）
 *   - admin namespace 非 default locale 缺失豁免（fallback chain 处理）
 *
 * CLI:
 *   --strict     把 warning 升级为 error
 *   --json       输出 JSON 而非 ASCII 表格
 *   --self-test  内置 fixture 自检（不读真实文件）
 *
 * 退出码：
 *   0  全部 default key 完整（warning 不影响）
 *   1  default locale 缺 key / parse 错误 / --strict 下有 warning
 *   2  registry / fixture 加载失败
 */

import { readFileSync, readdirSync, existsSync } from 'node:fs'
import { resolve, join } from 'node:path'

const ROOT = resolve(import.meta.dirname, '..')
const REGISTRY_PATH = join(ROOT, 'config/locales.json')
const MESSAGES_DIR = join(ROOT, 'apps/web/src/i18n/messages')

// 与 lint-i18n.ts 对齐：admin 在非 default locale 走 fallback，不计入覆盖率。
const FALLBACK_ONLY_NAMESPACES = new Set(['admin'])

interface LocaleEntry {
  code: string
  bcp47: string
  fallback: string[]
}

interface Registry {
  default: string
  locales: LocaleEntry[]
}

interface NamespaceCoverage {
  namespace: string
  total: number
  translated: Record<string, number>
  missing: Record<string, string[]>
  exempt: Record<string, boolean>
}

interface CoverageResult {
  defaultLocale: string
  locales: string[]
  namespaces: NamespaceCoverage[]
  errors: string[]
  warnings: string[]
  totals: Record<string, { translated: number; total: number }>
}

// ── CLI ──────────────────────────────────────────────────────────────────────

const args = new Set(process.argv.slice(2))
const FLAG_STRICT = args.has('--strict')
const FLAG_JSON = args.has('--json')
const FLAG_SELF_TEST = args.has('--self-test')

// ── 工具 ─────────────────────────────────────────────────────────────────────

function collectKeys(obj: unknown, prefix: string, out: Set<string>): void {
  if (obj === null || obj === undefined) return
  if (typeof obj !== 'object') {
    out.add(prefix)
    return
  }
  for (const [k, v] of Object.entries(obj as Record<string, unknown>)) {
    const next = prefix ? `${prefix}.${k}` : k
    collectKeys(v, next, out)
  }
}

function parseJsonFile(path: string): unknown {
  return JSON.parse(readFileSync(path, 'utf8'))
}

function readRegistry(path: string): Registry {
  if (!existsSync(path)) {
    console.error(`[i18n-coverage] registry not found: ${path}`)
    process.exit(2)
  }
  const raw = readFileSync(path, 'utf8')
  let parsed: Registry
  try {
    parsed = JSON.parse(raw) as Registry
  } catch (e) {
    console.error(`[i18n-coverage] invalid JSON in registry: ${(e as Error).message}`)
    process.exit(2)
  }
  if (!parsed.default || !Array.isArray(parsed.locales) || parsed.locales.length === 0) {
    console.error(`[i18n-coverage] malformed registry (default/locales)`)
    process.exit(2)
  }
  if (!parsed.locales.some((l) => l.code === parsed.default)) {
    console.error(`[i18n-coverage] default "${parsed.default}" not in locales[]`)
    process.exit(2)
  }
  return parsed
}

function listNamespaces(localeDir: string): string[] {
  if (!existsSync(localeDir)) return []
  return readdirSync(localeDir)
    .filter((f) => f.endsWith('.json'))
    .map((f) => f.replace(/\.json$/, ''))
    .sort()
}

// ── 核心：计算覆盖率 ─────────────────────────────────────────────────────────

interface ComputeInput {
  registry: Registry
  /** 读取 (locale, namespace) → 扁平化 key 集合；null 表示文件不存在；undefined 表示 parse 失败。 */
  readKeys: (locale: string, namespace: string) => Set<string> | null | undefined
  /** 枚举 default locale 下的 namespace 列表 */
  listDefaultNamespaces: () => string[]
}

function computeCoverage(input: ComputeInput): CoverageResult {
  const { registry, readKeys, listDefaultNamespaces } = input
  const defaultLocale = registry.default
  const localeCodes = registry.locales.map((l) => l.code)

  const errors: string[] = []
  const warnings: string[] = []
  const namespaces: NamespaceCoverage[] = []
  const totals: Record<string, { translated: number; total: number }> = {}
  for (const code of localeCodes) totals[code] = { translated: 0, total: 0 }

  const defaultNamespaces = listDefaultNamespaces()
  if (defaultNamespaces.length === 0) {
    errors.push(`默认 locale (${defaultLocale}) 下无任何 namespace JSON 文件`)
  }

  for (const ns of defaultNamespaces) {
    const defaultKeys = readKeys(defaultLocale, ns)
    if (defaultKeys === undefined) {
      errors.push(`${defaultLocale}/${ns}.json parse 失败`)
      continue
    }
    if (defaultKeys === null) {
      // default 的 namespace 不存在（理论不可能，因为是枚举出来的）—— 视为 error
      errors.push(`${defaultLocale}/${ns}.json 缺失`)
      continue
    }

    const cov: NamespaceCoverage = {
      namespace: ns,
      total: defaultKeys.size,
      translated: {},
      missing: {},
      exempt: {},
    }

    for (const locale of localeCodes) {
      if (locale === defaultLocale) {
        cov.translated[locale] = defaultKeys.size
        cov.missing[locale] = []
        totals[locale]!.total += defaultKeys.size
        totals[locale]!.translated += defaultKeys.size
        continue
      }

      // admin 在非 default locale 走 fallback chain，整体豁免
      if (FALLBACK_ONLY_NAMESPACES.has(ns)) {
        cov.exempt[locale] = true
        cov.translated[locale] = defaultKeys.size // 视作 100%（fallback 兜底）
        cov.missing[locale] = []
        totals[locale]!.total += defaultKeys.size
        totals[locale]!.translated += defaultKeys.size
        continue
      }

      const localeKeys = readKeys(locale, ns)
      if (localeKeys === undefined) {
        errors.push(`${locale}/${ns}.json parse 失败`)
        cov.translated[locale] = 0
        cov.missing[locale] = []
        totals[locale]!.total += defaultKeys.size
        continue
      }
      if (localeKeys === null) {
        // 整个 namespace 文件不存在
        warnings.push(`${locale}/${ns}.json 缺失（默认 locale 提供了此 namespace）`)
        cov.translated[locale] = 0
        cov.missing[locale] = [...defaultKeys].sort()
        totals[locale]!.total += defaultKeys.size
        continue
      }

      const missing = [...defaultKeys].filter((k) => !localeKeys.has(k)).sort()
      const translatedCount = defaultKeys.size - missing.length
      cov.translated[locale] = translatedCount
      cov.missing[locale] = missing
      totals[locale]!.total += defaultKeys.size
      totals[locale]!.translated += translatedCount

      if (missing.length > 0) {
        warnings.push(
          `${locale}/${ns}.json 缺 ${missing.length}/${defaultKeys.size} key：` +
            missing.slice(0, 5).join(', ') +
            (missing.length > 5 ? ` …(+${missing.length - 5})` : ''),
        )
      }
    }

    namespaces.push(cov)
  }

  return {
    defaultLocale,
    locales: localeCodes,
    namespaces,
    errors,
    warnings,
    totals,
  }
}

// ── 输出：ASCII 表格 ────────────────────────────────────────────────────────

function pct(translated: number, total: number): string {
  if (total === 0) return '  -  '
  const ratio = (translated / total) * 100
  return ratio.toFixed(0).padStart(3, ' ') + '%'
}

function cell(translated: number, total: number, exempt: boolean): string {
  if (exempt) return ' fallback'
  if (total === 0) return '   -   '
  return `${translated}/${total} (${pct(translated, total).trim()})`
}

function renderTable(result: CoverageResult): string {
  const { defaultLocale, locales, namespaces, totals } = result
  const cols = ['namespace', ...locales]
  const rows: string[][] = []

  for (const cov of namespaces) {
    const row = [cov.namespace]
    for (const locale of locales) {
      row.push(cell(cov.translated[locale] ?? 0, cov.total, !!cov.exempt[locale]))
    }
    rows.push(row)
  }

  // totals row
  const totalsRow = ['TOTAL']
  for (const locale of locales) {
    const t = totals[locale]!
    totalsRow.push(`${t.translated}/${t.total} (${pct(t.translated, t.total).trim()})`)
  }
  rows.push(totalsRow)

  // 计算列宽
  const widths = cols.map((h, i) => {
    let w = h.length
    for (const r of rows) w = Math.max(w, (r[i] ?? '').length)
    return w
  })

  const fmt = (cells: string[]): string =>
    cells.map((c, i) => c.padEnd(widths[i]!)).join('  ')

  const out: string[] = []
  out.push(fmt(cols))
  out.push(widths.map((w) => '─'.repeat(w)).join('  '))
  for (const r of rows.slice(0, -1)) out.push(fmt(r))
  out.push(widths.map((w) => '─'.repeat(w)).join('  '))
  out.push(fmt(rows[rows.length - 1]!))

  out.push('')
  out.push(`default locale: ${defaultLocale}   (cells = translated/total (%))`)
  out.push(`fallback = admin namespace via fallback chain (D2/D3 i18n plan)`)

  return out.join('\n')
}

// ── 输出：JSON ───────────────────────────────────────────────────────────────

function renderJson(result: CoverageResult): string {
  return JSON.stringify(
    {
      defaultLocale: result.defaultLocale,
      locales: result.locales,
      totals: result.totals,
      namespaces: result.namespaces.map((n) => ({
        namespace: n.namespace,
        total: n.total,
        translated: n.translated,
        missing: n.missing,
        exempt: n.exempt,
      })),
      errors: result.errors,
      warnings: result.warnings,
    },
    null,
    2,
  )
}

// ── 真实文件加载 + 主入口 ───────────────────────────────────────────────────

function runReal(): CoverageResult {
  const registry = readRegistry(REGISTRY_PATH)
  const defaultLocale = registry.default

  const readKeys: ComputeInput['readKeys'] = (locale, ns) => {
    const file = join(MESSAGES_DIR, locale, `${ns}.json`)
    if (!existsSync(file)) return null
    try {
      const parsed = parseJsonFile(file)
      const keys = new Set<string>()
      collectKeys(parsed, '', keys)
      return keys
    } catch {
      return undefined
    }
  }

  const listDefaultNamespaces = (): string[] => {
    return listNamespaces(join(MESSAGES_DIR, defaultLocale))
  }

  return computeCoverage({ registry, readKeys, listDefaultNamespaces })
}

// ── Self-test ────────────────────────────────────────────────────────────────

function runSelfTest(): void {
  const registry: Registry = {
    default: 'cn',
    locales: [
      { code: 'cn', bcp47: 'zh-CN', fallback: [] },
      { code: 'en', bcp47: 'en-US', fallback: [] },
      { code: 'ja', bcp47: 'ja-JP', fallback: [] },
    ],
  }

  const fixtures: Record<string, Record<string, unknown>> = {
    cn: {
      common: { hello: 'a', nav: { home: 'a', about: 'a' } },
      admin: { dashboard: 'a' },
      home: { hero: { title: 'a', sub: 'a' } },
    },
    en: {
      common: { hello: 'a', nav: { home: 'a' } }, // 缺 nav.about → warning
      // admin 缺失 → exempt (fallback)
      home: { hero: { title: 'a', sub: 'a' } },
    },
    ja: {
      common: { hello: 'a', nav: { home: 'a', about: 'a' } },
      home: { hero: { title: 'a' } }, // 缺 hero.sub → warning
    },
  }

  const readKeys: ComputeInput['readKeys'] = (locale, ns) => {
    const file = fixtures[locale]?.[ns]
    if (file === undefined) return null
    const keys = new Set<string>()
    collectKeys(file, '', keys)
    return keys
  }
  const listDefaultNamespaces = (): string[] => Object.keys(fixtures.cn!).sort()

  const result = computeCoverage({ registry, readKeys, listDefaultNamespaces })

  const failures: string[] = []
  const expect = (cond: boolean, msg: string) => {
    if (!cond) failures.push(msg)
  }

  // 默认 locale 满覆盖
  expect(result.totals.cn!.translated === result.totals.cn!.total, 'cn 应该 100%')
  // en common 应该是 2/3
  const commonCov = result.namespaces.find((n) => n.namespace === 'common')!
  expect(commonCov.translated.en === 2 && commonCov.total === 3, `en/common 应是 2/3, 实际 ${commonCov.translated.en}/${commonCov.total}`)
  // en admin 应该是 exempt
  const adminCov = result.namespaces.find((n) => n.namespace === 'admin')!
  expect(adminCov.exempt.en === true, 'en/admin 应该 exempt')
  expect(adminCov.exempt.ja === true, 'ja/admin 应该 exempt')
  // ja home 缺 hero.sub
  const homeCov = result.namespaces.find((n) => n.namespace === 'home')!
  expect(homeCov.translated.ja === 1 && homeCov.total === 2, `ja/home 应是 1/2, 实际 ${homeCov.translated.ja}/${homeCov.total}`)
  // errors 为空（default 完整）
  expect(result.errors.length === 0, `errors 应为空, 实际 ${JSON.stringify(result.errors)}`)
  // warnings 应包含 en/common 与 ja/home
  const w = result.warnings.join('\n')
  expect(/en\/common/.test(w), 'warnings 应包含 en/common')
  expect(/ja\/home/.test(w), 'warnings 应包含 ja/home')

  // 渲染不抛
  const table = renderTable(result)
  expect(table.includes('common'), '表格应含 common 行')
  expect(table.includes('TOTAL'), '表格应含 TOTAL 行')
  const json = renderJson(result)
  expect(json.includes('"defaultLocale": "cn"'), 'JSON 应含 defaultLocale')

  if (failures.length > 0) {
    console.error(`[i18n-coverage --self-test] FAIL (${failures.length})`)
    for (const f of failures) console.error(`  - ${f}`)
    process.exit(1)
  }
  console.log('[i18n-coverage --self-test] OK')
  process.exit(0)
}

// ── main ────────────────────────────────────────────────────────────────────

function main(): void {
  if (FLAG_SELF_TEST) {
    runSelfTest()
    return
  }

  const result = runReal()

  if (FLAG_JSON) {
    console.log(renderJson(result))
  } else {
    console.log(renderTable(result))
  }

  // errors 总是阻塞
  if (result.errors.length > 0) {
    console.error('')
    console.error(`[i18n-coverage] ERROR (${result.errors.length}) — default locale 不完整：`)
    for (const e of result.errors) console.error(`  - ${e}`)
    process.exit(1)
  }

  // warnings
  if (result.warnings.length > 0) {
    console.error('')
    console.error(`[i18n-coverage] WARNING (${result.warnings.length}) — 非 default locale 缺 key：`)
    for (const w of result.warnings) console.error(`  - ${w}`)
    if (FLAG_STRICT) {
      console.error('')
      console.error(`[i18n-coverage] --strict: warning 视为 error → exit 1`)
      process.exit(1)
    }
  } else {
    console.log('')
    console.log('[i18n-coverage] OK — 所有 locale 100% 覆盖')
  }
  process.exit(0)
}

main()
