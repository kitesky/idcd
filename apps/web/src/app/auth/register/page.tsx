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
} from "@/components/ui"
import { AuthLayout } from "@/components/auth/AuthLayout"
import { apiRequest } from "@/lib/api"

const registerSchema = z.object({
  email: z.string().email({ message: "请输入有效的邮箱地址" }),
  password: z
    .string()
    .min(8, { message: "密码至少需要 8 个字符" })
    .regex(/^(?=.*[A-Za-z])(?=.*\d)/, {
      message: "密码必须包含字母和数字",
    }),
  confirmPassword: z.string(),
}).refine((data) => data.password === data.confirmPassword, {
  message: "两次输入的密码不一致",
  path: ["confirmPassword"],
})

type RegisterFormValues = z.infer<typeof registerSchema>

export default function RegisterPage() {
  const router = useRouter()
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

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

      router.push("/app/dashboard" as any)
    } catch (err) {
      setError(err instanceof Error ? err.message : "注册失败，请稍后重试")
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <AuthLayout
      title="注册账号"
      description="创建您的 idcd 账号"
      footer={
        <p>
          已有账号？{" "}
          <Link href={"/auth/login" as any} className="text-primary hover:underline">
            立即登录
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
                <FormLabel>确认密码</FormLabel>
                <FormControl>
                  <Input
                    type="password"
                    placeholder="再次输入密码"
                    disabled={isLoading}
                    {...field}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <Button type="submit" className="w-full" disabled={isLoading}>
            {isLoading ? "注册中..." : "注册"}
          </Button>
        </form>
      </Form>
    </AuthLayout>
  )
}
