"use client"

import { useRouter } from "next/navigation"
import { Button } from "@/components/ui/button"
import type { Locale } from "@/i18n/routing"

interface LanguageSwitcherProps {
  currentLocale: Locale
  label: string
}

export function LanguageSwitcher({ currentLocale, label }: LanguageSwitcherProps) {
  const router = useRouter()

  function handleSwitch() {
    const next: Locale = currentLocale === "cn" ? "en" : "cn"
    // Canonical cookie name is `idcd_locale`; clear the legacy `locale` cookie
    // so sessions converge on the new name.
    document.cookie = `idcd_locale=${next};path=/;max-age=31536000;samesite=lax`
    document.cookie = "locale=;path=/;max-age=0"
    router.refresh()
  }

  return (
    <Button
      variant="ghost"
      size="sm"
      onClick={handleSwitch}
      className="text-xs text-muted-foreground hover:text-foreground"
      title={label}
    >
      {currentLocale === "cn" ? "EN" : "中文"}
    </Button>
  )
}
