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
import { getASNInfo, type ASNInfo } from "@/lib/api"

export default function AsnInfoClient() {
  const [query, setQuery] = useState("")
  const [loading, setLoading] = useState(false)
  const [result, setResult] = useState<ASNInfo | null>(null)
  const [error, setError] = useState("")

  const handleSubmit = async () => {
    const q = query.trim()
    if (!q || loading) return
    try {
      setLoading(true)
      setError("")
      setResult(null)
      const data = await getASNInfo(q)
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
        <h1 className="text-3xl font-bold">ASN 查询</h1>
        <p className="text-muted-foreground mt-2">
          查询 IP 地址或 AS 号对应的自治系统、ISP 和国家/地区信息
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>查询配置</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="asn-query">IP 地址或 AS 号</Label>
            <div className="flex gap-2">
              <Input
                id="asn-query"
                placeholder="1.1.1.1 或 AS13335"
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
            <CardTitle>ASN 信息</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3 text-sm">
            {(
              [
                ["查询值", result.query],
                ["ASN", result.asn],
                ["ISP", result.isp],
                ["国家/地区", result.country],
                ["国家代码", result.country_code],
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
          <p>• <strong>IP 地址</strong>：输入 IPv4（1.1.1.1）或 IPv6 地址</p>
          <p>• <strong>AS 号</strong>：输入格式为 AS13335 或 13335</p>
          <p>• 显示自治系统归属 ISP 和地理位置信息</p>
        </CardContent>
      </Card>
    </div>
  )
}
