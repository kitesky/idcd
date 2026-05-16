"use client"

import { useState } from "react"
import { useRouter } from "next/navigation"
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

export default function RegisterPage() {
  const t = useTranslations("auth")
  const router = useRouter()
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const registerSchema = z.object({
    email: z.string().email({ message: t("register.errors.invalidEmail") }),
    password: z
      .string()
      .min(8, { message: t("register.errors.passwordTooShort") })
      .regex(/^(?=.*[A-Za-z])(?=.*\d)/, {
        message: t("register.errors.passwordWeak"),
      }),
    confirmPassword: z.string(),
  }).refine((data) => data.password === data.confirmPassword, {
    message: t("register.errors.passwordMismatch"),
    path: ["confirmPassword"],
  })

  type RegisterFormValues = z.infer<typeof registerSchema>

  const form = useForm<RegisterFormValues>({
    resolver: zodResolver(registerSchema),
    defaultValues: {
      email: "",
      password: "",
      confirmPassword: "",
    },
  })

  async function onSubmit(values: RegisterFormValues) {
    setIsLoading(true)
    setError(null)

    try {
      // Server sets an HttpOnly cookie on success — no token handling needed client-side.
      await apiRequest("/v1/auth/register", {
        method: "POST",
        body: JSON.stringify({
          email: values.email,
          password: values.password,
        }),
      })

      router.push(`/auth/verify-email?email=${encodeURIComponent(values.email)}` as any)
    } catch (err) {
      setError(err instanceof Error ? err.message : t("register.errors.registerFailed"))
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <AuthLayout
      title={t("register.title")}
      description={t("register.description")}
      footer={
        <p>
          {t("register.hasAccount")}{" "}
          <Link href={"/auth/login" as any} className="text-primary hover:underline">
            {t("register.login")}
          </Link>
        </p>
      }
    >
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
                <FormLabel>{t("register.email")}</FormLabel>
                <FormControl>
                  <Input
                    type="email"
                    placeholder={t("register.emailPlaceholder")}
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
                <FormLabel>{t("register.password")}</FormLabel>
                <FormControl>
                  <Input
                    type="password"
                    placeholder={t("register.passwordPlaceholder")}
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
                <FormLabel>{t("register.confirmPassword")}</FormLabel>
                <FormControl>
                  <Input
                    type="password"
                    placeholder={t("register.confirmPasswordPlaceholder")}
                    disabled={isLoading}
                    {...field}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <Button type="submit" className="w-full" disabled={isLoading}>
            {isLoading ? t("register.submitting") : t("register.submit")}
          </Button>
        </form>
      </Form>
    </AuthLayout>
  )
}
