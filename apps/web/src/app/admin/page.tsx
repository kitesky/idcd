"use client"

import { useState } from "react"
import { toast } from "sonner"
import { useTranslations } from "next-intl"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"

const ADMIN_TOKEN =
  typeof process !== "undefined" ? (process.env.NEXT_PUBLIC_ADMIN_TOKEN ?? "") : ""

export default function AdminRoot() {
  const t = useTranslations("admin")
  const [loading, setLoading] = useState(false)

  async function handleTestEmail() {
    setLoading(true)
    try {
      const res = await fetch(
        "/internal/admin/test-email?to=admin@example.com",
        {
          method: "POST",
          headers: {
            Authorization: `Bearer ${ADMIN_TOKEN}`,
          },
        }
      )
      if (res.ok) {
        const data = await res.json()
        toast.success(data?.data?.message ?? t("home.emailSent"))
      } else {
        toast.error(t("home.requestFailed"))
      }
    } catch {
      toast.error(t("home.networkError"))
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">{t("home.title")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t("home.subtitle")}</p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{t("home.configStatus")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="flex items-center justify-between">
            <span className="text-sm">{t("home.emailService")}</span>
            <div className="flex items-center gap-2">
              <Badge variant="outline" className="text-yellow-600">
                {t("home.needsConfig")}
              </Badge>
              <Button
                size="sm"
                variant="outline"
                onClick={handleTestEmail}
                disabled={loading}
              >
                {loading ? t("home.sending") : t("home.sendTestEmail")}
              </Button>
            </div>
          </div>
          <p className="text-xs text-muted-foreground">
            配置方法：在{" "}
            <code className="font-mono">config/dev.env.yaml</code> 添加{" "}
            <code className="font-mono">notifier.smtp</code>{" "}
            段落（参见{" "}
            <code className="font-mono">dev.env.example.yaml</code>），然后启动
            notifier 服务：
            <code className="font-mono">
              go run ./apps/notifier/cmd/notifier/
            </code>
          </p>
        </CardContent>
      </Card>
    </div>
  )
}
