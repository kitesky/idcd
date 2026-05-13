"use client"

import { useState, useEffect } from "react"
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

const verifySchema = z.object({
  code: z.string().length(6, { message: "验证码必须为 6 位数字" }).regex(/^\d+$/, {
    message: "验证码只能包含数字",
  }),
})

type VerifyFormValues = z.infer<typeof verifySchema>

export default function VerifyEmailPage() {
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
      router.push("/auth/register" as any)
      return
    }

    setEmail(emailParam)
    setOtpId(otpIdParam)
  }, [searchParams, router])

  const form = useForm<VerifyFormValues>({
    resolver: zodResolver(verifySchema),
    defaultValues: {
      code: "",
    },
  })

  async function onSubmit(values: VerifyFormValues) {
    if (!email || !otpId) return

    setIsLoading(true)
    setError(null)

    try {
      await apiRequest("/v1/auth/verify-email", {
        method: "POST",
        body: JSON.stringify({
          email,
          otp_id: otpId,
          code: values.code,
        }),
      })

      router.push("/auth/login" as any)
    } catch (err) {
      setError(err instanceof Error ? err.message : "验证失败，请检查验证码是否正确")
    } finally {
      setIsLoading(false)
    }
  }

  if (!email || !otpId) {
    return null
  }

  return (
    <AuthLayout
      title="验证邮箱"
      description={`我们已向 ${email} 发送了一封包含验证码的邮件`}
      footer={
        <p>
          没有收到邮件？{" "}
          <Link href={"/auth/register" as any} className="text-primary hover:underline">
            重新注册
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
                    placeholder="输入 6 位验证码"
                    maxLength={6}
                    disabled={isLoading}
                    {...field}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <Button type="submit" className="w-full" disabled={isLoading}>
            {isLoading ? "验证中..." : "验证"}
          </Button>
        </form>
      </Form>
    </AuthLayout>
  )
}
