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
import { getSSLInfo, type SSLInfo } from "@/lib/api"

export default function SslInfoClient() {
  const [query, setQuery] = useState("")
  const [loading, setLoading] = useState(false)
  const [result, setResult] = useState<SSLInfo | null>(null)
  const [error, setError] = useState("")

  const handleSubmit = async () => {
    const q = query.trim()
    if (!q || loading) return
    try {
      setLoading(true)
      setError("")
      setResult(null)
      const data = await getSSLInfo(q)
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
        <h1 className="text-3xl font-bold">SSL 证书检测</h1>
        <p className="text-muted-foreground mt-2">
          检查域名的 SSL 证书有效性、颁发机构、到期日期和 SAN 域名列表
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>检测配置</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="ssl-query">域名</Label>
            <div className="flex gap-2">
              <Input
                id="ssl-query"
                placeholder="example.com"
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
                {loading ? "检测中..." : "检测"}
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
              <CardTitle>证书信息</CardTitle>
              <Badge variant={result.days_remaining > 30 ? "default" : "destructive"}>
                {result.days_remaining > 0
                  ? `${result.days_remaining} 天后到期`
                  : "已过期"}
              </Badge>
            </div>
          </CardHeader>
          <CardContent className="space-y-3 text-sm">
            {(
              [
                ["域名", result.domain],
                ["颁发机构", result.issuer],
                ["有效期起", result.valid_from],
                ["有效期至", result.valid_to],
                ["证书状态", result.is_valid ? "有效" : "无效"],
              ] as [string, string][]
            ).map(([k, v]) => (
              <div key={k} className="flex gap-2">
                <span className="text-muted-foreground w-24 shrink-0 font-medium">{k}</span>
                <span className="font-mono break-all">{v ?? "-"}</span>
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
            • <strong>域名</strong>：输入不含 https:// 的裸域名（如 example.com）
          </p>
          <p>• 检测 HTTPS 证书有效性和到期时间</p>
          <p>• 显示证书颁发机构和有效期信息</p>
        </CardContent>
      </Card>
    </div>
  )
}
