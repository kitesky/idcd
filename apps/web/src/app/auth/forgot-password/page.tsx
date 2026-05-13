"use client"

import { useState } from "react"
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

const forgotPasswordSchema = z.object({
  email: z.string().email({ message: "请输入有效的邮箱地址" }),
})

type ForgotPasswordFormValues = z.infer<typeof forgotPasswordSchema>

export default function ForgotPasswordPage() {
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState(false)

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
      setError(err instanceof Error ? err.message : "请求失败，请稍后重试")
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <AuthLayout
      title="忘记密码"
      description="输入您的邮箱地址，我们将发送重置密码的链接"
      footer={
        <p>
          <Link href={"/auth/login" as any} className="text-primary hover:underline">
            返回登录
          </Link>
        </p>
      }
    >
      {success ? (
        <div className="space-y-4">
          <div className="bg-green-500/15 text-green-600 dark:text-green-400 text-sm p-3 rounded-md">
            重置链接已发送！请检查您的邮箱并按照说明操作。
          </div>
          <Button asChild className="w-full" variant="outline">
            <Link href={"/auth/login" as any}>返回登录</Link>
          </Button>
        </div>
      ) : (
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

            <Button type="submit" className="w-full" disabled={isLoading}>
              {isLoading ? "发送中..." : "发送重置链接"}
            </Button>
          </form>
        </Form>
      )}
    </AuthLayout>
  )
}
