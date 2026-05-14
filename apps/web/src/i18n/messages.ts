import type { Locale } from './config'
import zh from './messages/zh.json'
import en from './messages/en.json'

const messages = { zh, en } as const

export type Messages = typeof zh

export function getMessages(locale: Locale): Messages {
  return messages[locale]
}
