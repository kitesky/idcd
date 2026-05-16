"use client"

import { useState } from "react"
import { useRouter } from "next/navigation"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import * as z from "zod/v3"
import Link from "next/link"
import { useTranslations } from "next-intl"
import {
  Alert,
  AlertDescription,
  Button,
  Input,
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
  Separator,
} from "@/components/ui"
import { AuthLayout } from "@/components/auth/AuthLayout"
import { apiRequest } from "@/lib/api"

async function loginWithPasskey(router: ReturnType<typeof useRouter>, cancelMsg: string) {
  const { data } = await apiRequest<{ data: { options: { challenge: string; allowCredentials?: { id: string; type: string }[]; [key: string]: unknown }; challenge_id: string } }>("/v1/auth/passkeys/begin", {
    method: "POST",
    body: JSON.stringify({}),
  })

  const credential = await navigator.credentials.get({
    publicKey: {
      ...data.options,
      challenge: Uint8Array.from(atob(data.options.challenge.replace(/-/g, "+").replace(/_/g, "/")), c => c.charCodeAt(0)),
      allowCredentials: (data.options.allowCredentials || []).map((c: { id: string; type: string }) => ({
        ...c,
        type: "public-key" as const,
        id: Uint8Array.from(atob(c.id.replace(/-/g, "+").replace(/_/g, "/")), ch => ch.charCodeAt(0)),
      })),
    },
  }) as PublicKeyCredential | null

  if (!credential) throw new Error(cancelMsg)

  const response = credential.response as AuthenticatorAssertionResponse
  await apiRequest("/v1/auth/passkeys/complete", {
    method: "POST",
    body: JSON.stringify({
      challenge: data.challenge_id,
      response: {
        id: credential.id,
        rawId: btoa(String.fromCharCode(...new Uint8Array(credential.rawId))).replace(/\+/g, "-").replace(/\//g, "_").replace(/=/g, ""),
        response: {
          clientDataJSON: btoa(String.fromCharCode(...new Uint8Array(response.clientDataJSON))).replace(/\+/g, "-").replace(/\//g, "_").replace(/=/g, ""),
          authenticatorData: btoa(String.fromCharCode(...new Uint8Array(response.authenticatorData))).replace(/\+/g, "-").replace(/\//g, "_").replace(/=/g, ""),
          signature: btoa(String.fromCharCode(...new Uint8Array(response.signature))).replace(/\+/g, "-").replace(/\//g, "_").replace(/=/g, ""),
        },
      },
    }),
  })
  router.push("/app/dashboard" as never)
}

export default function LoginPage() {
  const t = useTranslations("auth")
  const router = useRouter()
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [passkeyLoading, setPasskeyLoading] = useState(false)

  const loginSchema = z.object({
    email: z.string().email({ message: t("login.errors.invalidEmail") }),
    password: z.string().min(1, { message: t("login.errors.passwordRequired") }),
  })

  type LoginFormValues = z.infer<typeof loginSchema>

  const form = useForm<LoginFormValues>({
    resolver: zodResolver(loginSchema),
    defaultValues: {
      email: "",
      password: "",
    },
  })

  async function onSubmit(values: LoginFormValues) {
    setIsLoading(true)
    setError(null)

    try {
      // The server sets an HttpOnly cookie — no token handling needed on the client.
      await apiRequest("/v1/auth/login", {
        method: "POST",
        body: JSON.stringify(values),
      })

      router.push("/app/dashboard" as any)
    } catch (err) {
      setError(err instanceof Error ? err.message : t("login.errors.loginFailed"))
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <AuthLayout
      title={t("login.title")}
      description={t("login.description")}
      footer={
        <div className="space-y-2">
          <p>
            <Link href={"/auth/forgot-password" as any} className="text-primary hover:underline">
              {t("login.forgotPassword")}
            </Link>
          </p>
          <p>
            {t("login.noAccount")}{" "}
            <Link href={"/auth/register" as any} className="text-primary hover:underline">
              {t("login.register")}
            </Link>
          </p>
        </div>
      }
    >
      <Form {...form}>
        <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-5">
          {error && (
            <Alert variant="destructive">
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}

          <FormField
            control={form.control}
            name="email"
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t("login.email")}</FormLabel>
                <FormControl>
                  <Input
                    type="email"
                    placeholder={t("login.emailPlaceholder")}
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
                <FormLabel>{t("login.password")}</FormLabel>
                <FormControl>
                  <Input
                    type="password"
                    placeholder={t("login.passwordPlaceholder")}
                    disabled={isLoading}
                    {...field}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <Button type="submit" className="w-full" disabled={isLoading}>
            {isLoading ? t("login.submitting") : t("login.submit")}
          </Button>
        </form>
      </Form>

      <Button
        variant="outline"
        className="w-full mt-4"
        data-testid="btn-passkey-login"
        disabled={passkeyLoading}
        onClick={async () => {
          setPasskeyLoading(true)
          setError(null)
          try {
            await loginWithPasskey(router, t("login.errors.passkeyCancel"))
          } catch (err) {
            setError(err instanceof Error ? err.message : t("login.errors.passkeyFailed"))
          } finally {
            setPasskeyLoading(false)
          }
        }}
      >
        {passkeyLoading ? t("login.passkeyVerifying") : t("login.passkey")}
      </Button>

      <div className="relative my-6">
        <Separator />
        <span className="absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 bg-card px-2 text-xs text-muted-foreground">
          {t("login.orThirdParty")}
        </span>
      </div>
      <div className="grid grid-cols-2 gap-3">
        <Button variant="outline" asChild>
          <a href="/api/v1/auth/dingtalk">
            <svg viewBox="0 0 24 24" className="mr-2 h-4 w-4" fill="currentColor" aria-hidden="true">
              <path d="M12 2C6.477 2 2 6.477 2 12s4.477 10 10 10 10-4.477 10-10S17.523 2 12 2zm4.5 14.5h-2l-2.5-4-2.5 4h-2l3.5-5.5L7.5 7.5h2l2.5 3.5 2.5-3.5h2l-3.5 5 3.5 5z" />
            </svg>
            {t("login.dingtalk")}
          </a>
        </Button>
        <Button variant="outline" asChild>
          <a href="/api/v1/auth/feishu">
            <svg viewBox="0 0 24 24" className="mr-2 h-4 w-4" fill="currentColor" aria-hidden="true">
              <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 14H11V8h2v8zm0-10H11V4h2v2z" />
            </svg>
            {t("login.feishu")}
          </a>
        </Button>
      </div>
    </AuthLayout>
  )
}
