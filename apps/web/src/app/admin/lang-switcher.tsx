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
    const next: Locale = currentLocale === "zh" ? "en" : "zh"
    document.cookie = `locale=${next};path=/;max-age=31536000`
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
      {currentLocale === "zh" ? "EN" : "中文"}
    </Button>
  )
}
