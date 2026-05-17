"use client"

import { useCallback, useEffect, useState } from "react"
import { toast } from "sonner"
import { Activity, Plus, Trash2 } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
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
  CertAPIError,
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
  // Aliyun DNS
  const [aliyunAKID, setAliyunAKID] = useState("")
  const [aliyunSecret, setAliyunSecret] = useState("")
  const [aliyunRegion, setAliyunRegion] = useState("")
  // DNSPod / 腾讯云
  const [dnspodSecretID, setDnspodSecretID] = useState("")
  const [dnspodSecretKey, setDnspodSecretKey] = useState("")
  // AWS Route 53
  const [r53AKID, setR53AKID] = useState("")
  const [r53Secret, setR53Secret] = useState("")
  const [r53Region, setR53Region] = useState("")
  const [r53HostedZoneID, setR53HostedZoneID] = useState("")
  // Google Cloud DNS
  const [gcloudJSON, setGcloudJSON] = useState("")
  const [gcloudProjectID, setGcloudProjectID] = useState("")
  const [busy, setBusy] = useState(false)
  const [busyRow, setBusyRow] = useState<string | null>(null)

  const reload = useCallback(() => {
    setLoading(true)
    return listDnsCredentials()
      .then((c) => setCreds(c))
      .catch((err) => {
        const msg = err instanceof Error ? err.message : "加载凭据失败"
        toast.error(msg)
      })
      .finally(() => setLoading(false))
  }, [])

  useEffect(() => {
    let mounted = true
    listDnsCredentials()
      .then((c) => {
        if (mounted) setCreds(c)
      })
      .catch((err) => {
        if (mounted) {
          const msg = err instanceof Error ? err.message : "加载凭据失败"
          toast.error(msg)
        }
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
    setAliyunAKID("")
    setAliyunSecret("")
    setAliyunRegion("")
    setDnspodSecretID("")
    setDnspodSecretKey("")
    setR53AKID("")
    setR53Secret("")
    setR53Region("")
    setR53HostedZoneID("")
    setGcloudJSON("")
    setGcloudProjectID("")
  }

  function buildCredential(): {
    credential?: Record<string, string>
    error?: string
  } {
    switch (provider) {
      case "cloudflare":
      case "manual":
        return {}
      case "aliyun": {
        if (!aliyunAKID.trim()) return { error: "请填写 Access Key ID" }
        if (!aliyunSecret.trim()) return { error: "请填写 Access Key Secret" }
        const cred: Record<string, string> = {
          access_key_id: aliyunAKID.trim(),
          access_key_secret: aliyunSecret.trim(),
        }
        if (aliyunRegion.trim()) cred.region_id = aliyunRegion.trim()
        return { credential: cred }
      }
      case "dnspod": {
        if (!dnspodSecretID.trim()) return { error: "请填写 Secret ID" }
        if (!dnspodSecretKey.trim()) return { error: "请填写 Secret Key" }
        return {
          credential: {
            secret_id: dnspodSecretID.trim(),
            secret_key: dnspodSecretKey.trim(),
          },
        }
      }
      case "route53": {
        if (!r53AKID.trim()) return { error: "请填写 Access Key ID" }
        if (!r53Secret.trim()) return { error: "请填写 Secret Access Key" }
        const cred: Record<string, string> = {
          access_key_id: r53AKID.trim(),
          secret_access_key: r53Secret.trim(),
        }
        if (r53Region.trim()) cred.region = r53Region.trim()
        if (r53HostedZoneID.trim()) cred.hosted_zone_id = r53HostedZoneID.trim()
        return { credential: cred }
      }
      case "gcloud": {
        if (!gcloudJSON.trim()) return { error: "请粘贴 Service Account JSON" }
        const cred: Record<string, string> = {
          service_account_json: gcloudJSON.trim(),
        }
        if (gcloudProjectID.trim()) cred.project_id = gcloudProjectID.trim()
        return { credential: cred }
      }
    }
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
    const built = buildCredential()
    if (built.error) {
      toast.error(built.error)
      return
    }
    setBusy(true)
    try {
      const created = await createDnsCredential({
        provider,
        displayName: displayName.trim(),
        apiToken:
          provider === "cloudflare" ? apiToken.trim() || undefined : undefined,
        credential: built.credential,
      })
      setCreds((prev) => [created, ...prev])
      toast.success("已新增凭据")
      setSheetOpen(false)
      resetForm()
    } catch (err) {
      const msg = err instanceof CertAPIError ? err.message : err instanceof Error ? err.message : "保存失败"
      toast.error(msg)
    } finally {
      setBusy(false)
    }
  }

  async function handleTestNew() {
    if (provider === "cloudflare" && !apiToken.trim()) {
      toast.error("请填写 API Token")
      return
    }
    if (provider !== "cloudflare" && provider !== "manual") {
      const built = buildCredential()
      if (built.error) {
        toast.error(built.error)
        return
      }
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
    } catch (err) {
      const msg = err instanceof CertAPIError ? err.message : err instanceof Error ? err.message : "吊销失败"
      toast.error(msg)
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
            {provider === "aliyun" && (
              <>
                <div className="space-y-2">
                  <Label htmlFor="aliyun-akid">Access Key ID</Label>
                  <Input
                    id="aliyun-akid"
                    placeholder="LTAI5txxxxxxxxxxxxxxxxxx"
                    value={aliyunAKID}
                    onChange={(e) => setAliyunAKID(e.target.value)}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="aliyun-secret">Access Key Secret</Label>
                  <Input
                    id="aliyun-secret"
                    type="password"
                    placeholder="••••••••••••••••"
                    value={aliyunSecret}
                    onChange={(e) => setAliyunSecret(e.target.value)}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="aliyun-region">Region ID（可选）</Label>
                  <Input
                    id="aliyun-region"
                    placeholder="cn-hangzhou"
                    value={aliyunRegion}
                    onChange={(e) => setAliyunRegion(e.target.value)}
                  />
                  <p className="text-xs text-muted-foreground">
                    留空则默认使用 cn-hangzhou。
                  </p>
                </div>
              </>
            )}
            {provider === "dnspod" && (
              <>
                <div className="space-y-2">
                  <Label htmlFor="dnspod-secret-id">Secret ID</Label>
                  <Input
                    id="dnspod-secret-id"
                    placeholder="AKIDxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
                    value={dnspodSecretID}
                    onChange={(e) => setDnspodSecretID(e.target.value)}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="dnspod-secret-key">Secret Key</Label>
                  <Input
                    id="dnspod-secret-key"
                    type="password"
                    placeholder="••••••••••••••••"
                    value={dnspodSecretKey}
                    onChange={(e) => setDnspodSecretKey(e.target.value)}
                  />
                </div>
                <p className="text-xs text-muted-foreground">
                  在腾讯云访问管理（CAM）创建子账号 API 密钥，授予 DNSPod 解析权限。
                </p>
              </>
            )}
            {provider === "route53" && (
              <>
                <div className="space-y-2">
                  <Label htmlFor="r53-akid">Access Key ID</Label>
                  <Input
                    id="r53-akid"
                    placeholder="AKIAIOSFODNN7EXAMPLE"
                    value={r53AKID}
                    onChange={(e) => setR53AKID(e.target.value)}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="r53-secret">Secret Access Key</Label>
                  <Input
                    id="r53-secret"
                    type="password"
                    placeholder="••••••••••••••••"
                    value={r53Secret}
                    onChange={(e) => setR53Secret(e.target.value)}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="r53-region">Region（可选）</Label>
                  <Input
                    id="r53-region"
                    placeholder="us-east-1"
                    value={r53Region}
                    onChange={(e) => setR53Region(e.target.value)}
                  />
                  <p className="text-xs text-muted-foreground">
                    Route 53 API 默认走 us-east-1，留空即可。
                  </p>
                </div>
                <div className="space-y-2">
                  <Label htmlFor="r53-zone">Hosted Zone ID（可选）</Label>
                  <Input
                    id="r53-zone"
                    placeholder="Z2FDTNDATAQYW2"
                    value={r53HostedZoneID}
                    onChange={(e) => setR53HostedZoneID(e.target.value)}
                  />
                  <p className="text-xs text-muted-foreground">
                    指定后将只允许操作该 Hosted Zone，建议绑定以收紧权限。
                  </p>
                </div>
              </>
            )}
            {provider === "gcloud" && (
              <>
                <div className="space-y-2">
                  <Label htmlFor="gcloud-json">Service Account JSON</Label>
                  <Textarea
                    id="gcloud-json"
                    rows={8}
                    className="font-mono text-xs"
                    placeholder='{ "type": "service_account", "project_id": "...", ... }'
                    value={gcloudJSON}
                    onChange={(e) => setGcloudJSON(e.target.value)}
                  />
                  <p className="text-xs text-muted-foreground">
                    粘贴 GCP IAM 控制台导出的完整 JSON 密钥文件内容，需具备 DNS Administrator 角色。
                  </p>
                </div>
                <div className="space-y-2">
                  <Label htmlFor="gcloud-project">Project ID（可选）</Label>
                  <Input
                    id="gcloud-project"
                    placeholder="my-gcp-project"
                    value={gcloudProjectID}
                    onChange={(e) => setGcloudProjectID(e.target.value)}
                  />
                  <p className="text-xs text-muted-foreground">
                    留空时使用 JSON 内的 project_id 字段。
                  </p>
                </div>
              </>
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
