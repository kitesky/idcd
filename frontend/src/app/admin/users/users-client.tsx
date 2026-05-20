"use client"

import { useState } from "react"
import { useRouter, usePathname } from "next/navigation"
import { useTranslations } from "next-intl"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"

interface User {
  id: string; email: string; status: string; plan: string
  monitor_count: number; created_at: string
}

interface UsersResp { users: User[]; total: number; page: number; per_page: number }

const planVariant:   Record<string, "default" | "secondary" | "outline"> = { enterprise: "default", team: "default", pro: "secondary", free: "outline" }
const statusVariant: Record<string, "default" | "destructive" | "secondary"> = { active: "default", suspended: "destructive", deleted: "secondary" }

export function UsersClient({
  initialData,
  initialPage,
  initialQ,
}: {
  initialData: UsersResp
  initialPage: number
  initialQ: string
}) {
  const t = useTranslations("admin")
  const router = useRouter()
  const pathname = usePathname()
  const [inputQ, setInputQ] = useState(initialQ)

  const data = initialData
  const page = initialPage
  const q = initialQ
  const totalPages = data ? Math.ceil(data.total / 20) : 1

  function navigate(nextPage: number, nextQ: string) {
    const params = new URLSearchParams()
    if (nextPage > 1) params.set("page", String(nextPage))
    if (nextQ) params.set("q", nextQ)
    const qs = params.toString()
    router.push((qs ? `${pathname}?${qs}` : pathname) as never)
  }

  function statusLabel(status: string): string {
    const key = `users.status.${status}` as Parameters<typeof t>[0]
    try { return t(key) } catch { return status }
  }

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold">{t("users.title")}</h1>

      <form
        onSubmit={e => { e.preventDefault(); navigate(1, inputQ) }}
        className="flex gap-2"
      >
        <Input
          placeholder={t("users.searchPlaceholder")}
          value={inputQ}
          onChange={e => setInputQ(e.target.value)}
          className="w-64"
        />
        <Button type="submit" size="sm">{t("users.search")}</Button>
        {q && (
          <Button
            type="button"
            size="sm"
            variant="ghost"
            onClick={() => { setInputQ(""); navigate(1, "") }}
          >
            {t("users.clearSearch")}
          </Button>
        )}
      </form>

      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">
            {data ? t("users.totalUsers", { total: data.total }) : t("users.loading")}
          </CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("users.table.email")}</TableHead>
                <TableHead>{t("users.table.status")}</TableHead>
                <TableHead>{t("users.table.plan")}</TableHead>
                <TableHead>{t("users.table.monitorCount")}</TableHead>
                <TableHead>{t("users.table.createdAt")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {data?.users.map(u => (
                <TableRow key={u.id}>
                  <TableCell className="font-medium">{u.email}</TableCell>
                  <TableCell>
                    <Badge variant={statusVariant[u.status] ?? "secondary"}>
                      {statusLabel(u.status)}
                    </Badge>
                  </TableCell>
                  <TableCell><Badge variant={planVariant[u.plan] ?? "outline"}>{u.plan}</Badge></TableCell>
                  <TableCell>{u.monitor_count}</TableCell>
                  <TableCell className="text-xs text-muted-foreground">
                    {new Date(u.created_at).toLocaleDateString()}
                  </TableCell>
                </TableRow>
              ))}
              {data?.users.length === 0 && (
                <TableRow>
                  <TableCell colSpan={5} className="py-8 text-center text-muted-foreground">
                    {t("users.noUsers")}
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      {data && totalPages > 1 && (
        <div className="flex items-center justify-end gap-2 text-sm">
          <Button size="sm" variant="outline" disabled={page <= 1} onClick={() => navigate(page - 1, q)}>
            {t("users.pagination.prev")}
          </Button>
          <span className="text-muted-foreground">{page} / {totalPages}</span>
          <Button size="sm" variant="outline" disabled={page >= totalPages} onClick={() => navigate(page + 1, q)}>
            {t("users.pagination.next")}
          </Button>
        </div>
      )}
    </div>
  )
}
