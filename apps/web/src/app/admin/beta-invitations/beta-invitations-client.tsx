"use client"

import { useCallback, useState } from "react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { cn } from "@/lib/utils"
import type { BetaInvitation, InvitationStatus } from "./page"

const EXPIRY_OPTIONS = [{ label: "7 天", value: "7" }, { label: "30 天", value: "30" }, { label: "90 天", value: "90" }, { label: "永久", value: "0" }]
const statusVariant: Record<InvitationStatus, "default" | "secondary" | "destructive" | "outline"> = {
  pending: "default", approved: "outline", used: "secondary", revoked: "destructive",
}

function formatDate(iso: string | null | undefined) {
  if (!iso) return "—"
  try { return new Date(iso).toLocaleDateString("zh-CN") } catch { return iso }
}

export function BetaInvitationsClient({ initialInvitations }: { initialInvitations: BetaInvitation[] }) {
  const [invitations, setInvitations] = useState(initialInvitations)
  const [statusFilter, setStatusFilter] = useState("")
  const [actionLoading, setActionLoading] = useState<Record<string, boolean>>({})
  const [toast, setToast] = useState<{ message: string; ok: boolean } | null>(null)
  const [showDialog, setShowDialog] = useState(false)
  const [newEmail, setNewEmail] = useState("")
  const [newExpiry, setNewExpiry] = useState("30")
  const [creating, setCreating] = useState(false)

  const showToast = useCallback((message: string, ok = true) => {
    setToast({ message, ok })
    setTimeout(() => setToast(null), 3000)
  }, [])

  const filtered = statusFilter ? invitations.filter(i => i.status === statusFilter) : invitations

  const handleAction = useCallback(async (id: string, action: "approve" | "revoke") => {
    setActionLoading(p => ({ ...p, [id]: true }))
    try {
      const res = await fetch(`/api/admin/beta-invitations/${id}`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ action }),
      })
      if (!res.ok) { const b = await res.json().catch(() => ({})); throw new Error(b?.error?.message ?? `HTTP ${res.status}`) }
      const updated = await res.json()
      setInvitations(p => p.map(i => i.id === id ? { ...i, ...updated.data } : i))
      showToast(action === "approve" ? "已审批" : "已撤销")
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : String(err), false)
    } finally {
      setActionLoading(p => ({ ...p, [id]: false }))
    }
  }, [showToast])

  const handleCreate = useCallback(async (e: React.FormEvent) => {
    e.preventDefault(); setCreating(true)
    try {
      const body: Record<string, unknown> = {}
      if (newEmail.trim()) body.email = newEmail.trim()
      if (newExpiry !== "0") body.expires_in_days = Number(newExpiry)
      const res = await fetch("/api/admin/beta-invitations", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      })
      if (!res.ok) { const b = await res.json().catch(() => ({})); throw new Error(b?.error?.message ?? `HTTP ${res.status}`) }
      const created = await res.json()
      setInvitations(p => [created.data, ...p])
      setShowDialog(false); setNewEmail(""); setNewExpiry("30")
      showToast("邀请码已创建")
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : String(err), false)
    } finally { setCreating(false) }
  }, [newEmail, newExpiry, showToast])

  return (
    <div className="space-y-4">
      {toast && (
        <div className={cn("fixed right-4 top-4 z-50 rounded-md px-4 py-3 text-sm shadow-lg", toast.ok ? "bg-green-600 text-white" : "bg-destructive text-destructive-foreground")}>
          {toast.message}
        </div>
      )}

      {showDialog && (
        <div className="fixed inset-0 z-40 flex items-center justify-center">
          <div className="absolute inset-0 bg-black/60" onClick={() => setShowDialog(false)} />
          <div className="relative z-50 w-full max-w-sm rounded-lg border bg-card p-6 shadow-xl">
            <h2 className="mb-4 text-lg font-semibold">创建邀请码</h2>
            <form onSubmit={handleCreate} className="space-y-4">
              <div className="space-y-1">
                <label className="text-sm font-medium">邮箱限制（可选）</label>
                <Input type="email" value={newEmail} onChange={e => setNewEmail(e.target.value)} placeholder="user@example.com" />
                <p className="text-xs text-muted-foreground">留空则任何人可兑换</p>
              </div>
              <div className="space-y-1">
                <label className="text-sm font-medium">有效期</label>
                <select value={newExpiry} onChange={e => setNewExpiry(e.target.value)}
                  className="flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring">
                  {EXPIRY_OPTIONS.map(o => <option key={o.value} value={o.value}>{o.label}</option>)}
                </select>
              </div>
              <div className="flex justify-end gap-2 pt-2">
                <Button type="button" variant="ghost" size="sm" onClick={() => setShowDialog(false)}>取消</Button>
                <Button type="submit" size="sm" disabled={creating}>{creating ? "创建中…" : "确认创建"}</Button>
              </div>
            </form>
          </div>
        </div>
      )}

      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Beta 邀请码管理</h1>
        <Button size="sm" onClick={() => setShowDialog(true)}>创建邀请码</Button>
      </div>

      <div className="flex gap-1">
        {[{ label: "全部", value: "" }, { label: "Pending", value: "pending" }, { label: "Approved", value: "approved" }, { label: "Used", value: "used" }, { label: "Revoked", value: "revoked" }].map(o => (
          <Button key={o.value} size="sm" variant={statusFilter === o.value ? "default" : "outline"} onClick={() => setStatusFilter(o.value)}>{o.label}</Button>
        ))}
      </div>

      <Card>
        <CardHeader className="pb-2"><CardTitle className="text-base">共 {filtered.length} 条邀请码</CardTitle></CardHeader>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>邀请码</TableHead><TableHead>邮箱限制</TableHead>
                <TableHead>状态</TableHead><TableHead>申请人</TableHead>
                <TableHead>到期时间</TableHead><TableHead>操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filtered.length === 0 ? (
                <TableRow><TableCell colSpan={6} className="py-8 text-center text-muted-foreground">暂无邀请码</TableCell></TableRow>
              ) : filtered.map(inv => (
                <TableRow key={inv.id}>
                  <TableCell className="font-mono text-sm font-medium">{inv.code}</TableCell>
                  <TableCell className="text-sm text-muted-foreground">{inv.email ?? "—"}</TableCell>
                  <TableCell><Badge variant={statusVariant[inv.status]}>{inv.status}</Badge></TableCell>
                  <TableCell className="font-mono text-xs text-muted-foreground">{inv.requested_by ?? "—"}</TableCell>
                  <TableCell className="text-xs text-muted-foreground">{formatDate(inv.expires_at)}</TableCell>
                  <TableCell>
                    {inv.status === "pending" && (
                      <Button size="sm" variant="outline" disabled={actionLoading[inv.id]} onClick={() => handleAction(inv.id, "approve")}>
                        {actionLoading[inv.id] ? "处理中…" : "审批"}
                      </Button>
                    )}
                    {inv.status === "approved" && (
                      <Button size="sm" variant="outline" disabled={actionLoading[inv.id]} onClick={() => handleAction(inv.id, "revoke")}>
                        {actionLoading[inv.id] ? "处理中…" : "撤销"}
                      </Button>
                    )}
                    {(inv.status === "used" || inv.status === "revoked") && <span className="text-sm text-muted-foreground">—</span>}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  )
}
