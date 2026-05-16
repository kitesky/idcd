import { loadMessages } from './request'
import { getLocale } from './locale'
import type { Locale } from './routing'

type Params = Record<string, string | number>

function lookup(obj: unknown, keys: string[]): string | undefined {
  let cur: unknown = obj
  for (const k of keys) {
    if (cur && typeof cur === 'object') {
      cur = (cur as Record<string, unknown>)[k]
    } else {
      return undefined
    }
  }
  return typeof cur === 'string' ? cur : undefined
}

function makeT(ns: unknown) {
  return function t(key: string, params?: Params): string {
    const raw = lookup(ns, key.split('.'))
    if (raw === undefined) return key
    if (!params) return raw
    return raw.replace(/\{(\w+)\}/g, (_, k) =>
      Object.prototype.hasOwnProperty.call(params, k) ? String(params[k]) : `{${k}}`,
    )
  }
}

export async function getT(namespace: string, locale?: Locale) {
  const l = locale ?? (await getLocale())
  const messages = await loadMessages(l)
  const ns = (messages as Record<string, unknown>)[namespace] ?? {}
  return makeT(ns)
}
