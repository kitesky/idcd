"use client"

import { useEffect } from "react"
import Link from "next/link"
import { Button } from "@/components/ui/button"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { AlertTriangle } from "lucide-react"

export default function AppError({
  error,
  reset,
}: {
  error: Error & { digest?: string }
  reset: () => void
}) {
  useEffect(() => {
    console.error("[AppError]", error)
  }, [error])

  return (
    <div className="flex flex-1 items-center justify-center p-8">
      <div className="w-full max-w-md space-y-4">
        <Alert variant="destructive">
          <AlertTriangle className="h-4 w-4" />
          <AlertTitle>页面加载失败</AlertTitle>
          <AlertDescription>
            {error.message || "加载时遇到意外错误，请重试。"}
            {error.digest && (
              <span className="mt-1 block font-mono text-xs opacity-60">
                ID：{error.digest}
              </span>
            )}
          </AlertDescription>
        </Alert>
        <div className="flex gap-2">
          <Button onClick={reset} size="sm">
            重试
          </Button>
          <Button variant="outline" size="sm" asChild>
            <Link href="/app/dashboard">返回控制台</Link>
          </Button>
        </div>
      </div>
    </div>
  )
}
