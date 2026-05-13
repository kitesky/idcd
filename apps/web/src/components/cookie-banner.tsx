"use client"

import { useState, useEffect } from "react"
import Link from "next/link"
import type { Route } from "next"
import { Button } from "@/components/ui"
import { X } from "lucide-react"

const COOKIE_CONSENT_KEY = "idcd-cookie-consent"

type ConsentType = "all" | "essential"

export function CookieBanner() {
  const [isVisible, setIsVisible] = useState(false)

  useEffect(() => {
    const saved = localStorage.getItem(COOKIE_CONSENT_KEY)
    const valid: ConsentType[] = ["all", "essential"]
    if (valid.includes(saved as ConsentType)) {
      setIsVisible(false)
    } else {
      setIsVisible(true)
    }
  }, [])

  const handleConsent = (type: ConsentType) => {
    localStorage.setItem(COOKIE_CONSENT_KEY, type)
    setIsVisible(false)
  }

  if (!isVisible) {
    return null
  }

  return (
    <div
      role="region"
      aria-label="Cookie 使用同意"
      className="fixed bottom-0 left-0 right-0 z-50 border-t bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/80"
    >
      <div className="mx-auto max-w-7xl px-4 py-4 sm:px-6 lg:px-8">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex-1">
            <p className="text-sm text-muted-foreground">
              我们使用 Cookie 来提供更好的用户体验。继续使用本站即表示您同意我们的{" "}
              <Link href={"/privacy" as Route} className="text-primary hover:underline">
                隐私政策
              </Link>
              {" "}和{" "}
              <Link href={"/terms" as Route} className="text-primary hover:underline">
                服务条款
              </Link>
              。
            </p>
          </div>

          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => handleConsent("essential")}
            >
              仅必要
            </Button>
            <Button
              size="sm"
              onClick={() => handleConsent("all")}
            >
              接受所有
            </Button>
            {/* X 按钮等同于拒绝（仅必要 Cookie），符合 GDPR/PIPL 要求 */}
            <Button
              variant="ghost"
              size="icon"
              className="h-8 w-8"
              onClick={() => handleConsent("essential")}
              aria-label="关闭（仅接受必要 Cookie）"
            >
              <X className="h-4 w-4" />
            </Button>
          </div>
        </div>
      </div>
    </div>
  )
}
