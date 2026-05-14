"use client"

import { useCallback, useState } from "react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { cn } from "@/lib/utils"

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"
const ADMIN_TOKEN = process.env.NEXT_PUBLIC_ADMIN_TOKEN ?? ""

type InvitationStatus = "pending" | "approved" | "used" | "revoked"

interface BetaInvitation {
  id: string
  code: string
  email: string | null
  status: InvitationStatus
  requested_by: string | null
  used_by?: string | null
  expires_at: string | null
  created_at: string
}

const MOCK_INVITATIONS: BetaInvitation[] = [
  {
    id: "bid_001",
    code: "ABC12345",
    email: "dev@example.com",
    status: "pending",
    requested_by: "usr_xxx",
    expires_at: null,
    created_at: "2026-05-14T10:00:00Z",
  },
  {
    id: "bid_002",
    code: "XY67890Z",
    email: null,
    status: "approved",
    requested_by: null,
    expires_at: "2026-06-14T10:00:00Z",
    created_at: "2026-05-10T09:00:00Z",
  },
  {
    id: "bid_003",
    code: "PQRS1234",
    email: "alice@corp.com",
    status: "used",
    requested_by: null,
    used_by: "usr_yyy",
    expires_at: null,
    created_at: "2026-05-01T08:00:00Z",
  },
]

const STATUS_FILTER_OPTIONS: Array<{ label: string; value: string }> = [
  { label: "全部", value: "" },
  { label: "Pending", value: "pending" },
  { label: "Approved", value: "approved" },
  { label: "Used", value: "used" },
  { label: "Revoked", value: "revoked" },
]

const EXPIRY_OPTIONS = [
  { label: "7 天", value: "7" },
  { label: "30 天", value: "30" },
  { label: "90 天", value: "90" },
  { label: "永久", value: "0" },
]

type BadgeVariant = "default" | "secondary" | "destructive" | "outline" | "success" | "warning"

const statusBadgeVariant: Record<InvitationStatus, BadgeVariant> = {
  pending: "default",
  approved: "success",
  used: "secondary",
  revoked: "destructive",
}

function formatDate(iso: string | null | undefined): string {
  if (!iso) return "—"
  try {
    return new Date(iso).toLocaleDateString("zh-CN")
  } catch {
    return iso
  }
}

export default function BetaInvitationsPage() {
  const [invitations, setInvitations] = useState<BetaInvitation[]>(MOCK_INVITATIONS)
  const [statusFilter, setStatusFilter] = useState("")
  const [actionLoading, setActionLoading] = useState<Record<string, boolean>>({})
  const [actionError, setActionError] = useState<Record<string, string>>({})
  const [toast, setToast] = useState<{ message: string; type: "success" | "error" } | null>(null)

  const [showDialog, setShowDialog] = useState(false)
  const [newEmail, setNewEmail] = useState("")
  const [newExpiry, setNewExpiry] = useState("30")
  const [creating, setCreating] = useState(false)

  const showToast = useCallback((message: string, type: "success" | "error" = "success") => {
    setToast({ message, type })
    setTimeout(() => setToast(null), 3000)
  }, [])

  const filteredInvitations = statusFilter
    ? invitations.filter((inv) => inv.status === statusFilter)
    : invitations

  const handleAction = useCallback(
    async (id: string, action: "approve" | "revoke") => {
      setActionLoading((prev) => ({ ...prev, [id]: true }))
      setActionError((prev) => {
        const next = { ...prev }
        delete next[id]
        return next
      })
      try {
        const res = await fetch(`${API_BASE}/v1/admin/beta-invitations/${id}`, {
          method: "PATCH",
          headers: {
            "Content-Type": "application/json",
            Authorization: `Bearer ${ADMIN_TOKEN}`,
          },
          body: JSON.stringify({ action }),
        })
        if (!res.ok) {
          const body = await res.json().catch(() => ({}))
          throw new Error(body?.error?.message ?? `HTTP ${res.status}`)
        }
        const updated = await res.json()
        setInvitations((prev) =>
          prev.map((inv) => (inv.id === id ? { ...inv, ...updated.data } : inv))
        )
        showToast(action === "approve" ? "已审批" : "已撤销")
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        setActionError((prev) => ({ ...prev, [id]: msg }))
        showToast(msg, "error")
      } finally {
        setActionLoading((prev) => ({ ...prev, [id]: false }))
      }
    },
    [showToast]
  )

  const handleCreate = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault()
      setCreating(true)
      try {
        const body: Record<string, unknown> = {}
        if (newEmail.trim()) body.email = newEmail.trim()
        if (newExpiry !== "0") body.expires_in_days = Number(newExpiry)
        const res = await fetch(`${API_BASE}/v1/admin/beta-invitations`, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            Authorization: `Bearer ${ADMIN_TOKEN}`,
          },
          body: JSON.stringify(body),
        })
        if (!res.ok) {
          const resp = await res.json().catch(() => ({}))
          throw new Error(resp?.error?.message ?? `HTTP ${res.status}`)
        }
        const created = await res.json()
        setInvitations((prev) => [created.data, ...prev])
        setShowDialog(false)
        setNewEmail("")
        setNewExpiry("30")
        showToast("邀请码已创建")
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        showToast(msg, "error")
      } finally {
        setCreating(false)
      }
    },
    [newEmail, newExpiry, showToast]
  )

  return (
    <div className="space-y-4">
      {toast && (
        <div
          className={cn(
            "fixed top-4 right-4 z-50 rounded-md px-4 py-3 text-sm shadow-lg",
            toast.type === "success"
              ? "bg-green-600 text-white"
              : "bg-destructive text-destructive-foreground"
          )}
        >
          {toast.message}
        </div>
      )}

      {showDialog && (
        <div className="fixed inset-0 z-40 flex items-center justify-center">
          <div
            className="absolute inset-0 bg-black/60"
            onClick={() => setShowDialog(false)}
          />
          <div className="relative z-50 w-full max-w-sm rounded-lg border border-border bg-card p-6 shadow-xl">
            <h2 className="text-lg font-semibold mb-4">创建邀请码</h2>
            <form onSubmit={handleCreate} className="space-y-4">
              <div className="space-y-1">
                <label className="text-sm font-medium">邮箱限制（可选）</label>
                <input
                  type="email"
                  value={newEmail}
                  onChange={(e) => setNewEmail(e.target.value)}
                  placeholder="user@example.com"
                  className="flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
                />
                <p className="text-xs text-muted-foreground">留空则任何人可兑换</p>
              </div>
              <div className="space-y-1">
                <label className="text-sm font-medium">有效期</label>
                <select
                  value={newExpiry}
                  onChange={(e) => setNewExpiry(e.target.value)}
                  className="flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
                >
                  {EXPIRY_OPTIONS.map((opt) => (
                    <option key={opt.value} value={opt.value}>
                      {opt.label}
                    </option>
                  ))}
                </select>
              </div>
              <div className="flex justify-end gap-2 pt-2">
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  onClick={() => setShowDialog(false)}
                >
                  取消
                </Button>
                <Button type="submit" size="sm" disabled={creating}>
                  {creating ? "创建中…" : "确认创建"}
                </Button>
              </div>
            </form>
          </div>
        </div>
      )}

      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Beta 邀请码管理</h1>
        <Button size="sm" onClick={() => setShowDialog(true)}>
          创建邀请码
        </Button>
      </div>

      <div className="flex gap-1">
        {STATUS_FILTER_OPTIONS.map((opt) => (
          <Button
            key={opt.value}
            size="sm"
            variant={statusFilter === opt.value ? "default" : "outline"}
            onClick={() => setStatusFilter(opt.value)}
          >
            {opt.label}
          </Button>
        ))}
      </div>

      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">
            共 {filteredInvitations.length} 条邀请码
          </CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>邀请码</TableHead>
                <TableHead>邮箱限制</TableHead>
                <TableHead>状态</TableHead>
                <TableHead>申请人</TableHead>
                <TableHead>到期时间</TableHead>
                <TableHead>操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filteredInvitations.length === 0 ? (
                <TableRow>
                  <TableCell
                    colSpan={6}
                    className="text-center text-muted-foreground py-8"
                  >
                    暂无邀请码
                  </TableCell>
                </TableRow>
              ) : (
                filteredInvitations.map((inv) => (
                  <TableRow key={inv.id}>
                    <TableCell className="font-mono text-sm font-medium">
                      {inv.code}
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {inv.email ?? "—"}
                    </TableCell>
                    <TableCell>
                      <Badge variant={statusBadgeVariant[inv.status]}>
                        {inv.status}
                      </Badge>
                    </TableCell>
                    <TableCell className="font-mono text-xs text-muted-foreground">
                      {inv.requested_by ?? "—"}
                    </TableCell>
                    <TableCell className="text-xs text-muted-foreground">
                      {formatDate(inv.expires_at)}
                    </TableCell>
                    <TableCell>
                      {inv.status === "pending" && (
                        <div className="flex flex-col gap-1">
                          <Button
                            size="sm"
                            variant="outline"
                            disabled={actionLoading[inv.id]}
                            onClick={() => handleAction(inv.id, "approve")}
                          >
                            {actionLoading[inv.id] ? "处理中…" : "审批"}
                          </Button>
                          {actionError[inv.id] && (
                            <span className="text-xs text-destructive">
                              {actionError[inv.id]}
                            </span>
                          )}
                        </div>
                      )}
                      {inv.status === "approved" && (
                        <div className="flex flex-col gap-1">
                          <Button
                            size="sm"
                            variant="outline"
                            disabled={actionLoading[inv.id]}
                            onClick={() => handleAction(inv.id, "revoke")}
                          >
                            {actionLoading[inv.id] ? "处理中…" : "撤销"}
                          </Button>
                          {actionError[inv.id] && (
                            <span className="text-xs text-destructive">
                              {actionError[inv.id]}
                            </span>
                          )}
                        </div>
                      )}
                      {(inv.status === "used" || inv.status === "revoked") && (
                        <span className="text-muted-foreground text-sm">—</span>
                      )}
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  )
}
