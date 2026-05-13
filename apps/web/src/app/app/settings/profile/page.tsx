"use client"

import { useState, useEffect } from "react"
import { useRouter } from "next/navigation"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import * as z from "zod"
import {
  Button,
  Input,
  Textarea,
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui"
import { apiRequest } from "@/lib/api"

const profileSchema = z.object({
  email: z.string().email(),
  display_name: z.string().max(50, { message: "显示名称不能超过 50 个字符" }).optional(),
  bio: z.string().max(500, { message: "个人简介不能超过 500 个字符" }).optional(),
})

type ProfileFormValues = z.infer<typeof profileSchema>

interface UserProfile {
  email: string
  display_name?: string
  bio?: string
}

export default function ProfilePage() {
  const router = useRouter()
  const [isLoading, setIsLoading] = useState(false)
  const [isDeleting, setIsDeleting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState(false)
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)

  const form = useForm<ProfileFormValues>({
    resolver: zodResolver(profileSchema),
    defaultValues: {
      email: "",
      display_name: "",
      bio: "",
    },
  })

  useEffect(() => {
    async function loadProfile() {
      try {
        const profile = await apiRequest<UserProfile>("/v1/account/profile")
        form.reset({
          email: profile.email,
          display_name: profile.display_name || "",
          bio: profile.bio || "",
        })
      } catch (err) {
        console.error("Failed to load profile:", err)
        // Cookie may be expired — redirect to login.
        router.push("/auth/login" as any)
      }
    }

    loadProfile()
  }, [router, form])

  async function onSubmit(values: ProfileFormValues) {
    setIsLoading(true)
    setError(null)
    setSuccess(false)

    try {
      await apiRequest("/v1/account/profile", {
        method: "PATCH",
        body: JSON.stringify({
          display_name: values.display_name || undefined,
          bio: values.bio || undefined,
        }),
      })

      setSuccess(true)
      setTimeout(() => setSuccess(false), 3000)
    } catch (err) {
      setError(err instanceof Error ? err.message : "更新失败，请稍后重试")
    } finally {
      setIsLoading(false)
    }
  }

  async function handleDeleteAccount() {
    if (!showDeleteConfirm) {
      setShowDeleteConfirm(true)
      return
    }

    setIsDeleting(true)
    setError(null)

    try {
      await apiRequest("/v1/account", {
        method: "DELETE",
      })

      // Cookie is cleared server-side by the delete-account endpoint.
      router.push("/")
    } catch (err) {
      setError(err instanceof Error ? err.message : "删除账号失败")
      setIsDeleting(false)
      setShowDeleteConfirm(false)
    }
  }

  return (
    <main className="flex-1 container max-w-3xl py-8">
      <Card>
        <CardHeader>
          <CardTitle>个人资料</CardTitle>
          <CardDescription>管理您的账号信息</CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          {error && (
            <div className="bg-destructive/15 text-destructive text-sm p-3 rounded-md">
              {error}
            </div>
          )}

          {success && (
            <div className="bg-green-500/15 text-green-600 dark:text-green-400 text-sm p-3 rounded-md">
              资料更新成功！
            </div>
          )}

          <Form {...form}>
            <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
              <FormField
                control={form.control}
                name="email"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>邮箱</FormLabel>
                    <FormControl>
                      <Input type="email" disabled {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="display_name"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>显示名称</FormLabel>
                    <FormControl>
                      <Input
                        type="text"
                        placeholder="您的显示名称"
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
                name="bio"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>个人简介</FormLabel>
                    <FormControl>
                      <Textarea
                        placeholder="介绍一下您自己"
                        disabled={isLoading}
                        {...field}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <Button type="submit" disabled={isLoading}>
                {isLoading ? "保存中..." : "保存更改"}
              </Button>
            </form>
          </Form>

          <div className="pt-6 border-t border-border">
            <h3 className="text-lg font-semibold text-destructive mb-2">危险区域</h3>
            <p className="text-sm text-muted-foreground mb-4">
              删除账号后将进入 30 天冷静期，期间可以恢复账号。
            </p>

            {showDeleteConfirm ? (
              <div className="space-y-3 p-4 border border-destructive rounded-md">
                <p className="text-sm font-medium">确定要删除账号吗？此操作无法撤销（30 天冷静期后）。</p>
                <div className="flex gap-3">
                  <Button
                    variant="destructive"
                    onClick={handleDeleteAccount}
                    disabled={isDeleting}
                  >
                    {isDeleting ? "删除中..." : "确认删除"}
                  </Button>
                  <Button
                    variant="outline"
                    onClick={() => setShowDeleteConfirm(false)}
                    disabled={isDeleting}
                  >
                    取消
                  </Button>
                </div>
              </div>
            ) : (
              <Button variant="destructive" onClick={handleDeleteAccount}>
                删除账号
              </Button>
            )}
          </div>
        </CardContent>
      </Card>
    </main>
  )
}
