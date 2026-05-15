"use client"

import { useState } from "react"
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  Badge,
  Button,
  Input,
  Label,
} from "@/components/ui"
import { apiRequest } from "@/lib/api"

interface IPInfo {
  ip: string
  country: string
  city: string
  asn: string
  isp: string
  is_datacenter: boolean
  is_proxy: boolean
}

async function getIPInfo(q: string): Promise<IPInfo> {
  return apiRequest<IPInfo>(`/v1/info/ip?q=${encodeURIComponent(q)}`)
}

export default function IpInfoClient() {
  const [query, setQuery] = useState("")
  const [loading, setLoading] = useState(false)
  const [result, setResult] = useState<IPInfo | null>(null)
  const [error, setError] = useState("")

  const handleSubmit = async () => {
    const q = query.trim()
    if (!q || loading) return
    try {
      setLoading(true)
      setError("")
      setResult(null)
      const data = await getIPInfo(q)
      setResult(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : "查询失败")
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">IP 地址查询</h1>
        <p className="text-muted-foreground mt-2">
          查询 IP 地址的地理位置、归属 ASN 和 ISP 信息
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>查询配置</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="ip-query">IP 地址或域名</Label>
            <div className="flex gap-2">
              <Input
                id="ip-query"
                placeholder="1.1.1.1 或 example.com"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && !loading && handleSubmit()}
                disabled={loading}
              />
              <Button
                onClick={handleSubmit}
                disabled={!query.trim() || loading}
                className="min-w-[100px]"
              >
                {loading ? "查询中..." : "查询"}
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>

      {error && (
        <Card>
          <CardContent className="pt-6">
            <Badge variant="destructive">错误：{error}</Badge>
          </CardContent>
        </Card>
      )}

      {result && (
        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle>查询结果</CardTitle>
              <div className="flex gap-2">
                {result.is_datacenter && <Badge variant="secondary">数据中心</Badge>}
                {result.is_proxy && <Badge variant="destructive">代理/VPN</Badge>}
              </div>
            </div>
          </CardHeader>
          <CardContent className="space-y-3 text-sm">
            {(
              [
                ["IP", result.ip],
                ["国家/地区", result.country],
                ["城市", result.city],
                ["ASN", result.asn],
                ["ISP", result.isp],
              ] as [string, string][]
            ).map(([k, v]) => (
              <div key={k} className="flex gap-2">
                <span className="text-muted-foreground w-24 shrink-0 font-medium">{k}</span>
                <span className="font-mono break-all">{v || "-"}</span>
              </div>
            ))}
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader>
          <CardTitle>使用说明</CardTitle>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground space-y-2">
          <p>
            • <strong>支持格式</strong>：IPv4（1.1.1.1）、IPv6（2606::1）、域名（example.com）
          </p>
          <p>• 显示地理位置、归属 ASN 和运营商信息</p>
          <p>• 标识数据中心 IP 和代理/VPN 出口</p>
        </CardContent>
      </Card>
    </div>
  )
}
