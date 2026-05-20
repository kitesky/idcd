/**
 * Shared next-intl error handlers — applied to both server (`getRequestConfig`)
 * and client (`<NextIntlClientProvider>`) so missing keys behave consistently.
 *
 * Dev mode (NODE_ENV !== 'production'):
 *   - onError: warn instead of throw; loud enough to notice in devtools
 *   - getMessageFallback: render `🌐 ns.key` so the gap is visible in the UI
 *
 * Prod mode:
 *   - onError: silent (next-intl default behavior is to throw, which would
 *     blow up the page; we deliberately downgrade to silent)
 *   - getMessageFallback: render the key itself (next-intl default), which is
 *     less alarming to end users than an emoji prefix
 */
import type { IntlError } from 'next-intl'

const isDev = process.env.NODE_ENV !== 'production'

export function onError(error: IntlError) {
  if (isDev) {
    console.warn('[i18n]', error.message)
  }
  // prod: swallow — getMessageFallback handles the UI side
}

export function getMessageFallback({
  namespace,
  key,
}: {
  namespace?: string
  key: string
  error: IntlError
}): string {
  const full = namespace ? `${namespace}.${key}` : key
  return isDev ? `🌐 ${full}` : full
}
