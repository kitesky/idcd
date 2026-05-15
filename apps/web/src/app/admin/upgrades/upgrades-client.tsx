"use client"

import { useCallback, useState } from "react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Slider } from "@/components/ui/slider"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import type { UpgradeRollout } from "./page"

const statusVariant: Record<UpgradeRollout["status"], "default" | "secondary" | "destructive" | "outline"> = {
  active: "default",
  paused: "outline",
  completed: "secondary",
}

const statusLabel: Record<UpgradeRollout["status"], string> = {
  active: "进行中",
  paused: "已暂停",
  completed: "已完成",
}

function formatDate(iso: string) {
  try {
    return new Date(iso).toLocaleString("zh-CN", {
      month: "2-digit", day: "2-digit",
      hour: "2-digit", minute: "2-digit",
    })
  } catch {
    return iso
  }
}

export function UpgradesClient({ initialRollouts }: { initialRollouts: UpgradeRollout[] }) {
  const [rollouts, setRollouts] = useState(initialRollouts)
  const [toast, setToast] = useState<{ message: string; ok: boolean } | null>(null)
  const [actionLoading, setActionLoading] = useState<Record<string, boolean>>({})

  // Create dialog state
  const [showCreate, setShowCreate] = useState(false)
  const [creating, setCreating] = useState(false)
  const [form, setForm] = useState({ version: "", download_url: "", checksum: "", rollout_pct: 1 })

  const showToast = useCallback((message: string, ok = true) => {
    setToast({ message, ok })
    setTimeout(() => setToast(null), 3000)
  }, [])

  const handleCreate = useCallback(async (e: React.FormEvent) => {
    e.preventDefault()
    setCreating(true)
    try {
      const res = await fetch("/api/admin/upgrade-rollouts", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(form),
      })
      if (!res.ok) {
        const b = await res.json().catch(() => ({}))
        throw new Error(b?.error?.message ?? `HTTP ${res.status}`)
      }
      const created = await res.json()
      setRollouts(p => [created.data, ...p])
      setShowCreate(false)
      setForm({ version: "", download_url: "", checksum: "", rollout_pct: 1 })
      showToast("升级计划已创建")
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : String(err), false)
    } finally {
      setCreating(false)
    }
  }, [form, showToast])

  const handlePatch = useCallback(async (id: string, patch: Partial<Pick<UpgradeRollout, "rollout_pct" | "status">>) => {
    setActionLoading(p => ({ ...p, [id]: true }))
    try {
      const res = await fetch(`/api/admin/upgrade-rollouts/${id}`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(patch),
      })
      if (!res.ok) {
        const b = await res.json().catch(() => ({}))
        throw new Error(b?.error?.message ?? `HTTP ${res.status}`)
      }
      const updated = await res.json()
      setRollouts(p => p.map(r => r.id === id ? { ...r, ...updated.data } : r))
      showToast("已更新")
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : String(err), false)
    } finally {
      setActionLoading(p => ({ ...p, [id]: false }))
    }
  }, [showToast])

  return (
    <div className="space-y-4">
      {toast && (
        <div className={`fixed bottom-4 right-4 z-50 rounded-md px-4 py-2 text-sm text-white shadow-lg ${toast.ok ? "bg-green-600" : "bg-destructive"}`}>
          {toast.message}
        </div>
      )}

      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">共 {rollouts.length} 个升级计划</p>
        <Dialog open={showCreate} onOpenChange={setShowCreate}>
          <DialogTrigger asChild>
            <Button size="sm">新建升级计划</Button>
          </DialogTrigger>
          <DialogContent className="sm:max-w-md">
            <DialogHeader>
              <DialogTitle>新建 OTA 升级计划</DialogTitle>
            </DialogHeader>
            <form onSubmit={handleCreate} className="space-y-4">
              <div className="space-y-1">
                <Label htmlFor="version">版本号</Label>
                <Input
                  id="version"
                  placeholder="v1.2.0"
                  value={form.version}
                  onChange={e => setForm(p => ({ ...p, version: e.target.value }))}
                  required
                />
              </div>
              <div className="space-y-1">
                <Label htmlFor="download_url">下载地址</Label>
                <Input
                  id="download_url"
                  placeholder="https://releases.idcd.com/agent-v1.2.0"
                  value={form.download_url}
                  onChange={e => setForm(p => ({ ...p, download_url: e.target.value }))}
                  required
                />
              </div>
              <div className="space-y-1">
                <Label htmlFor="checksum">SHA-256 校验和</Label>
                <Input
                  id="checksum"
                  placeholder="sha256:abc123..."
                  value={form.checksum}
                  onChange={e => setForm(p => ({ ...p, checksum: e.target.value }))}
                  required
                />
              </div>
              <div className="space-y-2">
                <Label>灰度比例: {form.rollout_pct}%</Label>
                <Slider
                  min={1}
                  max={100}
                  step={1}
                  value={[form.rollout_pct]}
                  onValueChange={([v]) => setForm(p => ({ ...p, rollout_pct: v }))}
                />
                <div className="flex justify-between text-xs text-muted-foreground">
                  <span>1%</span>
                  <span>10%</span>
                  <span>100%</span>
                </div>
              </div>
              <DialogFooter>
                <Button type="button" variant="outline" onClick={() => setShowCreate(false)}>取消</Button>
                <Button type="submit" disabled={creating}>{creating ? "创建中..." : "创建"}</Button>
              </DialogFooter>
            </form>
          </DialogContent>
        </Dialog>
      </div>

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">升级计划列表</CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>版本</TableHead>
                <TableHead>灰度比例</TableHead>
                <TableHead>状态</TableHead>
                <TableHead>创建时间</TableHead>
                <TableHead className="text-right">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rollouts.length === 0 && (
                <TableRow>
                  <TableCell colSpan={5} className="text-center text-muted-foreground py-8">
                    暂无升级计划
                  </TableCell>
                </TableRow>
              )}
              {rollouts.map(r => (
                <TableRow key={r.id}>
                  <TableCell className="font-mono text-sm">{r.version}</TableCell>
                  <TableCell>
                    <div className="flex items-center gap-2">
                      <div className="w-24">
                        <Slider
                          min={1}
                          max={100}
                          step={1}
                          value={[r.rollout_pct]}
                          disabled={r.status !== "active" || actionLoading[r.id]}
                          onValueChange={([v]) => {
                            if (v !== r.rollout_pct) handlePatch(r.id, { rollout_pct: v })
                          }}
                        />
                      </div>
                      <span className="text-sm tabular-nums w-10">{r.rollout_pct}%</span>
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge variant={statusVariant[r.status]}>{statusLabel[r.status]}</Badge>
                  </TableCell>
                  <TableCell className="text-muted-foreground text-xs">{formatDate(r.created_at)}</TableCell>
                  <TableCell className="text-right">
                    <div className="flex justify-end gap-2">
                      {r.status === "active" && (
                        <Button
                          size="sm"
                          variant="outline"
                          disabled={actionLoading[r.id]}
                          onClick={() => handlePatch(r.id, { status: "paused" })}
                        >
                          暂停
                        </Button>
                      )}
                      {r.status === "paused" && (
                        <Button
                          size="sm"
                          variant="outline"
                          disabled={actionLoading[r.id]}
                          onClick={() => handlePatch(r.id, { status: "active" })}
                        >
                          继续
                        </Button>
                      )}
                      {r.status !== "completed" && (
                        <Button
                          size="sm"
                          variant="secondary"
                          disabled={actionLoading[r.id]}
                          onClick={() => handlePatch(r.id, { status: "completed" })}
                        >
                          完成
                        </Button>
                      )}
                    </div>
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
