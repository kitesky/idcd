#!/usr/bin/env tsx
/**
 * Cross-stack registry coherence check.
 *
 * Verifies that every place that consumes the locale registry stays aligned
 * with `backend/config/locales.json`. Today this means the schema parses, the JSON
 * matches the schema, and known consumer files declare a hash that matches
 * the current registry contents.
 *
 * Exit codes:
 *   0  OK
 *   1  registry malformed
 *   2  a consumer is out of sync (re-run codegen / re-import)
 */

import { createHash } from 'node:crypto'
import { readFileSync, existsSync } from 'node:fs'
import { resolve } from 'node:path'

const ROOT = resolve(import.meta.dirname, '..')
// canonical 注册表在 backend/config/locales.json (后端 Go 服务也读它),
// frontend 通过 git 仓库相对路径跨目录读取
const REGISTRY_PATH = resolve(ROOT, '../backend/config/locales.json')

// Next.js 16 outputFileTracingRoot 锁在 frontend/ 后无法跨目录读取,
// 因此 web runtime 复制了一份;必须保持与 canonical config 同 canonical hash。
const WEB_REGISTRY_PATH = resolve(ROOT, 'src/i18n/locales.json')

function fail(code: number, ...lines: string[]): never {
  for (const l of lines) console.error(l)
  process.exit(code)
}

function ok(...lines: string[]): never {
  for (const l of lines) console.log(l)
  process.exit(0)
}

interface LocaleEntry {
  code: string
  bcp47: string
  label: string
  nativeLabel: string
  baseLanguage: string
  acceptLanguageAliases: string[]
  dir: 'ltr' | 'rtl'
  fontStack: string
  fallback: string[]
  currency: string
}

interface Registry {
  default: string
  locales: LocaleEntry[]
}

function readRegistry(path: string, errCode: number): Registry {
  if (!existsSync(path)) {
    fail(errCode, `[i18n] registry not found at ${path}`)
  }
  let parsed: Registry
  try {
    parsed = JSON.parse(readFileSync(path, 'utf8')) as Registry
  } catch (e) {
    fail(errCode, `[i18n] invalid JSON in ${path}: ${(e as Error).message}`)
  }
  validate(parsed, errCode, path)
  return parsed
}

function validate(r: Registry, errCode: number, path: string): void {
  const prefix = `[i18n] ${path}`
  if (typeof r.default !== 'string' || !r.default) {
    fail(errCode, `${prefix}: registry.default missing`)
  }
  if (!Array.isArray(r.locales) || r.locales.length === 0) {
    fail(errCode, `${prefix}: registry.locales empty`)
  }
  const codes = new Set<string>()
  for (const l of r.locales) {
    const required: (keyof LocaleEntry)[] = [
      'code',
      'bcp47',
      'label',
      'nativeLabel',
      'baseLanguage',
      'acceptLanguageAliases',
      'dir',
      'fontStack',
      'fallback',
      'currency',
    ]
    for (const k of required) {
      if (l[k] === undefined || l[k] === null) {
        fail(errCode, `${prefix}: locale ${l.code ?? '(unnamed)'} missing field "${k}"`)
      }
    }
    if (codes.has(l.code)) {
      fail(errCode, `${prefix}: duplicate locale code "${l.code}"`)
    }
    codes.add(l.code)
    if (!/^[a-z]{2,5}$/.test(l.code)) {
      fail(errCode, `${prefix}: locale code "${l.code}" must match /^[a-z]{2,5}$/`)
    }
    if (!/^[a-z]{2}(-[A-Z][a-z]{3})?(-[A-Z]{2})?$/.test(l.bcp47)) {
      fail(errCode, `${prefix}: locale "${l.code}" bcp47 "${l.bcp47}" not well-formed`)
    }
    if (l.dir !== 'ltr' && l.dir !== 'rtl') {
      fail(errCode, `${prefix}: locale "${l.code}" dir must be ltr or rtl`)
    }
    if (!/^[A-Z]{3}$/.test(l.currency)) {
      fail(errCode, `${prefix}: locale "${l.code}" currency "${l.currency}" must be ISO 4217 (3 uppercase letters)`)
    }
  }
  if (!codes.has(r.default)) {
    fail(errCode, `${prefix}: default "${r.default}" not in locales[]`)
  }
}

function hashCanonical(parsed: Registry): string {
  // Hash only the load-bearing fields. Cosmetic fields like `$schema` differ
  // between the canonical config copy (declares schema) and the web mirror
  // (just plain JSON), but must not break the equivalence check.
  const stripped: Registry = { default: parsed.default, locales: parsed.locales }
  const canonical = JSON.stringify(canonicalize(stripped))
  return createHash('sha256').update(canonical).digest('hex').slice(0, 12)
}

function canonicalize(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(canonicalize)
  if (value && typeof value === 'object') {
    const out: Record<string, unknown> = {}
    for (const key of Object.keys(value as Record<string, unknown>).sort()) {
      out[key] = canonicalize((value as Record<string, unknown>)[key])
    }
    return out
  }
  return value
}

function checkWebMirror(canonicalHash: string): void {
  const webHash = hashCanonical(readRegistry(WEB_REGISTRY_PATH, 2))
  if (webHash !== canonicalHash) {
    fail(
      2,
      `[i18n] src/i18n/locales.json out of sync with backend/config/locales.json`,
      `       config hash: ${canonicalHash}`,
      `       web hash:    ${webHash}`,
      `       fix: copy backend/config/locales.json → src/i18n/locales.json (preserve $schema if present)`,
    )
  }
}

function main(): void {
  const parsed = readRegistry(REGISTRY_PATH, 1)
  const hash = hashCanonical(parsed)
  const codes = parsed.locales.map((l) => l.code).join(', ')

  checkWebMirror(hash)

  ok(
    `[i18n] registry OK`,
    `       default: ${parsed.default}`,
    `       locales: ${codes}`,
    `       hash:    ${hash}`,
    `       web mirror: in sync`,
  )
}

main()
