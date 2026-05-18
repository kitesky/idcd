#!/usr/bin/env tsx
/**
 * audit-dead-keys — 二次验证 lint-i18n Rule 7 的 dead key candidates。
 *
 * 第一次扫描 (lint-i18n --check-dead) 只能识别 t('literal') 直接调用 +
 * t(`prefix.${var}.suffix`) 动态前缀。看不到的情况：
 *   - t(varName) 间接调用 (e.g. const key = `${slug}.title`; t(key))
 *   - t(obj.field) 对象属性访问 (e.g. t(catConfig.titleKey))
 *   - 跨文件传 t 后 helper 内调用
 *
 * 这些场景下 i18n key 的字面量字符串仍然会出现在源码里（多半是 const string
 * 字面量或对象字面量 value）。所以二次验证：扫源码**所有 string literals**，
 * 看 candidate dead key 的末尾 1-2 段是否被任何字面量引用过。
 *
 * 输出分级：
 *   HIGH    末尾 2 段在源码完全未出现 → 高置信度 dead，可删
 *   MEDIUM  末尾 1 段未出现但末尾 2 段出现 → 可能 dead，需人工看
 *   LOW     末尾 1 段在源码中出现 → 大概率误报 (动态构造)，保留
 *
 * 用法：
 *   pnpm --filter @idcd/web exec tsx ../../scripts/audit-dead-keys.ts
 *   pnpm --filter @idcd/web exec tsx ../../scripts/audit-dead-keys.ts --json
 */

import { readFileSync, readdirSync, existsSync, statSync } from 'node:fs'
import { resolve, join } from 'node:path'
import { scanProject, isCoveredByDynamic, FILE_EXT_RE } from './_i18n-scan.js'

const ROOT = resolve(import.meta.dirname, '..')
const REGISTRY_PATH = join(ROOT, 'config/locales.json')
const MESSAGES_DIR = join(ROOT, 'apps/web/src/i18n/messages')
const SCAN_DIRS = [
  join(ROOT, 'apps/web/src/app'),
  join(ROOT, 'apps/web/src/components'),
  join(ROOT, 'apps/web/src/lib'),
]
const DEAD_KEY_EXEMPT = new Set(['nav', 'errors'])
const FALLBACK_ONLY = new Set(['admin'])

const JSON_OUTPUT = process.argv.includes('--json')

// ── 读取 registry / 收集 messages key ──────────────────────────────────────

const registry = JSON.parse(readFileSync(REGISTRY_PATH, 'utf8')) as {
  default: string
  locales: { code: string }[]
}
const defaultLocale = registry.default

function collectKeys(obj: unknown, prefix: string, out: Set<string>): void {
  if (obj === null || obj === undefined) return
  if (typeof obj !== 'object') {
    out.add(prefix)
    return
  }
  for (const [k, v] of Object.entries(obj as Record<string, unknown>)) {
    collectKeys(v, prefix ? `${prefix}.${k}` : k, out)
  }
}

function readNsKeys(ns: string): Set<string> {
  const file = join(MESSAGES_DIR, defaultLocale, `${ns}.json`)
  if (!existsSync(file)) return new Set()
  const parsed = JSON.parse(readFileSync(file, 'utf8'))
  const keys = new Set<string>()
  collectKeys(parsed, '', keys)
  return keys
}

function listNamespaces(): string[] {
  const dir = join(MESSAGES_DIR, defaultLocale)
  if (!existsSync(dir)) return []
  return readdirSync(dir)
    .filter((f) => f.endsWith('.json'))
    .map((f) => f.replace(/\.json$/, ''))
}

// ── 全源码 string literal 扫描 ─────────────────────────────────────────────

const SKIP_DIRS = new Set(['node_modules', '.next', '__tests__'])
const SKIP_FILE_RE = /\.(test|d)\.(ts|tsx)$/

function walkFiles(dir: string, out: string[]): void {
  if (!existsSync(dir)) return
  for (const name of readdirSync(dir)) {
    const full = join(dir, name)
    const st = statSync(full)
    if (st.isDirectory()) {
      if (SKIP_DIRS.has(name)) continue
      walkFiles(full, out)
    } else if (FILE_EXT_RE.test(name) && !SKIP_FILE_RE.test(name)) {
      out.push(full)
    }
  }
}

/**
 * 抓所有 i18n 风格的 string literal:
 *   "foo.bar" / 'foo.bar' / `foo.bar` — 至少含一个 `.`，字符限于
 *   [A-Za-z0-9_]，避免抓到 import 路径 / URL。
 * 也收集"末尾段"（最后一段，用于 LOW 级别判定）。
 */
const I18N_LITERAL_RE = /[`'"]([a-zA-Z_][a-zA-Z0-9_]*(?:\.[a-zA-Z0-9_]+)+)[`'"]/g
const IDENTIFIER_LITERAL_RE = /[`'"]([a-zA-Z_][a-zA-Z0-9_]*)[`'"]/g

function collectLiterals(): { dotted: Set<string>; identifiers: Set<string> } {
  const files: string[] = []
  for (const dir of SCAN_DIRS) walkFiles(dir, files)
  const dotted = new Set<string>()
  const identifiers = new Set<string>()
  for (const f of files) {
    try {
      const content = readFileSync(f, 'utf8')
      for (const m of content.matchAll(I18N_LITERAL_RE)) {
        dotted.add(m[1]!)
      }
      for (const m of content.matchAll(IDENTIFIER_LITERAL_RE)) {
        identifiers.add(m[1]!)
      }
    } catch {
      /* skip */
    }
  }
  return { dotted, identifiers }
}

// ── Audit 主流程 ───────────────────────────────────────────────────────────

interface CandidateAudit {
  ns: string
  key: string
  confidence: 'HIGH' | 'MEDIUM' | 'LOW'
  reason: string
}

function audit(): CandidateAudit[] {
  const { used: usedKeys, dynamic } = scanProject(SCAN_DIRS)
  const { dotted, identifiers } = collectLiterals()

  const results: CandidateAudit[] = []
  for (const ns of listNamespaces()) {
    if (DEAD_KEY_EXEMPT.has(ns) || FALLBACK_ONLY.has(ns)) continue
    const nsKeys = readNsKeys(ns)
    const usedInNs = usedKeys.get(ns) ?? new Set<string>()

    for (const k of nsKeys) {
      if (usedInNs.has(k)) continue
      if (isCoveredByDynamic(ns, k, dynamic)) continue

      const segs = k.split('.')
      const lastSeg = segs[segs.length - 1]!
      const last2 = segs.length >= 2 ? segs.slice(-2).join('.') : lastSeg

      // 检查源码 string literal 中是否出现末尾路径
      // dotted set 已含点的字面量（"hero.title", "ui.label" 等）
      // identifiers set 是无点单字面量（"title", "save" 等）
      let confidence: 'HIGH' | 'MEDIUM' | 'LOW'
      let reason: string

      // 检查 ns 全路径形式 "a.b.c" 或末尾 2 段 "b.c"
      let dottedMatch = false
      for (const lit of dotted) {
        if (lit === k || lit.endsWith('.' + last2) || lit === last2) {
          dottedMatch = true
          break
        }
      }

      const idMatch = identifiers.has(lastSeg)

      if (!dottedMatch && !idMatch) {
        confidence = 'HIGH'
        reason = '末尾段 / 末尾2段在源码无字面引用'
      } else if (!dottedMatch && idMatch) {
        confidence = 'LOW'
        reason = `末尾段 "${lastSeg}" 在源码字面出现 (可能动态拼接)`
      } else {
        confidence = 'MEDIUM'
        reason = `末尾2段 "${last2}" 出现在某个字面量里`
      }

      results.push({ ns, key: k, confidence, reason })
    }
  }
  return results
}

// ── 输出 ───────────────────────────────────────────────────────────────────

const results = audit()
const byConfidence = {
  HIGH: results.filter((r) => r.confidence === 'HIGH'),
  MEDIUM: results.filter((r) => r.confidence === 'MEDIUM'),
  LOW: results.filter((r) => r.confidence === 'LOW'),
}

if (JSON_OUTPUT) {
  console.log(
    JSON.stringify(
      {
        total: results.length,
        high: byConfidence.HIGH.length,
        medium: byConfidence.MEDIUM.length,
        low: byConfidence.LOW.length,
        candidates: results,
      },
      null,
      2,
    ),
  )
  process.exit(0)
}

console.log(`[audit-dead-keys] ${defaultLocale} 候选 dead key 分级：\n`)
console.log(`  HIGH    ${byConfidence.HIGH.length}  (末尾段/末尾2段在源码完全未出现，建议删除)`)
console.log(`  MEDIUM  ${byConfidence.MEDIUM.length}  (末尾2段以字面出现，可能死代码，人工审阅)`)
console.log(`  LOW     ${byConfidence.LOW.length}  (末尾段在源码常见，大概率动态构造引用)\n`)

if (byConfidence.HIGH.length > 0) {
  console.log('── HIGH confidence dead keys (按 namespace 分组) ──')
  const byNs = new Map<string, string[]>()
  for (const r of byConfidence.HIGH) {
    if (!byNs.has(r.ns)) byNs.set(r.ns, [])
    byNs.get(r.ns)!.push(r.key)
  }
  for (const [ns, keys] of [...byNs.entries()].sort()) {
    console.log(`\n  cn/${ns}.json (${keys.length})`)
    for (const k of keys) console.log(`    ${k}`)
  }
}
