"use client"

import { useRouter } from "next/navigation"
import { Button } from "@/components/ui/button"
import type { Locale } from "@/i18n/routing"
import { locales, nativeLabelOf } from "@/i18n/registry"

interface LanguageSwitcherProps {
  currentLocale: Locale
  label: string
}

// Pick the next locale by walking the registry (registry-driven, no binary
// `locale === 'cn'` ladders — that would fail the CI lint and break the
// "add a locale = zero code changes" contract).
function nextLocale(current: Locale): Locale {
  const codes = locales.map((l) => l.code)
  const i = codes.indexOf(current)
  if (i < 0) return codes[0]!
  return codes[(i + 1) % codes.length]!
}

export function LanguageSwitcher({ currentLocale, label }: LanguageSwitcherProps) {
  const router = useRouter()

  function handleSwitch() {
    const next = nextLocale(currentLocale)
    // Canonical cookie name is `idcd_locale`; clear the legacy `locale` cookie
    // so sessions converge on the new name.
    document.cookie = `idcd_locale=${next};path=/;max-age=31536000;samesite=lax`
    document.cookie = "locale=;path=/;max-age=0"
    router.refresh()
  }

  const next = nextLocale(currentLocale)

  return (
    <Button
      variant="ghost"
      size="sm"
      onClick={handleSwitch}
      className="text-xs text-muted-foreground hover:text-foreground"
      title={label}
    >
      {nativeLabelOf(next)}
    </Button>
  )
}
