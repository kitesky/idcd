"use client"

import { Suspense, useEffect } from "react"
import { useRouter, useSearchParams } from "next/navigation"
import { useTranslations } from "next-intl"

function OAuthCallbackContent() {
  const t = useTranslations("auth")
  const router = useRouter()
  const searchParams = useSearchParams()

  useEffect(() => {
    const token = searchParams.get("token")
    if (token) {
      try {
        localStorage.setItem("auth_token", token)
      } catch {
        // localStorage may be unavailable in some environments
      }
    }
    router.replace("/app/dashboard" as any)
  }, [router, searchParams])

  return (
    <div className="flex-1 flex items-center justify-center">
      <div className="text-center space-y-4">
        <h1 className="text-2xl font-semibold">{t("oauthCallback.title")}</h1>
        <p className="text-muted-foreground">{t("oauthCallback.description")}</p>
      </div>
    </div>
  )
}

export default function OAuthCallbackPage() {
  const t = useTranslations("auth")
  return (
    <Suspense
      fallback={
        <div className="flex-1 flex items-center justify-center">
          <div className="text-center space-y-4">
            <h1 className="text-2xl font-semibold">{t("oauthCallback.title")}</h1>
            <p className="text-muted-foreground">{t("oauthCallback.description")}</p>
          </div>
        </div>
      }
    >
      <OAuthCallbackContent />
    </Suspense>
  )
}
