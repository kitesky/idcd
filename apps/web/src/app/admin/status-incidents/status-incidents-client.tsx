"use client"

import { useCallback, useState } from "react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { cn } from "@/lib/utils"
import type { AdminIncident, CreateIncidentInput, IncidentSeverity } from "./types"

const SEVERITIES: IncidentSeverity[] = ["degradation", "partial_outage", "outage", "maintenance"]
const SEVERITY_LABEL: Record<IncidentSeverity, string> = {
  degradation: "降级",
  partial_outage: "部分中断",
  outage: "中断",
  maintenance: "维护",
}
const SEVERITY_VARIANT: Record<IncidentSeverity, "destructive" | "warning" | "secondary"> = {
  outage: "destructive",
  partial_outage: "destructive",
  degradation: "warning",
  maintenance: "secondary",
}

// Service keys we expect on this self-status page. Free-text fallback so
// you can also log incidents for service_keys not on this list (e.g. a new
// service that isn't yet registered with the SERVICE_DISPLAY map on /status).
const KNOWN_SERVICE_KEYS = ["api", "cert-svc", "gateway", "aggregator", "notifier", "web"]

// Convert a UI datetime-local string ("2026-05-20T12:34") to a real ISO string
// in the user's local timezone. The DB column is TIMESTAMPTZ so we want the
// browser's intent (12:34 local) to round-trip correctly.
function toISO(local: string): string {
  if (!local) return ""
  const d = new Date(local)
  return d.toISOString()
}

// Inverse: turn an ISO string back into a datetime-local value (UTC →
// browser's local) for editing. Returns "" on falsy input.
function toLocalInput(iso?: string | null): string {
  if (!iso) return ""
  const d = new Date(iso)
  const pad = (n: number) => String(n).padStart(2, "0")
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

export function StatusIncidentsClient({ initialIncidents }: { initialIncidents: AdminIncident[] }) {
  const [incidents, setIncidents] = useState(initialIncidents)
  const [editing, setEditing] = useState<AdminIncident | null>(null)
  const [showDialog, setShowDialog] = useState(false)
  const [saving, setSaving] = useState(false)
  const [toast, setToast] = useState<{ ok: boolean; msg: string } | null>(null)

  // Form state — shared by create + edit.
  const [serviceKey, setServiceKey] = useState("api")
  const [startedAt, setStartedAt] = useState("")
  const [endedAt, setEndedAt] = useState("")
  const [severity, setSeverity] = useState<IncidentSeverity>("degradation")
  const [title, setTitle] = useState("")
  const [summary, setSummary] = useState("")

  const showToast = useCallback((msg: string, ok = true) => {
    setToast({ ok, msg })
    setTimeout(() => setToast(null), 3000)
  }, [])

  const openCreate = useCallback(() => {
    setEditing(null)
    setServiceKey("api")
    setStartedAt(toLocalInput(new Date().toISOString()))
    setEndedAt("")
    setSeverity("degradation")
    setTitle("")
    setSummary("")
    setShowDialog(true)
  }, [])

  const openEdit = useCallback((inc: AdminIncident) => {
    setEditing(inc)
    setServiceKey(inc.service_key)
    setStartedAt(toLocalInput(inc.started_at))
    setEndedAt(toLocalInput(inc.ended_at))
    setSeverity(inc.severity)
    setTitle(inc.title)
    setSummary(inc.summary ?? "")
    setShowDialog(true)
  }, [])

  const submit = useCallback(async (e: React.FormEvent) => {
    e.preventDefault()
    setSaving(true)
    try {
      const body: Partial<CreateIncidentInput> = {
        service_key: serviceKey,
        started_at: toISO(startedAt),
        ended_at: endedAt ? toISO(endedAt) : null,
        severity,
        title,
        summary: summary || undefined,
      }
      const url = editing ? `/api/admin/status-incidents/${editing.id}` : "/api/admin/status-incidents"
      const method = editing ? "PATCH" : "POST"
      const res = await fetch(url, {
        method,
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      })
      if (!res.ok) {
        const b = await res.json().catch(() => ({}))
        throw new Error(b?.error?.message ?? `HTTP ${res.status}`)
      }
      // Refetch list — simpler than reconciling a single PATCH return shape.
      const listRes = await fetch("/api/admin/status-incidents", { cache: "no-store" })
      const listJson = await listRes.json()
      setIncidents(listJson.incidents ?? [])
      setShowDialog(false)
      showToast(editing ? "事件已更新" : "事件已创建")
    } catch (err) {
      showToast(err instanceof Error ? err.message : String(err), false)
    } finally {
      setSaving(false)
    }
  }, [editing, serviceKey, startedAt, endedAt, severity, title, summary, showToast])

  const remove = useCallback(async (inc: AdminIncident) => {
    if (!confirm(`确定删除事件 "${inc.title}"？此操作不可恢复。`)) return
    try {
      const res = await fetch(`/api/admin/status-incidents/${inc.id}`, { method: "DELETE" })
      if (!res.ok) {
        const b = await res.json().catch(() => ({}))
        throw new Error(b?.error?.message ?? `HTTP ${res.status}`)
      }
      setIncidents(p => p.filter(x => x.id !== inc.id))
      showToast("事件已删除")
    } catch (err) {
      showToast(err instanceof Error ? err.message : String(err), false)
    }
  }, [showToast])

  return (
    <div className="space-y-4">
      {toast && (
        <div className={cn(
          "fixed right-4 top-4 z-50 rounded-md px-4 py-3 text-sm shadow-lg",
          toast.ok ? "bg-success text-success-foreground" : "bg-destructive text-destructive-foreground",
        )}>
          {toast.msg}
        </div>
      )}

      {showDialog && (
        <div className="fixed inset-0 z-40 flex items-center justify-center">
          <div className="absolute inset-0 bg-black/60" onClick={() => setShowDialog(false)} />
          <div className="relative z-50 w-full max-w-md rounded-lg border bg-card p-6 shadow-xl">
            <h2 className="mb-4 text-lg font-semibold">{editing ? "编辑事件" : "新建事件"}</h2>
            <form onSubmit={submit} className="space-y-4">
              <div className="space-y-1">
                <label className="text-sm font-medium">服务</label>
                <Input
                  list="known-services"
                  value={serviceKey}
                  onChange={e => setServiceKey(e.target.value)}
                  placeholder="api"
                  required
                />
                <datalist id="known-services">
                  {KNOWN_SERVICE_KEYS.map(k => <option key={k} value={k} />)}
                </datalist>
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1">
                  <label className="text-sm font-medium">开始时间</label>
                  <Input type="datetime-local" value={startedAt} onChange={e => setStartedAt(e.target.value)} required />
                </div>
                <div className="space-y-1">
                  <label className="text-sm font-medium">恢复时间（可空）</label>
                  <Input type="datetime-local" value={endedAt} onChange={e => setEndedAt(e.target.value)} />
                </div>
              </div>
              <div className="space-y-1">
                <label className="text-sm font-medium">严重程度</label>
                <select
                  value={severity}
                  onChange={e => setSeverity(e.target.value as IncidentSeverity)}
                  className="flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm"
                >
                  {SEVERITIES.map(s => <option key={s} value={s}>{SEVERITY_LABEL[s]}</option>)}
                </select>
              </div>
              <div className="space-y-1">
                <label className="text-sm font-medium">标题</label>
                <Input value={title} onChange={e => setTitle(e.target.value)} required maxLength={200} />
              </div>
              <div className="space-y-1">
                <label className="text-sm font-medium">说明（可选）</label>
                <textarea
                  value={summary}
                  onChange={e => setSummary(e.target.value)}
                  rows={3}
                  className="flex w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-sm"
                />
              </div>
              <div className="flex justify-end gap-2 pt-2">
                <Button type="button" variant="ghost" size="sm" onClick={() => setShowDialog(false)}>取消</Button>
                <Button type="submit" size="sm" disabled={saving}>{saving ? "保存中..." : "保存"}</Button>
              </div>
            </form>
          </div>
        </div>
      )}

      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">状态事件管理</h1>
          <p className="mt-1 text-sm text-muted-foreground">手动录入 idcd.com/status 公开的事件时间线（v1 手动；自动告警接入是 v2）</p>
        </div>
        <Button size="sm" onClick={openCreate}>新建事件</Button>
      </div>

      <Card>
        <CardContent className="p-0">
          {incidents.length === 0 ? (
            <p className="p-8 text-center text-sm text-muted-foreground">暂无事件记录</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>服务</TableHead>
                  <TableHead>严重程度</TableHead>
                  <TableHead>标题</TableHead>
                  <TableHead>开始</TableHead>
                  <TableHead>恢复</TableHead>
                  <TableHead className="text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {incidents.map(inc => (
                  <TableRow key={inc.id}>
                    <TableCell className="font-mono text-xs">{inc.service_key}</TableCell>
                    <TableCell><Badge variant={SEVERITY_VARIANT[inc.severity]}>{SEVERITY_LABEL[inc.severity]}</Badge></TableCell>
                    <TableCell className="max-w-xs truncate">{inc.title}</TableCell>
                    <TableCell className="whitespace-nowrap text-xs">{new Date(inc.started_at).toLocaleString("zh-CN")}</TableCell>
                    <TableCell className="whitespace-nowrap text-xs">
                      {inc.ended_at ? new Date(inc.ended_at).toLocaleString("zh-CN") : <span className="text-warning">进行中</span>}
                    </TableCell>
                    <TableCell className="text-right">
                      <Button variant="ghost" size="sm" onClick={() => openEdit(inc)}>编辑</Button>
                      <Button variant="ghost" size="sm" onClick={() => remove(inc)} className="text-destructive hover:text-destructive">删除</Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
