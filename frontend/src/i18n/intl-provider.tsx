'use client'

import { NextIntlClientProvider } from 'next-intl'
import type { ReactNode } from 'react'
import { onError, getMessageFallback } from './error-handlers'

type Messages = Parameters<typeof NextIntlClientProvider>[0]['messages']

interface Props {
  locale: string
  messages: Messages
  now: Date
  children: ReactNode
}

export function IntlProvider({ locale, messages, now, children }: Props) {
  return (
    <NextIntlClientProvider
      locale={locale}
      messages={messages}
      timeZone="Asia/Shanghai"
      now={now}
      onError={onError}
      getMessageFallback={getMessageFallback}
    >
      {children}
    </NextIntlClientProvider>
  )
}
