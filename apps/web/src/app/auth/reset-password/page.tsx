"use client"

import { useState, useEffect, Suspense } from "react"
import { useRouter, useSearchParams } from "next/navigation"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import * as z from "zod/v3"
import Link from "next/link"
import { useTranslations } from "next-intl"
import {
  Button,
  Input,
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui"
import { AuthLayout } from "@/components/auth/AuthLayout"
import { apiRequest } from "@/lib/api"

function ResetPasswordForm() {
  const t = useTranslations("auth")
  const router = useRouter()
  const searchParams = useSearchParams()
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [email, setEmail] = useState("")
  const [otpId, setOtpId] = useState("")

  useEffect(() => {
    const emailParam = searchParams.get("email")
    const otpIdParam = searchParams.get("otp_id")

    if (!emailParam || !otpIdParam) {
      router.push("/auth/forgot-password" as any)
      return
    }

    setEmail(emailParam)
    setOtpId(otpIdParam)
  }, [searchParams, router])

  const resetPasswordSchema = z.object({
    password: z
      .string()
      .min(8, { message: t("resetPassword.errors.passwordTooShort") })
      .regex(/^(?=.*[A-Za-z])(?=.*\d)/, {
        message: t("resetPassword.errors.passwordWeak"),
      }),
    confirmPassword: z.string(),
    code: z.string().length(6, { message: t("resetPassword.errors.codeLength") }).regex(/^\d+$/, {
      message: t("resetPassword.errors.codeNumeric"),
    }),
  }).refine((data) => data.password === data.confirmPassword, {
    message: t("resetPassword.errors.passwordMismatch"),
    path: ["confirmPassword"],
  })

  type ResetPasswordFormValues = z.infer<typeof resetPasswordSchema>

  const form = useForm<ResetPasswordFormValues>({
    resolver: zodResolver(resetPasswordSchema),
    defaultValues: {
      password: "",
      confirmPassword: "",
      code: "",
    },
  })

  async function onSubmit(values: ResetPasswordFormValues) {
    if (!email || !otpId) return

    setIsLoading(true)
    setError(null)

    try {
      await apiRequest("/v1/auth/reset-password", {
        method: "POST",
        body: JSON.stringify({
          email,
          otp_id: otpId,
          new_password: values.password,
          code: values.code,
        }),
      })

      router.push("/auth/login" as any)
    } catch (err) {
      setError(err instanceof Error ? err.message : t("resetPassword.errors.resetFailed"))
    } finally {
      setIsLoading(false)
    }
  }

  if (!email || !otpId) {
    return null
  }

  return (
    <AuthLayout
      title={t("resetPassword.title")}
      description={t("resetPassword.descriptionFor", { email })}
      footer={
        <p>
          <Link href={"/auth/login" as any} className="text-primary hover:underline">
            {t("resetPassword.backToLogin")}
          </Link>
        </p>
      }
    >
      <Form {...form}>
        <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
          {error && (
            <div className="bg-destructive/15 text-destructive text-sm p-3 rounded-md">
              {error}
            </div>
          )}

          <FormField
            control={form.control}
            name="code"
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t("resetPassword.code")}</FormLabel>
                <FormControl>
                  <Input
                    type="text"
                    placeholder={t("resetPassword.codePlaceholder")}
                    maxLength={6}
                    disabled={isLoading}
                    {...field}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name="password"
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t("resetPassword.newPassword")}</FormLabel>
                <FormControl>
                  <Input
                    type="password"
                    placeholder={t("resetPassword.newPasswordPlaceholder")}
                    disabled={isLoading}
                    {...field}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name="confirmPassword"
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t("resetPassword.confirmPassword")}</FormLabel>
                <FormControl>
                  <Input
                    type="password"
                    placeholder={t("resetPassword.confirmPasswordPlaceholder")}
                    disabled={isLoading}
                    {...field}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <Button type="submit" className="w-full" disabled={isLoading}>
            {isLoading ? t("resetPassword.submitting") : t("resetPassword.submit")}
          </Button>
        </form>
      </Form>
    </AuthLayout>
  )
}

export default function ResetPasswordPage() {
  const t = useTranslations("auth")
  return (
    <Suspense fallback={<div>{t("resetPassword.loading")}</div>}>
      <ResetPasswordForm />
    </Suspense>
  )
}
