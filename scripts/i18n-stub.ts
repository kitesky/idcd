#!/usr/bin/env tsx
/**
 * i18n:stub — 扫描源码里的 t('foo.bar') 调用，给 messages JSON 追加 TODO 占位。
 *
 * 用法：
 *   pnpm --filter @idcd/web i18n:stub              # default locale (cn) 上加占位
 *   pnpm --filter @idcd/web i18n:stub --locale en  # 指定 locale
 *   pnpm --filter @idcd/web i18n:stub --dry-run    # 只打印缺 key，不写入
 *   pnpm --filter @idcd/web i18n:stub --self-test  # 内置 fixture 自检
 *
 * 扫描规则（regex 简化版，AST 太重）：
 *   1. 找每个 .ts/.tsx 文件里 `(?:const|let) X = useTranslations(['ns'])?`
 *      建立变量名 → namespace prefix 映射（namespace 可嵌套，如 'billing.usage'）
 *   2. 找该变量的 X('key.path') / X(`key.path`) 调用（仅纯字面量）
 *   3. 拼成完整 JSON 路径：useTranslations('billing.usage') + t('tomorrowAt')
 *      → billing.json 里的 usage.tomorrowAt
 *
 * 已知限制：
 *   - 跳过动态 key：t(`${x}.y`) / t(item.title) — 反正这些场景在调用点已 as never
 *   - 跳过 getT / getTranslations（server）— 同等模式可在后续追加
 *   - 不做跨文件变量传递分析（t 被传给 helper 后的调用看不到）— 用 t prop 传给
 *     helper 的场景 helper 内部的 t() 不会被关联到任何 namespace
 */

import { readFileSync, existsSync, writeFileSync } from 'node:fs'
import { resolve, join } from 'node:path'
import { scanFile, scanProject } from './_i18n-scan.js'

const ROOT = resolve(import.meta.dirname, '..')
const REGISTRY_PATH = join(ROOT, 'config/locales.json')
const MESSAGES_DIR = join(ROOT, 'apps/web/src/i18n/messages')
const SCAN_DIRS = [
  join(ROOT, 'apps/web/src/app'),
  join(ROOT, 'apps/web/src/components'),
  join(ROOT, 'apps/web/src/lib'),
]

// ── CLI parsing ────────────────────────────────────────────────────────────

interface Args {
  locale?: string
  dryRun: boolean
  selfTest: boolean
}

function parseArgs(): Args {
  const args = process.argv.slice(2)
  let locale: string | undefined
  let dryRun = false
  let selfTest = false
  for (let i = 0; i < args.length; i++) {
    const a = args[i]!
    if (a === '--dry-run') dryRun = true
    else if (a === '--self-test') selfTest = true
    else if (a === '--locale') locale = args[++i]
    else if (a.startsWith('--locale=')) locale = a.slice('--locale='.length)
  }
  return { locale, dryRun, selfTest }
}

// ── JSON 操作 ──────────────────────────────────────────────────────────────

type JsonNode = string | number | boolean | null | { [k: string]: JsonNode } | JsonNode[]
type JsonObject = { [k: string]: JsonNode }

function flatKeys(obj: JsonNode, prefix: string, out: Set<string>): void {
  if (obj === null || obj === undefined) return
  if (typeof obj !== 'object') {
    out.add(prefix)
    return
  }
  if (Array.isArray(obj)) {
    obj.forEach((v, i) => flatKeys(v, prefix ? `${prefix}.${i}` : String(i), out))
    return
  }
  for (const [k, v] of Object.entries(obj)) {
    flatKeys(v, prefix ? `${prefix}.${k}` : k, out)
  }
}

function setDeep(obj: JsonObject, path: string[], value: string): boolean {
  // 返回 true 表示真的写入了新 key
  let cur: JsonObject = obj
  for (let i = 0; i < path.length - 1; i++) {
    const seg = path[i]!
    const next = cur[seg]
    if (next === undefined) {
      cur[seg] = {}
      cur = cur[seg] as JsonObject
    } else if (typeof next === 'object' && next !== null && !Array.isArray(next)) {
      cur = next as JsonObject
    } else {
      // 类型冲突：路径中段已经是 string/number/bool — 不覆盖，跳过
      console.warn(`[i18n-stub] 路径冲突，跳过 ${path.join('.')}: 中间段 "${seg}" 已是非对象`)
      return false
    }
  }
  const last = path[path.length - 1]!
  if (cur[last] !== undefined) return false // 已存在
  cur[last] = value
  return true
}

interface StubReport {
  locale: string
  ns: string
  added: string[]
}

function stubLocale(locale: string, used: Map<string, Set<string>>, dryRun: boolean): StubReport[] {
  const reports: StubReport[] = []
  const localeDir = join(MESSAGES_DIR, locale)
  if (!existsSync(localeDir)) {
    console.error(`[i18n-stub] locale 目录不存在: ${localeDir}`)
    return reports
  }

  for (const [topNs, usedKeysInNs] of used) {
    const file = join(localeDir, `${topNs}.json`)
    if (!existsSync(file)) {
      console.warn(`[i18n-stub] 跳过 ${locale}/${topNs}.json（不存在，先建文件再 stub）`)
      continue
    }
    const raw = readFileSync(file, 'utf8')
    const parsed = JSON.parse(raw) as JsonObject
    const existing = new Set<string>()
    flatKeys(parsed, '', existing)

    const added: string[] = []
    for (const k of usedKeysInNs) {
      if (existing.has(k)) continue
      const path = k.split('.')
      const placeholder = `TODO: ${topNs}.${k}`
      if (setDeep(parsed, path, placeholder)) {
        added.push(k)
      }
    }

    if (added.length > 0) {
      reports.push({ locale, ns: topNs, added })
      if (!dryRun) {
        // 保留末尾换行（lint:i18n 不强制，但符合 git 习惯）
        const trailingNL = raw.endsWith('\n') ? '\n' : ''
        writeFileSync(file, JSON.stringify(parsed, null, 2) + trailingNL)
      }
    }
  }

  return reports
}

// ── Self-test ──────────────────────────────────────────────────────────────

function selfTest(): number {
  let failed = 0
  function expect(cond: boolean, msg: string) {
    if (!cond) {
      console.error(`  ✗ ${msg}`)
      failed++
    } else {
      console.log(`  ✓ ${msg}`)
    }
  }

  // case 1: useTranslations('home') + t('hero.title') → home.json 的 hero.title
  {
    const used = new Map<string, Set<string>>()
    scanFile(`const t = useTranslations('home'); return t('hero.title')`, used)
    expect(used.get('home')?.has('hero.title') === true, "useTranslations('home') + t('hero.title')")
  }

  // case 2: nested namespace useTranslations('billing.usage') + t('tomorrowAt')
  // → billing.json 的 usage.tomorrowAt
  {
    const used = new Map<string, Set<string>>()
    scanFile(`const t = useTranslations('billing.usage'); t('tomorrowAt')`, used)
    expect(used.get('billing')?.has('usage.tomorrowAt') === true, "嵌套 ns billing.usage + t('tomorrowAt')")
  }

  // case 3: useTranslations() 不带 ns + t('errors.UNKNOWN') → errors.json 的 UNKNOWN
  {
    const used = new Map<string, Set<string>>()
    scanFile(`const tErr = useTranslations(); tErr('errors.UNKNOWN')`, used)
    expect(used.get('errors')?.has('UNKNOWN') === true, "useTranslations() 无 ns + t('errors.UNKNOWN')")
  }

  // case 4: 动态 key 应被忽略
  {
    const used = new Map<string, Set<string>>()
    scanFile('const t = useTranslations("home"); t(`${slug}.title`)', used)
    expect(used.has('home') === false || used.get('home')?.size === 0, '动态模板 key 应跳过')
  }

  // case 4b: 同文件多个 binding 相同变量名，按位置区分作用域
  {
    const used = new Map<string, Set<string>>()
    scanFile(
      `function A() { const t = useTranslations("nodes.myApplications.statusLabel"); t("pending") }
       function B() { const t = useTranslations("nodes"); t("title") }`,
      used,
    )
    expect(
      used.get('nodes')?.has('myApplications.statusLabel.pending') === true,
      'A 函数的 t() 关联到 statusLabel namespace',
    )
    expect(used.get('nodes')?.has('title') === true, 'B 函数的 t() 关联到 nodes 根')
  }

  // case 5: setDeep 在已存在 key 时不覆盖
  {
    const obj: JsonObject = { hero: { title: '已有翻译' } }
    const inserted = setDeep(obj, ['hero', 'title'], 'TODO: x')
    expect(inserted === false, 'setDeep 已存在 key 不覆盖')
    expect((obj.hero as JsonObject).title === '已有翻译', '原值保留')
  }

  // case 6: setDeep 嵌套创建
  {
    const obj: JsonObject = {}
    setDeep(obj, ['a', 'b', 'c'], 'TODO: a.b.c')
    expect(
      ((obj.a as JsonObject).b as JsonObject).c === 'TODO: a.b.c',
      '深层嵌套 setDeep',
    )
  }

  // case 7: getT / getTranslations 也被识别（不只是 useTranslations）
  {
    const used = new Map<string, Set<string>>()
    scanFile(`const t = await getT('about', locale); t('hero.title')`, used)
    expect(used.get('about')?.has('hero.title') === true, 'getT 也被 scanner 识别')
  }
  {
    const used = new Map<string, Set<string>>()
    scanFile(`const t = await getTranslations('home'); t('hero.cta')`, used)
    expect(used.get('home')?.has('hero.cta') === true, 'getTranslations 也被 scanner 识别')
  }

  console.log(`\n[i18n-stub] self-test: ${failed === 0 ? 'OK' : `${failed} failed`}`)
  return failed === 0 ? 0 : 1
}

// ── Main ───────────────────────────────────────────────────────────────────

const args = parseArgs()

if (args.selfTest) {
  process.exit(selfTest())
}

if (!existsSync(REGISTRY_PATH)) {
  console.error(`[i18n-stub] registry not found: ${REGISTRY_PATH}`)
  process.exit(2)
}
const registry = JSON.parse(readFileSync(REGISTRY_PATH, 'utf8')) as {
  default: string
  locales: { code: string }[]
}
const targetLocale = args.locale ?? registry.default

if (!registry.locales.some((l) => l.code === targetLocale)) {
  console.error(`[i18n-stub] locale "${targetLocale}" 不在 registry`)
  process.exit(2)
}

const { used, filesScanned } = scanProject(SCAN_DIRS)

let totalUsed = 0
for (const set of used.values()) totalUsed += set.size

console.log(`[i18n-stub] 扫描 ${filesScanned} 个源文件，找到 ${used.size} 个 namespace, 共 ${totalUsed} 个静态 key`)

const reports = stubLocale(targetLocale, used, args.dryRun)
const totalAdded = reports.reduce((s, r) => s + r.added.length, 0)

if (totalAdded === 0) {
  console.log(`[i18n-stub] ${targetLocale} 已完整，无新增 TODO 占位`)
  process.exit(0)
}

console.log(`\n${args.dryRun ? '[dry-run] ' : ''}${targetLocale} 新增 ${totalAdded} 个 TODO 占位：`)
for (const r of reports) {
  console.log(`  ${r.ns}.json (+${r.added.length}):`)
  for (const k of r.added.slice(0, 10)) {
    console.log(`    ${k}`)
  }
  if (r.added.length > 10) console.log(`    …(+${r.added.length - 10})`)
}

if (args.dryRun) {
  console.log('\n[dry-run] 未实际写入。去掉 --dry-run 应用变更。')
}
