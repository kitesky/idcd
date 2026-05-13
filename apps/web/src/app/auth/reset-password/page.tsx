"use client"

import { useState, useEffect, Suspense } from "react"
import { useRouter, useSearchParams } from "next/navigation"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import * as z from "zod"
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
} from "@idcd/ui"
import { AuthLayout } from "@/components/auth/AuthLayout"
import { apiRequest } from "@/lib/api"

const resetPasswordSchema = z.object({
  password: z
    .string()
    .min(8, { message: "密码至少需要 8 个字符" })
    .regex(/^(?=.*[A-Za-z])(?=.*\d)/, {
      message: "密码必须包含字母和数字",
    }),
  confirmPassword: z.string(),
  code: z.string().length(6, { message: "验证码必须为 6 位数字" }).regex(/^\d+$/, {
    message: "验证码只能包含数字",
  }),
}).refine((data) => data.password === data.confirmPassword, {
  message: "两次输入的密码不一致",
  path: ["confirmPassword"],
})

type ResetPasswordFormValues = z.infer<typeof resetPasswordSchema>

function ResetPasswordForm() {
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
      setError(err instanceof Error ? err.message : "重置密码失败，请检查验证码是否正确")
    } finally {
      setIsLoading(false)
    }
  }

  if (!email || !otpId) {
    return null
  }

  return (
    <AuthLayout
      title="重置密码"
      description={`为 ${email} 设置新密码`}
      footer={
        <p>
          <Link href={"/auth/login" as any} className="text-primary hover:underline">
            返回登录
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
                <FormLabel>验证码</FormLabel>
                <FormControl>
                  <Input
                    type="text"
                    placeholder="输入邮件中的 6 位验证码"
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
                <FormLabel>新密码</FormLabel>
                <FormControl>
                  <Input
                    type="password"
                    placeholder="至少 8 位，包含字母和数字"
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
                <FormLabel>确认新密码</FormLabel>
                <FormControl>
                  <Input
                    type="password"
                    placeholder="再次输入新密码"
                    disabled={isLoading}
                    {...field}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <Button type="submit" className="w-full" disabled={isLoading}>
            {isLoading ? "重置中..." : "重置密码"}
          </Button>
        </form>
      </Form>
    </AuthLayout>
  )
}

export default function ResetPasswordPage() {
  return (
    <Suspense fallback={<div>加载中...</div>}>
      <ResetPasswordForm />
    </Suspense>
  )
}
