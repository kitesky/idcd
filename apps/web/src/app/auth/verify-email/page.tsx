"use client"

import { useState, Suspense } from "react"
import { useRouter, useSearchParams } from "next/navigation"
import Link from "next/link"
import {
  Alert,
  AlertDescription,
  Button,
  Input,
} from "@/components/ui"
import { AuthLayout } from "@/components/auth/AuthLayout"
import { apiRequest } from "@/lib/api"

function VerifyEmailForm() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const email = searchParams.get("email") ?? ""
  const otpId = searchParams.get("otp_id") ?? ""

  const [code, setCode] = useState("")
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [resendStatus, setResendStatus] = useState<"idle" | "sent" | "loading">("idle")

  async function handleVerify() {
    if (code.length !== 6) {
      setError("请输入 6 位验证码")
      return
    }

    setIsLoading(true)
    setError(null)

    try {
      await apiRequest("/v1/auth/verify-email", {
        method: "POST",
        body: JSON.stringify({
          email,
          otp_id: otpId,
          code,
        }),
      })

      router.push("/app/dashboard" as any)
    } catch (err) {
      setError(err instanceof Error ? err.message : "验证失败，请检查验证码是否正确")
    } finally {
      setIsLoading(false)
    }
  }

  async function handleResend() {
    setResendStatus("loading")

    try {
      await apiRequest("/v1/auth/resend-verify", {
        method: "POST",
      })

      setResendStatus("sent")
      setTimeout(() => setResendStatus("idle"), 1500)
    } catch {
      // fail-soft: silently ignore errors
      setResendStatus("idle")
    }
  }

  return (
    <div data-testid="verify-email-page">
      <AuthLayout
        title="验证邮箱"
        description={email ? `验证码已发送到 ${email}，请查收` : "请查收验证邮件"}
        footer={
          <div className="space-y-2">
            <p>
              <Link
                href={"/app/dashboard" as any}
                className="text-muted-foreground hover:underline text-sm"
                data-testid="skip-verify-link"
              >
                跳过，稍后验证
              </Link>
            </p>
          </div>
        }
      >
        <div className="space-y-4">
          {error && (
            <Alert variant="destructive">
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}

          {resendStatus === "sent" && (
            <Alert>
              <AlertDescription>已重新发送验证码，请查收邮件</AlertDescription>
            </Alert>
          )}

          <div className="space-y-2">
            <label
              htmlFor="verify-code"
              className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70"
            >
              验证码
            </label>
            <Input
              id="verify-code"
              type="text"
              inputMode="numeric"
              maxLength={6}
              placeholder="输入 6 位验证码"
              value={code}
              onChange={(e) => {
                const val = e.target.value.replace(/\D/g, "")
                setCode(val)
              }}
              disabled={isLoading}
              data-testid="verify-code-input"
            />
          </div>

          <Button
            type="button"
            className="w-full"
            disabled={isLoading}
            onClick={handleVerify}
            data-testid="verify-submit-btn"
          >
            {isLoading ? "验证中..." : "验证"}
          </Button>

          <Button
            type="button"
            variant="outline"
            className="w-full"
            disabled={resendStatus === "loading"}
            onClick={handleResend}
            data-testid="resend-btn"
          >
            {resendStatus === "loading" ? "发送中..." : "重新发送"}
          </Button>
        </div>
      </AuthLayout>
    </div>
  )
}

export default function VerifyEmailPage() {
  return (
    <Suspense fallback={<div>加载中...</div>}>
      <VerifyEmailForm />
    </Suspense>
  )
}
