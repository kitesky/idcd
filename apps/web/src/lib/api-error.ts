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
 * Translator signature compatible with both `useTranslations()` from next-intl
 * and the lightweight `getT` helper. Matches next-intl's `TranslationValues`
 * shape so call sites can pass the hook return value directly.
 */
type Translator = (
  key: string,
  params?: Record<string, string | number | boolean | Date | null | undefined>,
) => string

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
export function translateApiError(err: ApiError, t: Translator): string {
  if (err.code) {
    const key = `errors.${err.code}`
    try {
      // `err.params` is typed loosely (`Record<string, unknown>`) because the
      // backend may send arbitrary JSON values; next-intl only accepts
      // `TranslationValues` (string | number | boolean | Date | null). We trust
      // the backend to send compatible scalars for error params; cast here to
      // satisfy the stricter Translator signature.
      const translated = t(
        key,
        err.params as Record<string, string | number | boolean | Date | null | undefined> | undefined,
      )
      // Both `useTranslations` and the local getT helper return the key itself
      // when a translation is missing. Detect that and fall through.
      if (translated && translated !== key) return translated
    } catch {
      // `useTranslations` throws on missing key in strict mode — silently
      // fall through to the next layer.
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
