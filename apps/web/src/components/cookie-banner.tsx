"use client"

import { useState, useEffect } from "react"
import { Button } from "@idcd/ui"
import { X } from "lucide-react"

const COOKIE_CONSENT_KEY = "idcd-cookie-consent"

type ConsentType = "all" | "essential" | null

export function CookieBanner() {
  const [consent, setConsent] = useState<ConsentType>(null)
  const [isVisible, setIsVisible] = useState(false)

  useEffect(() => {
    // 检查用户是否已经做出选择
    const savedConsent = localStorage.getItem(COOKIE_CONSENT_KEY)
    if (savedConsent) {
      setConsent(savedConsent as ConsentType)
      setIsVisible(false)
    } else {
      // 延迟显示，避免影响首次渲染
      const timer = setTimeout(() => setIsVisible(true), 1000)
      return () => clearTimeout(timer)
    }
  }, [])

  const handleConsent = (type: ConsentType) => {
    if (type) {
      localStorage.setItem(COOKIE_CONSENT_KEY, type)
      setConsent(type)
      setIsVisible(false)

      // 这里可以添加实际的 Cookie 设置逻辑
      // 例如：启用分析工具、设置追踪 Cookie 等
      if (type === "all") {
        console.log("用户接受了所有 Cookie")
        // 启用所有分析和追踪功能
      } else if (type === "essential") {
        console.log("用户仅接受必要 Cookie")
        // 仅启用必要功能
      }
    }
  }

  if (!isVisible) {
    return null
  }

  return (
    <div className="fixed bottom-0 left-0 right-0 z-50 border-t bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/80">
      <div className="mx-auto max-w-7xl px-4 py-4 sm:px-6 lg:px-8">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          {/* 文本内容 */}
          <div className="flex-1">
            <p className="text-sm text-muted-foreground">
              我们使用 Cookie 来提供更好的用户体验。继续使用本站即表示您同意我们的{" "}
              <a
                href="/privacy"
                className="text-primary hover:underline"
                onClick={() => setIsVisible(false)}
              >
                隐私政策
              </a>
              {" "}和{" "}
              <a
                href="/terms"
                className="text-primary hover:underline"
                onClick={() => setIsVisible(false)}
              >
                服务条款
              </a>
              。
            </p>
          </div>

          {/* 按钮组 */}
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
            <Button
              variant="ghost"
              size="icon"
              className="h-8 w-8"
              onClick={() => setIsVisible(false)}
              aria-label="关闭"
            >
              <X className="h-4 w-4" />
            </Button>
          </div>
        </div>
      </div>
    </div>
  )
}
