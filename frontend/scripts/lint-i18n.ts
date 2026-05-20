#!/usr/bin/env tsx
/**
 * i18n CI lint.
 *
 * 强制规则（详 docs/prd/I18N-PLAN.md §Phase 5.1）：
 *
 *   1. 完整性：default locale 的每个 namespace 都必须存在
 *   2. Key 一致性：每个 namespace 在所有 locale 的 key 集合等于 default locale
 *      （admin namespace 在非 default locale 走 fallback，跳过）
 *   3. 前后端 errcode 对齐：errcode/codes.go ↔ messages/{locale}/errors.json
 *   4. 禁止二元 locale 判断：`locale === 'en'` / `locale === 'cn'` 等
 *   5. 禁止 binary locale map：`{ cn: 'x', en: 'y' }`
 *   6. HTML lang 必须 BCP47：`<html lang={bcp47Of(locale)}>` 不允许直接传 locale code
 *   7. Dead key 检测：messages JSON 有但源码里没 t() 引用的 key
 *      仅在 --check-dead 显式开启（regex scanner 误报率高 — 跨文件 t 传递 +
 *      动态 t(`${x}.y`) 都看不到，作为 advisory 工具单独跑：
 *      `pnpm --filter @idcd/web i18n:dead-keys` 包装了 --check-dead 调用）
 *
 * 退出码：
 *   0  全部通过（含 warnings）
 *   1  有 error 级别违规
 */

import { execSync } from 'node:child_process'
import { readFileSync, readdirSync, existsSync } from 'node:fs'
import { resolve, join } from 'node:path'
import { scanProject, isCoveredByDynamic } from './_i18n-scan.js'

const ROOT = resolve(import.meta.dirname, '..')
const REGISTRY_PATH = join(ROOT, '../backend/config/locales.json')
const MESSAGES_DIR = join(ROOT, 'src/i18n/messages')
const ERRCODE_PATH = join(ROOT, 'lib/shared/errcode/codes.go')
const SCAN_DIRS = [
  join(ROOT, 'src/app'),
  join(ROOT, 'src/components'),
  join(ROOT, 'src/lib'),
]

// Admin 在非 default locale 走 fallback chain，不要求 key 集合等于 default。
const FALLBACK_ONLY_NAMESPACES = new Set(['admin'])

// Rule 7 豁免：namespace 整体跳过 dead-key 检测。
// - nav: 大量 key 通过 sidebar-data.ts 等配置数组传给 t(item.title)，scanner
//   看不到，会全部误报为 dead
// - errors: 后端 errcode 决定 key 集合（已由 Rule 3 校验），前端不一定每个 code
//   都有显式 t() 调用
const DEAD_KEY_EXEMPT_NAMESPACES = new Set(['nav', 'errors'])

// CLI flags
const STRICT = process.argv.includes('--strict')
const CHECK_DEAD = process.argv.includes('--check-dead') || STRICT

interface Registry {
  default: string
  locales: { code: string }[]
}

interface Violation {
  rule: number
  ruleName: string
  detail: string
  location?: string
  severity: 'error' | 'warning'
}

const violations: Violation[] = []

function report(
  rule: number,
  ruleName: string,
  detail: string,
  location?: string,
  severity: 'error' | 'warning' = 'error',
) {
  violations.push({ rule, ruleName, detail, location, severity })
}

// ── 读取 registry ─────────────────────────────────────────────────────────────

if (!existsSync(REGISTRY_PATH)) {
  console.error(`[lint-i18n] registry not found: ${REGISTRY_PATH}`)
  process.exit(2)
}
const registry: Registry = JSON.parse(readFileSync(REGISTRY_PATH, 'utf8'))
const defaultLocale = registry.default
const allLocales = registry.locales.map((l) => l.code)

if (!allLocales.includes(defaultLocale)) {
  console.error(`[lint-i18n] default locale "${defaultLocale}" missing from locales[]`)
  process.exit(2)
}

// ── Rule 1 + 2: messages JSON 完整性 + key 一致性 ─────────────────────────────

function readJsonNamespaces(locale: string): Set<string> {
  const dir = join(MESSAGES_DIR, locale)
  if (!existsSync(dir)) return new Set()
  return new Set(
    readdirSync(dir)
      .filter((f) => f.endsWith('.json'))
      .map((f) => f.replace(/\.json$/, '')),
  )
}

function collectKeys(obj: unknown, prefix: string, out: Set<string>) {
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

function readNamespaceKeys(locale: string, ns: string): Set<string> | null {
  const file = join(MESSAGES_DIR, locale, `${ns}.json`)
  if (!existsSync(file)) return null
  try {
    const parsed = JSON.parse(readFileSync(file, 'utf8'))
    const keys = new Set<string>()
    collectKeys(parsed, '', keys)
    return keys
  } catch (e) {
    report(1, '完整性', `${locale}/${ns}.json 解析失败: ${(e as Error).message}`)
    return null
  }
}

const defaultNamespaces = readJsonNamespaces(defaultLocale)
if (defaultNamespaces.size === 0) {
  report(1, '完整性', `默认 locale (${defaultLocale}) 下没有任何 namespace JSON 文件`)
}

for (const ns of defaultNamespaces) {
  const defaultKeys = readNamespaceKeys(defaultLocale, ns)
  if (!defaultKeys) continue

  for (const locale of allLocales) {
    if (locale === defaultLocale) continue

    const localeFile = join(MESSAGES_DIR, locale, `${ns}.json`)
    if (!existsSync(localeFile)) {
      // 整个 namespace 缺失：admin 允许（fallback），其他 namespace 警告完整性
      if (!FALLBACK_ONLY_NAMESPACES.has(ns)) {
        report(1, '完整性', `${locale}/${ns}.json 缺失（默认 locale 提供了此 namespace）`)
      }
      continue
    }

    const localeKeys = readNamespaceKeys(locale, ns)
    if (!localeKeys) continue

    if (FALLBACK_ONLY_NAMESPACES.has(ns)) continue

    // 必须 key 集合相等
    const missing = [...defaultKeys].filter((k) => !localeKeys.has(k))
    const extra = [...localeKeys].filter((k) => !defaultKeys.has(k))
    if (missing.length > 0) {
      report(
        2,
        'Key 一致性',
        `${locale}/${ns}.json 缺 key (相对 ${defaultLocale})`,
        missing.slice(0, 10).join(', ') + (missing.length > 10 ? ` …(+${missing.length - 10})` : ''),
      )
    }
    if (extra.length > 0) {
      report(
        2,
        'Key 一致性',
        `${locale}/${ns}.json 多 key (相对 ${defaultLocale})`,
        extra.slice(0, 10).join(', ') + (extra.length > 10 ? ` …(+${extra.length - 10})` : ''),
      )
    }
  }
}

// ── Rule 3: errcode 对齐 ─────────────────────────────────────────────────────

if (existsSync(ERRCODE_PATH)) {
  const errcodeSrc = readFileSync(ERRCODE_PATH, 'utf8')
  // 简单粗暴：抓所有 const xxx Code = "code.xxx" 形式
  const codeMatches = [...errcodeSrc.matchAll(/Code\s*=\s*"([a-z][a-z0-9_.-]+)"/gi)]
  const goCodes = new Set(codeMatches.map((m) => m[1]!).filter((s) => s.includes('.')))

  for (const locale of allLocales) {
    const errFile = join(MESSAGES_DIR, locale, 'errors.json')
    if (!existsSync(errFile)) continue
    try {
      const parsed = JSON.parse(readFileSync(errFile, 'utf8'))
      const jsonCodes = new Set<string>()
      collectKeys(parsed, '', jsonCodes)
      const missing = [...goCodes].filter((c) => !jsonCodes.has(c))
      if (missing.length > 0) {
        report(
          3,
          'errcode 对齐',
          `${locale}/errors.json 缺 Go 端定义的 errcode`,
          missing.slice(0, 10).join(', ') + (missing.length > 10 ? ` …(+${missing.length - 10})` : ''),
        )
      }
    } catch {
      // skip parse failures (caught by rule 1)
    }
  }
}

// ── Rule 4 + 5: 二元 locale 判断 + binary locale map（grep） ────────────────

function gitGrep(pattern: string, paths: string[]): string[] {
  try {
    const result = execSync(
      `git grep -nE ${JSON.stringify(pattern)} -- ${paths.map((p) => JSON.stringify(p)).join(' ')}`,
      { cwd: ROOT, encoding: 'utf8', stdio: ['ignore', 'pipe', 'ignore'] },
    )
    return result.split('\n').filter(Boolean)
  } catch {
    // git grep returns exit 1 if no match — that's the good outcome
    return []
  }
}

// 排除 registry / lint 脚本 / messages JSON 自身 / 测试 / 注释
function notExcluded(line: string): boolean {
  // git grep 行格式：path:lineno:content。取 content 部分判断注释
  const colonIdx = line.indexOf(':', line.indexOf(':') + 1)
  const content = colonIdx > 0 ? line.slice(colonIdx + 1) : line
  const trimmed = content.trimStart()
  // 单行注释或块注释行
  if (trimmed.startsWith('//') || trimmed.startsWith('*') || trimmed.startsWith('/*')) return false

  return (
    !line.includes('src/i18n/registry.ts') &&
    !line.includes('lib/shared/i18n/registry.go') &&
    !line.includes('scripts/lint-i18n.ts') &&
    !line.includes('scripts/sync-locale-registry.ts') &&
    !line.includes('i18n/messages/') &&
    !line.includes('config/locales.json') &&
    !line.includes('docs/prd/') &&
    !line.match(/\.test\.(ts|tsx)/) &&
    !line.match(/__tests__/) &&
    // 大量后端 logger.Field("locale", l) 等不属于二元判断
    !line.match(/zap\.|slog\.|log\./) &&
    // 显式的 acceptLanguageAliases 数组里的 cn/en/zh 标记不算
    !line.includes('acceptLanguageAliases')
  )
}

// Rule 4a: locale === 'en' / locale === 'cn' / locale !== 'en' …
const binaryEq = gitGrep(
  "locale[[:space:]]*[!=]==[[:space:]]*['\\\"](cn|en|ja|ko|zh)['\\\"]",
  ['*.ts', '*.tsx', '*.go'],
).filter(notExcluded)
for (const line of binaryEq) {
  report(4, '禁止二元 locale 判断', line)
}

// Rule 4b: locale.startsWith('en' | 'cn' | 'zh')
const startsWithEN = gitGrep(
  "\\.startsWith\\(['\\\"](en|cn|zh)['\\\"]",
  ['*.ts', '*.tsx', '*.go'],
).filter(notExcluded)
for (const line of startsWithEN) {
  report(4, '禁止 locale.startsWith', line)
}

// Rule 5: { cn: 'x', en: 'y' } binary locale map
const binaryMap = gitGrep(
  "\\{[[:space:]]*(cn|en|zh)[[:space:]]*:[[:space:]]*['\\\"][^'\\\"]+['\\\"][[:space:]]*,[[:space:]]*(cn|en|zh)[[:space:]]*:",
  ['*.ts', '*.tsx'],
).filter(notExcluded)
for (const line of binaryMap) {
  report(5, '禁止 binary locale map', line)
}

// ── Rule 7: Dead key 检测（仅 --check-dead 启用） ─────────────────────────────
// 扫源码所有 useTranslations(ns) + t('key') 调用 → usedKeys
// 与 default locale messages JSON 的所有 key 对比 → messages 有但源码不用 = dead
//
// 已知大量 false positive (regex 看不到动态 t(`${x}.y`) 和跨文件传 t)，
// 所以默认关闭，作为 advisory 工具跑：`pnpm i18n:dead-keys`

if (CHECK_DEAD) {
  const { used: usedKeys, dynamic } = scanProject(SCAN_DIRS)
  for (const ns of defaultNamespaces) {
    if (DEAD_KEY_EXEMPT_NAMESPACES.has(ns)) continue
    if (FALLBACK_ONLY_NAMESPACES.has(ns)) continue
    const defaultKeys = readNamespaceKeys(defaultLocale, ns)
    if (!defaultKeys) continue

    const usedInNs = usedKeys.get(ns) ?? new Set<string>()
    const dead = [...defaultKeys].filter(
      (k) => !usedInNs.has(k) && !isCoveredByDynamic(ns, k, dynamic),
    )

    if (dead.length > 0) {
      report(
        7,
        'Dead key',
        `${defaultLocale}/${ns}.json 有 ${dead.length} 个 key 未被源码 t() 引用`,
        dead.slice(0, 10).join(', ') + (dead.length > 10 ? ` …(+${dead.length - 10})` : ''),
        STRICT ? 'error' : 'warning',
      )
    }
  }
}

// ── Rule 6: HTML lang BCP47 ───────────────────────────────────────────────────
// 抓 <html lang={...}> 表达式，禁止直接传 locale code
const htmlLang = gitGrep('<html[[:space:]]+lang=', ['*.ts', '*.tsx']).filter(notExcluded)
for (const line of htmlLang) {
  // 允许 lang={bcp47Of(locale)} / lang="zh-CN" 字面 / lang={someStaticBcp47}
  // 排除 lang={locale} 直传
  if (line.match(/<html\s+lang=\{locale\}/)) {
    report(6, 'HTML lang 必须 BCP47', `直接传 locale code 给 <html lang>，应该用 bcp47Of(locale)：${line}`)
  }
}

// ── 输出 ─────────────────────────────────────────────────────────────────────

const errors = violations.filter((v) => v.severity === 'error')
const warnings = violations.filter((v) => v.severity === 'warning')

if (violations.length === 0) {
  console.log('[lint-i18n] OK — 全部规则通过')
  process.exit(0)
}

const byRule = new Map<number, Violation[]>()
for (const v of violations) {
  if (!byRule.has(v.rule)) byRule.set(v.rule, [])
  byRule.get(v.rule)!.push(v)
}

const stream = errors.length > 0 ? console.error : console.warn
stream(`[lint-i18n] 发现 ${errors.length} error / ${warnings.length} warning：\n`)
for (const [rule, items] of [...byRule.entries()].sort((a, b) => a[0] - b[0])) {
  const sev = items[0]!.severity
  const tag = sev === 'error' ? 'Rule' : 'Rule (warning)'
  stream(`── ${tag} ${rule}: ${items[0]!.ruleName} (${items.length} 处) ──`)
  for (const v of items) {
    stream(`  - ${v.detail}`)
    if (v.location) stream(`      ${v.location}`)
  }
  stream('')
}

process.exit(errors.length > 0 ? 1 : 0)
