"use client"

import { useCallback, useEffect, useState } from "react"
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

const API_BASE   = process.env.NEXT_PUBLIC_API_URL   ?? "http://localhost:8080"
const ADMIN_TOKEN = process.env.NEXT_PUBLIC_ADMIN_TOKEN ?? ""

const planVariant:   Record<string, "default" | "secondary" | "outline"> = { enterprise: "default", team: "default", pro: "secondary", free: "outline" }
const statusVariant: Record<string, "default" | "destructive" | "secondary"> = { active: "default", suspended: "destructive", deleted: "secondary" }

export default function UsersPage() {
  const [data, setData]   = useState<UsersResp | null>(null)
  const [page, setPage]   = useState(1)
  const [q, setQ]         = useState("")
  const [inputQ, setInputQ] = useState("")
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(() => {
    const params = new URLSearchParams({ page: String(page), per_page: "20" })
    if (q) params.set("q", q)
    fetch(`${API_BASE}/internal/admin/users?${params}`, { headers: { Authorization: `Bearer ${ADMIN_TOKEN}` } })
      .then(r => { if (!r.ok) throw new Error(`HTTP ${r.status}`); return r.json() })
      .then(j => setData(j.data))
      .catch(e => setError(e.message))
  }, [page, q])

  useEffect(() => { load() }, [load])

  const totalPages = data ? Math.ceil(data.total / 20) : 1

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold">用户管理</h1>

      <form onSubmit={e => { e.preventDefault(); setPage(1); setQ(inputQ) }} className="flex gap-2">
        <Input placeholder="搜索邮箱…" value={inputQ} onChange={e => setInputQ(e.target.value)} className="w-64" />
        <Button type="submit" size="sm">搜索</Button>
        {q && <Button type="button" size="sm" variant="ghost" onClick={() => { setInputQ(""); setQ(""); setPage(1) }}>清除</Button>}
      </form>

      {error && <p className="text-destructive">加载失败：{error}</p>}

      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">{data ? `共 ${data.total} 名用户` : "加载中…"}</CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>邮箱</TableHead><TableHead>状态</TableHead>
                <TableHead>套餐</TableHead><TableHead>监控数</TableHead><TableHead>注册时间</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {data?.users.map(u => (
                <TableRow key={u.id}>
                  <TableCell className="font-medium">{u.email}</TableCell>
                  <TableCell><Badge variant={statusVariant[u.status] ?? "secondary"}>{u.status}</Badge></TableCell>
                  <TableCell><Badge variant={planVariant[u.plan] ?? "outline"}>{u.plan}</Badge></TableCell>
                  <TableCell>{u.monitor_count}</TableCell>
                  <TableCell className="text-xs text-muted-foreground">{new Date(u.created_at).toLocaleDateString("zh-CN")}</TableCell>
                </TableRow>
              ))}
              {data?.users.length === 0 && (
                <TableRow><TableCell colSpan={5} className="py-8 text-center text-muted-foreground">暂无用户</TableCell></TableRow>
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      {data && totalPages > 1 && (
        <div className="flex items-center justify-end gap-2 text-sm">
          <Button size="sm" variant="outline" disabled={page <= 1} onClick={() => setPage(p => p - 1)}>上一页</Button>
          <span className="text-muted-foreground">{page} / {totalPages}</span>
          <Button size="sm" variant="outline" disabled={page >= totalPages} onClick={() => setPage(p => p + 1)}>下一页</Button>
        </div>
      )}
    </div>
  )
}
