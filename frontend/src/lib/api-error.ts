import type { useTranslations } from 'next-intl'

/**
 * Normalized error shape returned by the idcd API.
 *
 * See `docs/prd/I18N-PLAN.md` §2.3 — the backend serializes errors as
 * `{ code, message, params, request_id }`. `message` is the locale-aware
 * fallback prepared by the server, but the canonical contract is `code`
 * (a stable string like `AUTH_REQUIRED`) which the frontend can translate
 * independently.
 */
export interface ApiError {
  code?: string
  message?: string
  params?: Record<string, unknown>
  request_id?: string
}

/**
 * Accepts next-intl's global `t` (`useTranslations()` without namespace).
 * Call sites pass the hook return value directly without `as never`. Dynamic
 * `errors.${code}` lookups inside this helper still need `as never` since
 * runtime-built keys can't be statically proven to be valid.
 */
type GlobalT = ReturnType<typeof useTranslations>

/**
 * Convert a structured API error into a user-facing string.
 *
 * Resolution order:
 *   1. `t('errors.${err.code}', err.params)` — frontend translation wins.
 *      If the key is missing, `t` returns the key itself; we detect that and
 *      fall through to the next layer.
 *   2. `err.message` — server-prepared, locale-aware copy.
 *   3. `t('errors.UNKNOWN')` — generic fallback.
 *
 * The `errors.UNKNOWN` key is expected to exist in every locale's
 * `messages/{locale}/errors.json` (enforced by lint:i18n in Phase 5).
 */
// next-intl 4 收紧 Translator 类型，`t(key as never, params)` 这条 escape hatch
// 失效。helper 内用 unknown 收口，cast 成宽松签名。
type LooseT = (key: string, params?: Record<string, unknown>) => string

export function translateApiError(err: ApiError, t: GlobalT): string {
  if (err.code) {
    const key = `errors.${err.code}`
    try {
      const translated = (t as unknown as LooseT)(key, err.params)
      // useTranslations returns the key itself when a translation is missing.
      // Detect that and fall through.
      if (translated && translated !== key) return translated
    } catch {
      // useTranslations throws on missing key in strict mode — fall through.
    }
  }

  if (err.message && err.message.trim().length > 0) {
    return err.message
  }

  try {
    return t('errors.UNKNOWN')
  } catch {
    return 'Unknown error'
  }
}
