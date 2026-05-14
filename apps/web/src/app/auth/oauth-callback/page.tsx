"use client"

import { Suspense, useEffect } from "react"
import { useRouter, useSearchParams } from "next/navigation"

function OAuthCallbackContent() {
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
        <h1 className="text-2xl font-semibold">登录中...</h1>
        <p className="text-muted-foreground">正在跳转，请稍候</p>
      </div>
    </div>
  )
}

export default function OAuthCallbackPage() {
  return (
    <Suspense
      fallback={
        <div className="flex-1 flex items-center justify-center">
          <div className="text-center space-y-4">
            <h1 className="text-2xl font-semibold">登录中...</h1>
            <p className="text-muted-foreground">正在跳转，请稍候</p>
          </div>
        </div>
      }
    >
      <OAuthCallbackContent />
    </Suspense>
  )
}
