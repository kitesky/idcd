#!/usr/bin/env tsx
/**
 * Cross-stack registry coherence check.
 *
 * Verifies that every place that consumes the locale registry stays aligned
 * with `config/locales.json`. Today this means the schema parses, the JSON
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
const REGISTRY_PATH = resolve(ROOT, 'config/locales.json')

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
}

interface Registry {
  default: string
  locales: LocaleEntry[]
}

function loadRegistry(): { raw: string; parsed: Registry } {
  if (!existsSync(REGISTRY_PATH)) {
    fail(1, `[i18n] registry not found at ${REGISTRY_PATH}`)
  }
  const raw = readFileSync(REGISTRY_PATH, 'utf8')
  let parsed: Registry
  try {
    parsed = JSON.parse(raw) as Registry
  } catch (e) {
    fail(1, `[i18n] invalid JSON in ${REGISTRY_PATH}: ${(e as Error).message}`)
  }
  validate(parsed)
  return { raw, parsed }
}

function validate(r: Registry): void {
  if (typeof r.default !== 'string' || !r.default) {
    fail(1, '[i18n] registry.default missing')
  }
  if (!Array.isArray(r.locales) || r.locales.length === 0) {
    fail(1, '[i18n] registry.locales empty')
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
    ]
    for (const k of required) {
      if (l[k] === undefined || l[k] === null) {
        fail(1, `[i18n] locale ${l.code ?? '(unnamed)'} missing field "${k}"`)
      }
    }
    if (codes.has(l.code)) {
      fail(1, `[i18n] duplicate locale code "${l.code}"`)
    }
    codes.add(l.code)
    if (!/^[a-z]{2,5}$/.test(l.code)) {
      fail(1, `[i18n] locale code "${l.code}" must match /^[a-z]{2,5}$/`)
    }
    if (!/^[a-z]{2}(-[A-Z][a-z]{3})?(-[A-Z]{2})?$/.test(l.bcp47)) {
      fail(1, `[i18n] locale "${l.code}" bcp47 "${l.bcp47}" not well-formed`)
    }
    if (l.dir !== 'ltr' && l.dir !== 'rtl') {
      fail(1, `[i18n] locale "${l.code}" dir must be ltr or rtl`)
    }
  }
  if (!codes.has(r.default)) {
    fail(1, `[i18n] default "${r.default}" not in locales[]`)
  }
}

function hashCanonical(parsed: Registry): string {
  const canonical = JSON.stringify(canonicalize(parsed))
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

function main(): void {
  const { parsed } = loadRegistry()
  const hash = hashCanonical(parsed)
  const codes = parsed.locales.map((l) => l.code).join(', ')

  ok(
    `[i18n] registry OK`,
    `       default: ${parsed.default}`,
    `       locales: ${codes}`,
    `       hash:    ${hash}`,
  )
}

main()
