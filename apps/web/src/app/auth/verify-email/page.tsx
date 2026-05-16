"use client"

import { useState, Suspense } from "react"
import { useRouter, useSearchParams } from "next/navigation"
import Link from "next/link"
import { useTranslations } from "next-intl"
import {
  Alert,
  AlertDescription,
  Button,
  Input,
} from "@/components/ui"
import { AuthLayout } from "@/components/auth/AuthLayout"
import { apiRequest } from "@/lib/api"

function VerifyEmailForm() {
  const t = useTranslations("auth")
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
      setError(t("verifyEmail.errors.codeLength"))
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
      setError(err instanceof Error ? err.message : t("verifyEmail.errors.verifyFailed"))
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
        title={t("verifyEmail.title")}
        description={email ? t("verifyEmail.descriptionWithEmail", { email }) : t("verifyEmail.descriptionFallback")}
        footer={
          <div className="space-y-2">
            <p>
              <Link
                href={"/app/dashboard" as any}
                className="text-muted-foreground hover:underline text-sm"
                data-testid="skip-verify-link"
              >
                {t("verifyEmail.skipVerify")}
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
              <AlertDescription>{t("verifyEmail.resendSuccess")}</AlertDescription>
            </Alert>
          )}

          <div className="space-y-2">
            <label
              htmlFor="verify-code"
              className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70"
            >
              {t("verifyEmail.code")}
            </label>
            <Input
              id="verify-code"
              type="text"
              inputMode="numeric"
              maxLength={6}
              placeholder={t("verifyEmail.codePlaceholder")}
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
            {isLoading ? t("verifyEmail.submitting") : t("verifyEmail.submit")}
          </Button>

          <Button
            type="button"
            variant="outline"
            className="w-full"
            disabled={resendStatus === "loading"}
            onClick={handleResend}
            data-testid="resend-btn"
          >
            {resendStatus === "loading" ? t("verifyEmail.resending") : t("verifyEmail.resend")}
          </Button>
        </div>
      </AuthLayout>
    </div>
  )
}

export default function VerifyEmailPage() {
  const t = useTranslations("auth")
  return (
    <Suspense fallback={<div>{t("verifyEmail.loading")}</div>}>
      <VerifyEmailForm />
    </Suspense>
  )
}
