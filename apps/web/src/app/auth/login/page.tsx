"use client"

import { useState } from "react"
import { useRouter } from "next/navigation"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import * as z from "zod/v3"
import Link from "next/link"
import {
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

async function loginWithPasskey(router: ReturnType<typeof useRouter>) {
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
        id: Uint8Array.from(atob(c.id.replace(/-/g, "+").replace(/_/g, "/")), ch => ch.charCodeAt(0)),
      })),
    },
  }) as PublicKeyCredential | null

  if (!credential) throw new Error("认证被取消")

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

const loginSchema = z.object({
  email: z.string().email({ message: "请输入有效的邮箱地址" }),
  password: z.string().min(1, { message: "请输入密码" }),
})

type LoginFormValues = z.infer<typeof loginSchema>

export default function LoginPage() {
  const router = useRouter()
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [passkeyLoading, setPasskeyLoading] = useState(false)

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
      setError(err instanceof Error ? err.message : "登录失败，请检查邮箱和密码")
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <AuthLayout
      title="登录"
      description="欢迎回到 idcd"
      footer={
        <div className="space-y-2">
          <p>
            <Link href={"/auth/forgot-password" as any} className="text-primary hover:underline">
              忘记密码？
            </Link>
          </p>
          <p>
            还没有账号？{" "}
            <Link href={"/auth/register" as any} className="text-primary hover:underline">
              立即注册
            </Link>
          </p>
        </div>
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
            name="email"
            render={({ field }) => (
              <FormItem>
                <FormLabel>邮箱</FormLabel>
                <FormControl>
                  <Input
                    type="email"
                    placeholder="your@email.com"
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
                <FormLabel>密码</FormLabel>
                <FormControl>
                  <Input
                    type="password"
                    placeholder="输入您的密码"
                    disabled={isLoading}
                    {...field}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <Button type="submit" className="w-full" disabled={isLoading}>
            {isLoading ? "登录中..." : "登录"}
          </Button>
        </form>
      </Form>

      <Button
        variant="outline"
        className="w-full mt-2"
        data-testid="btn-passkey-login"
        disabled={passkeyLoading}
        onClick={async () => {
          setPasskeyLoading(true)
          setError(null)
          try {
            await loginWithPasskey(router)
          } catch (err) {
            setError(err instanceof Error ? err.message : "Passkey 登录失败")
          } finally {
            setPasskeyLoading(false)
          }
        }}
      >
        {passkeyLoading ? "验证中..." : "使用 Passkey 登录"}
      </Button>

      <div className="relative my-4">
        <Separator />
        <span className="absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 bg-background px-2 text-xs text-muted-foreground">
          或使用第三方登录
        </span>
      </div>
      <div className="grid grid-cols-2 gap-3">
        <Button variant="outline" asChild>
          <a href="/api/v1/auth/dingtalk">
            <svg viewBox="0 0 24 24" className="mr-2 h-4 w-4" fill="currentColor" aria-hidden="true">
              <path d="M12 2C6.477 2 2 6.477 2 12s4.477 10 10 10 10-4.477 10-10S17.523 2 12 2zm4.5 14.5h-2l-2.5-4-2.5 4h-2l3.5-5.5L7.5 7.5h2l2.5 3.5 2.5-3.5h2l-3.5 5 3.5 5z" />
            </svg>
            钉钉登录
          </a>
        </Button>
        <Button variant="outline" asChild>
          <a href="/api/v1/auth/feishu">
            <svg viewBox="0 0 24 24" className="mr-2 h-4 w-4" fill="currentColor" aria-hidden="true">
              <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 14H11V8h2v8zm0-10H11V4h2v2z" />
            </svg>
            飞书登录
          </a>
        </Button>
      </div>
    </AuthLayout>
  )
}
