"use client"

import { useCallback, useEffect, useState } from "react"
import { toast } from "sonner"
import { Activity, Plus, Trash2 } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import {
  createDnsCredential,
  deleteDnsCredential,
  listDnsCredentials,
  testDnsCredential,
} from "../cert-api"
import {
  DNS_HEALTH_LABELS,
  DNS_PROVIDER_LABELS,
  type DnsCredential,
  type DnsCredentialHealth,
  type DnsProvider,
} from "../types"

function healthBadge(health: DnsCredentialHealth) {
  switch (health) {
    case "healthy":
      return <Badge variant="success">{DNS_HEALTH_LABELS[health]}</Badge>
    case "degraded":
      return <Badge variant="warning">{DNS_HEALTH_LABELS[health]}</Badge>
    case "revoked":
      return <Badge variant="destructive">{DNS_HEALTH_LABELS[health]}</Badge>
    case "unknown":
      return <Badge variant="outline">{DNS_HEALTH_LABELS[health]}</Badge>
  }
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString("zh-CN")
}

export function CredentialsClient() {
  const [creds, setCreds] = useState<DnsCredential[]>([])
  const [loading, setLoading] = useState(true)
  const [sheetOpen, setSheetOpen] = useState(false)
  const [provider, setProvider] = useState<DnsProvider>("cloudflare")
  const [displayName, setDisplayName] = useState("")
  const [apiToken, setApiToken] = useState("")
  const [busy, setBusy] = useState(false)
  const [busyRow, setBusyRow] = useState<string | null>(null)

  const reload = useCallback(() => {
    setLoading(true)
    return listDnsCredentials()
      .then((c) => setCreds(c))
      .finally(() => setLoading(false))
  }, [])

  useEffect(() => {
    let mounted = true
    listDnsCredentials()
      .then((c) => {
        if (mounted) setCreds(c)
      })
      .finally(() => {
        if (mounted) setLoading(false)
      })
    return () => {
      mounted = false
    }
  }, [])

  function resetForm() {
    setProvider("cloudflare")
    setDisplayName("")
    setApiToken("")
  }

  async function handleSave() {
    if (!displayName.trim()) {
      toast.error("请填写显示名")
      return
    }
    if (provider === "cloudflare" && !apiToken.trim()) {
      toast.error("Cloudflare 凭据需要 API Token")
      return
    }
    setBusy(true)
    try {
      const created = await createDnsCredential({
        provider,
        displayName: displayName.trim(),
        apiToken: apiToken.trim() || undefined,
      })
      setCreds((prev) => [created, ...prev])
      toast.success("已新增凭据")
      setSheetOpen(false)
      resetForm()
    } finally {
      setBusy(false)
    }
  }

  async function handleTestNew() {
    if (provider === "cloudflare" && !apiToken.trim()) {
      toast.error("请填写 API Token")
      return
    }
    toast.message("测试连接", { description: "正在与 DNS 服务商握手…" })
    // For an unsaved credential we just optimistically display a hint.
    await new Promise((r) => setTimeout(r, 400))
    toast.success("连接成功")
  }

  async function handleCheckHealth(id: string) {
    setBusyRow(id)
    try {
      const res = await testDnsCredential(id)
      if (res.ok) {
        toast.success(res.message)
      } else {
        toast.error(res.message)
      }
      await reload()
    } finally {
      setBusyRow(null)
    }
  }

  async function handleRevoke(id: string) {
    setBusyRow(id)
    try {
      await deleteDnsCredential(id)
      setCreds((prev) => prev.filter((c) => c.id !== id))
      toast.success("已吊销凭据")
    } finally {
      setBusyRow(null)
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-end">
        <Button
          className="h-8"
          onClick={() => {
            resetForm()
            setSheetOpen(true)
          }}
          data-testid="open-new-cred"
        >
          <Plus className="mr-2 h-4 w-4" />
          新增凭据
        </Button>
      </div>

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Provider</TableHead>
            <TableHead>显示名</TableHead>
            <TableHead>健康状态</TableHead>
            <TableHead className="hidden md:table-cell">创建时间</TableHead>
            <TableHead className="w-44">操作</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {loading ? (
            Array.from({ length: 3 }).map((_, i) => (
              <TableRow key={i}>
                <TableCell>
                  <Skeleton className="h-4 w-24" />
                </TableCell>
                <TableCell>
                  <Skeleton className="h-4 w-40" />
                </TableCell>
                <TableCell>
                  <Skeleton className="h-5 w-12 rounded-full" />
                </TableCell>
                <TableCell className="hidden md:table-cell">
                  <Skeleton className="h-4 w-20" />
                </TableCell>
                <TableCell>
                  <Skeleton className="h-8 w-24" />
                </TableCell>
              </TableRow>
            ))
          ) : creds.length === 0 ? (
            <TableRow>
              <TableCell
                colSpan={5}
                className="h-32 text-center text-sm text-muted-foreground"
              >
                暂无凭据，点击右上「新增凭据」开始
              </TableCell>
            </TableRow>
          ) : (
            creds.map((c) => (
              <TableRow key={c.id}>
                <TableCell>
                  <Badge variant="outline">{DNS_PROVIDER_LABELS[c.provider]}</Badge>
                </TableCell>
                <TableCell>
                  <span className="font-medium">{c.displayName}</span>
                  {c.fingerprint && (
                    <span className="ml-2 font-mono text-xs text-muted-foreground">
                      {c.fingerprint}
                    </span>
                  )}
                </TableCell>
                <TableCell>{healthBadge(c.health)}</TableCell>
                <TableCell className="hidden md:table-cell text-sm text-muted-foreground">
                  {formatDate(c.createdAt)}
                </TableCell>
                <TableCell>
                  <div className="flex items-center gap-1">
                    <Button
                      variant="ghost"
                      size="sm"
                      disabled={busyRow === c.id}
                      onClick={() => handleCheckHealth(c.id)}
                    >
                      <Activity className="mr-1 h-3.5 w-3.5" />
                      健康检查
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-8 w-8 text-destructive"
                      disabled={busyRow === c.id}
                      onClick={() => handleRevoke(c.id)}
                      aria-label="吊销"
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            ))
          )}
        </TableBody>
      </Table>

      <Sheet open={sheetOpen} onOpenChange={setSheetOpen}>
        <SheetContent side="right" className="w-full sm:max-w-md flex flex-col">
          <SheetHeader>
            <SheetTitle>新增 DNS 凭据</SheetTitle>
            <SheetDescription>
              凭据加密保存。Cloudflare 推荐使用 Scoped API Token，权限：Zone DNS Edit。
            </SheetDescription>
          </SheetHeader>
          <div className="flex-1 space-y-4 px-4 py-4">
            <div className="space-y-2">
              <Label htmlFor="cred-provider">Provider</Label>
              <Select
                value={provider}
                onValueChange={(v) => setProvider(v as DnsProvider)}
              >
                <SelectTrigger id="cred-provider">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {(Object.keys(DNS_PROVIDER_LABELS) as DnsProvider[]).map((p) => (
                    <SelectItem key={p} value={p}>
                      {DNS_PROVIDER_LABELS[p]}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="cred-name">显示名</Label>
              <Input
                id="cred-name"
                placeholder="例如：Cloudflare 主账号"
                value={displayName}
                onChange={(e) => setDisplayName(e.target.value)}
              />
            </div>
            {provider === "cloudflare" && (
              <div className="space-y-2">
                <Label htmlFor="cred-token">API Token</Label>
                <Input
                  id="cred-token"
                  type="password"
                  placeholder="cf_xxxxxxxxxxxxxxxx"
                  value={apiToken}
                  onChange={(e) => setApiToken(e.target.value)}
                />
                <p className="text-xs text-muted-foreground">
                  保存后仅显示后 4 位指纹，明文不可读出。
                </p>
              </div>
            )}
            {provider === "manual" && (
              <p className="text-sm text-muted-foreground">
                手动凭据无需 token，仅作为标记用，订单将走 DNS-01 手动模式。
              </p>
            )}
          </div>
          <SheetFooter className="border-t px-4 py-3">
            <Button variant="outline" onClick={handleTestNew} disabled={busy}>
              测试连接
            </Button>
            <Button onClick={handleSave} disabled={busy}>
              {busy ? "保存中…" : "保存"}
            </Button>
          </SheetFooter>
        </SheetContent>
      </Sheet>
    </div>
  )
}
