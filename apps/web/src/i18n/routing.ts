import {
  localeCodes,
  defaultLocale as registryDefault,
  type Locale as RegistryLocale,
} from './registry'

export const locales = localeCodes
export type Locale = RegistryLocale
export const defaultLocale: Locale = registryDefault

export function isValidLocale(locale: string): locale is Locale {
  return locales.includes(locale)
}
