#!/usr/bin/env tsx
/**
 * One-shot cleanup: 删除 audit-dead-keys --json 输出里的 HIGH 候选，
 * 按"已 i18n 化页面"原则过滤（跳过 leaderboard/tools/referral/oncall/
 * status.statusPages.detail/status.reports/status.page 等占位子树）。
 *
 * 同时从 cn + en 两个 locale 的对应 namespace JSON 删除，并清理
 * 删除后变空的中间对象。
 *
 * Usage:
 *   pnpm --filter @idcd/web exec tsx ../../scripts/_clean-dead-keys.ts          # apply
 *   pnpm --filter @idcd/web exec tsx ../../scripts/_clean-dead-keys.ts --dry-run
 */

import { readFileSync, writeFileSync, existsSync } from 'node:fs'
import { resolve, join } from 'node:path'

const ROOT = resolve(import.meta.dirname, '..')
const MESSAGES_DIR = join(ROOT, 'apps/web/src/i18n/messages')
const REGISTRY_PATH = join(ROOT, 'config/locales.json')
const AUDIT_JSON = '/tmp/dead-audit-full.json'

const DRY_RUN = process.argv.includes('--dry-run')

interface Candidate { ns: string; key: string; confidence: string; reason: string }

// Audit output 通过 audit-dead-keys --json 提前写到 /tmp/dead-audit-full.json
// (execSync 缓冲区会截断大输出)
if (!existsSync(AUDIT_JSON)) {
  console.error(`[clean-dead-keys] 先跑: pnpm --filter @idcd/web exec tsx ../../scripts/audit-dead-keys.ts --json > ${AUDIT_JSON}`)
  process.exit(2)
}
const audit = JSON.parse(readFileSync(AUDIT_JSON, 'utf8')) as { candidates: Candidate[] }

// 过滤规则
function shouldDelete(c: Candidate): boolean {
  if (c.confidence !== 'HIGH') return false

  // 整个 ns 跳过（页面未 i18n 化的占位 + tools 用 t.raw 看不到）
  if (['leaderboard', 'tools'].includes(c.ns)) return false

  // billing.referral.* — referral 页面未 i18n
  if (c.ns === 'billing' && c.key.startsWith('referral.')) return false

  // status.json: 只保留 list 页 dead 旧文案，跳过未 i18n 子页面占位
  if (c.ns === 'status') {
    if (c.key.startsWith('oncall.')) return false
    if (c.key.startsWith('reports.')) return false
    if (c.key.startsWith('statusPages.detail.')) return false
    if (c.key.startsWith('page.')) return false
  }

  return true
}

const targets = audit.candidates.filter(shouldDelete)

console.log(`[clean-dead-keys] 将删除 ${targets.length} 个 dead key (从 ${audit.candidates.filter(c=>c.confidence==='HIGH').length} HIGH 候选过滤)`)

// 按 ns 分组
const byNs = new Map<string, string[]>()
for (const t of targets) {
  if (!byNs.has(t.ns)) byNs.set(t.ns, [])
  byNs.get(t.ns)!.push(t.key)
}
for (const [ns, keys] of [...byNs.entries()].sort()) {
  console.log(`  ${ns}: ${keys.length}`)
}

// 收集所有 locale
const registry = JSON.parse(readFileSync(REGISTRY_PATH, 'utf8')) as {
  locales: { code: string }[]
}
const locales = registry.locales.map((l) => l.code)

// deepUnset：删 path，然后回溯删空对象
type JsonNode = string | number | boolean | null | { [k: string]: JsonNode } | JsonNode[]
type JsonObj = { [k: string]: JsonNode }

function deepUnset(obj: JsonObj, path: string[]): boolean {
  if (path.length === 0) return false
  const [head, ...rest] = path
  if (rest.length === 0) {
    if (head! in obj) {
      delete obj[head!]
      return true
    }
    return false
  }
  const next = obj[head!]
  if (!next || typeof next !== 'object' || Array.isArray(next)) return false
  const child = next as JsonObj
  const deleted = deepUnset(child, rest)
  // 清理空中间对象
  if (deleted && Object.keys(child).length === 0) {
    delete obj[head!]
  }
  return deleted
}

let totalDeleted = 0
for (const [ns, keys] of byNs) {
  for (const locale of locales) {
    const file = join(MESSAGES_DIR, locale, `${ns}.json`)
    if (!existsSync(file)) continue
    const raw = readFileSync(file, 'utf8')
    const parsed = JSON.parse(raw) as JsonObj
    let changed = false
    for (const k of keys) {
      if (deepUnset(parsed, k.split('.'))) {
        changed = true
        totalDeleted++
      }
    }
    if (changed && !DRY_RUN) {
      const trailing = raw.endsWith('\n') ? '\n' : ''
      writeFileSync(file, JSON.stringify(parsed, null, 2) + trailing)
    }
  }
}

console.log(`\n[clean-dead-keys] ${DRY_RUN ? '[dry-run] ' : ''}共删除 ${totalDeleted} 个 key (跨所有 locale)`)
if (DRY_RUN) {
  console.log('--dry-run 模式未实际写入。去掉 --dry-run 应用变更。')
}
