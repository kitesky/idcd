"use client"

import { useState } from "react"
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

export default function ForgotPasswordPage() {
  const t = useTranslations("auth")
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState(false)

  const forgotPasswordSchema = z.object({
    email: z.string().email({ message: t("forgotPassword.errors.invalidEmail") }),
  })

  type ForgotPasswordFormValues = z.infer<typeof forgotPasswordSchema>

  const form = useForm<ForgotPasswordFormValues>({
    resolver: zodResolver(forgotPasswordSchema),
    defaultValues: {
      email: "",
    },
  })

  async function onSubmit(values: ForgotPasswordFormValues) {
    setIsLoading(true)
    setError(null)
    setSuccess(false)

    try {
      await apiRequest("/v1/auth/forgot-password", {
        method: "POST",
        body: JSON.stringify(values),
      })

      setSuccess(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : t("forgotPassword.errors.requestFailed"))
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <AuthLayout
      title={t("forgotPassword.title")}
      description={t("forgotPassword.description")}
      footer={
        <p>
          <Link href={"/auth/login" as any} className="text-primary hover:underline">
            {t("forgotPassword.backToLogin")}
          </Link>
        </p>
      }
    >
      {success ? (
        <div className="space-y-5">
          <div className="bg-success/15 text-success text-sm p-3 rounded-md">
            {t("forgotPassword.successMessage")}
          </div>
          <Button asChild className="w-full" variant="outline">
            <Link href={"/auth/login" as any}>{t("forgotPassword.backToLogin")}</Link>
          </Button>
        </div>
      ) : (
        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-5">
            {error && (
              <div className="bg-destructive/15 text-destructive text-sm p-3 rounded-md">
                {error}
              </div>
            )}

            <FormField
              control={form.control}
              name="email"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("forgotPassword.email")}</FormLabel>
                  <FormControl>
                    <Input
                      type="email"
                      placeholder={t("forgotPassword.emailPlaceholder")}
                      disabled={isLoading}
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <Button type="submit" className="w-full" disabled={isLoading}>
              {isLoading ? t("forgotPassword.submitting") : t("forgotPassword.submit")}
            </Button>
          </form>
        </Form>
      )}
    </AuthLayout>
  )
}
