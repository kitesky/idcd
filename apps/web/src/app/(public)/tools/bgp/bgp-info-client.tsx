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
import { getBGPInfo, type BGPInfo } from "@/lib/api"

export default function BgpInfoClient() {
  const [query, setQuery] = useState("")
  const [loading, setLoading] = useState(false)
  const [result, setResult] = useState<BGPInfo | null>(null)
  const [error, setError] = useState("")

  const handleSubmit = async () => {
    const q = query.trim()
    if (!q || loading) return
    try {
      setLoading(true)
      setError("")
      setResult(null)
      const data = await getBGPInfo(q)
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
        <h1 className="text-3xl font-bold">BGP 路由查询</h1>
        <p className="text-muted-foreground mt-2">
          查询 IP 地址的 BGP 路由信息，包括所属前缀和 AS 路径
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>查询配置</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="bgp-query">IP 地址</Label>
            <div className="flex gap-2">
              <Input
                id="bgp-query"
                placeholder="1.1.1.1"
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
            <CardTitle>BGP 路由信息</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4 text-sm">
            <div className="flex gap-2">
              <span className="text-muted-foreground w-16 shrink-0 font-medium">IP</span>
              <span className="font-mono break-all">{result.ip}</span>
            </div>

            {result.asns && result.asns.length > 0 && (
              <div className="flex gap-2 flex-wrap items-start">
                <span className="text-muted-foreground w-16 shrink-0 font-medium">ASN</span>
                <div className="flex gap-1 flex-wrap">
                  {result.asns.map((asn) => (
                    <Badge key={asn} variant="secondary" className="font-mono text-xs">
                      {asn}
                    </Badge>
                  ))}
                </div>
              </div>
            )}

            {result.prefixes && result.prefixes.length > 0 && (
              <div className="flex gap-2 items-start">
                <span className="text-muted-foreground w-16 shrink-0 font-medium">前缀</span>
                <div className="space-y-1">
                  {result.prefixes.map((prefix) => (
                    <div key={prefix} className="font-mono text-xs">{prefix}</div>
                  ))}
                </div>
              </div>
            )}
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader>
          <CardTitle>使用说明</CardTitle>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground space-y-2">
          <p>• <strong>IP 地址</strong>：输入 IPv4 或 IPv6 地址</p>
          <p>• 显示该 IP 所属的 BGP 路由前缀（CIDR）</p>
          <p>• 显示路径涉及的 AS 号列表</p>
        </CardContent>
      </Card>
    </div>
  )
}
