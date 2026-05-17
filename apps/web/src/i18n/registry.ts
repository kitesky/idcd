import registryData from '@config/locales.json'

export interface LocaleEntry {
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

export interface Registry {
  default: string
  locales: LocaleEntry[]
}

const registry = registryData as Registry

export const locales = registry.locales
export const localeCodes = locales.map((l) => l.code)
export const defaultLocale = registry.default

if (!localeCodes.includes(defaultLocale)) {
  throw new Error(
    `[i18n/registry] default locale "${defaultLocale}" not present in locales[]`,
  )
}

const byCode: Record<string, LocaleEntry> = Object.fromEntries(
  locales.map((l) => [l.code, l]),
)

export type Locale = string

export function isSupported(code: string | null | undefined): code is Locale {
  return typeof code === 'string' && code in byCode
}

export function entryOf(code: string): LocaleEntry {
  const entry = byCode[code]
  if (!entry) {
    throw new Error(`[i18n/registry] unknown locale "${code}"`)
  }
  return entry
}

export function bcp47Of(code: string): string {
  return entryOf(code).bcp47
}

export function dirOf(code: string): 'ltr' | 'rtl' {
  return entryOf(code).dir
}

export function fontStackOf(code: string): string {
  return entryOf(code).fontStack
}

export function nativeLabelOf(code: string): string {
  return entryOf(code).nativeLabel
}

export function fallbackChain(code: string): string[] {
  const seen = new Set<string>()
  const result: string[] = []
  const add = (c: string) => {
    if (!seen.has(c) && byCode[c]) {
      seen.add(c)
      result.push(c)
    }
  }
  const entry = byCode[code]
  if (entry) {
    add(entry.code)
    for (const f of entry.fallback) add(f)
    for (const base of locales) {
      if (base.code !== entry.code && base.baseLanguage === entry.baseLanguage) {
        add(base.code)
      }
    }
  }
  add(defaultLocale)
  return result
}

interface ParsedAcceptLanguage {
  tag: string
  quality: number
}

function parseAcceptLanguage(header: string): ParsedAcceptLanguage[] {
  return header
    .split(',')
    .map((piece): ParsedAcceptLanguage | null => {
      const segments = piece.trim().split(';')
      const tag = (segments[0] ?? '').trim()
      if (!tag) return null
      let quality = 1
      for (const p of segments.slice(1)) {
        const [k, v] = p.split('=').map((s) => s.trim())
        if (k === 'q' && v !== undefined) {
          const parsed = Number.parseFloat(v)
          if (!Number.isNaN(parsed)) quality = parsed
        }
      }
      return { tag, quality }
    })
    .filter((x): x is ParsedAcceptLanguage => x !== null && x.tag !== '*')
    .sort((a, b) => b.quality - a.quality)
}

export function negotiate(header: string | null | undefined): Locale {
  if (!header) return defaultLocale
  const parsed = parseAcceptLanguage(header)
  for (const { tag } of parsed) {
    for (const entry of locales) {
      if (entry.acceptLanguageAliases.some((alias) => matchTag(tag, alias))) {
        return entry.code
      }
    }
  }
  return defaultLocale
}

function matchTag(requestTag: string, alias: string): boolean {
  const rt = requestTag.toLowerCase()
  const al = alias.toLowerCase()
  if (rt === al) return true
  return rt.startsWith(al + '-') || al.startsWith(rt + '-')
}

export const registryHash = computeHash(registryData)

function computeHash(obj: unknown): string {
  const json = JSON.stringify(obj)
  let hash = 0
  for (let i = 0; i < json.length; i++) {
    hash = (hash * 31 + json.charCodeAt(i)) | 0
  }
  return `r${hash.toString(36)}`
}
