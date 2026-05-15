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
import { getWhoisInfo, type WhoisInfo } from "@/lib/api"

export default function WhoisInfoClient() {
  const [query, setQuery] = useState("")
  const [loading, setLoading] = useState(false)
  const [result, setResult] = useState<WhoisInfo | null>(null)
  const [error, setError] = useState("")

  const handleSubmit = async () => {
    const q = query.trim()
    if (!q || loading) return
    try {
      setLoading(true)
      setError("")
      setResult(null)
      const data = await getWhoisInfo(q)
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
        <h1 className="text-3xl font-bold">WHOIS 查询</h1>
        <p className="text-muted-foreground mt-2">
          查询域名的注册信息、注册商、注册日期、到期日期和名称服务器
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>查询配置</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="whois-query">域名</Label>
            <div className="flex gap-2">
              <Input
                id="whois-query"
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
            <CardTitle>WHOIS 信息</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3 text-sm">
            {(
              [
                ["域名", result.domain],
                ["注册商", result.registrar],
                ["注册日期", result.creation_date],
                ["到期日期", result.expiry_date ?? result.expiration_date],
              ] as [string, string | undefined][]
            ).map(([k, v]) => (
              <div key={k} className="flex gap-2">
                <span className="text-muted-foreground w-24 shrink-0 font-medium">{k}</span>
                <span className="font-mono break-all">{v || "-"}</span>
              </div>
            ))}

            {result.name_servers && result.name_servers.length > 0 && (
              <div className="flex gap-2 flex-wrap items-start">
                <span className="text-muted-foreground w-24 shrink-0 font-medium">名称服务器</span>
                <div className="flex gap-1 flex-wrap">
                  {result.name_servers.map((ns) => (
                    <Badge key={ns} variant="secondary" className="font-mono text-xs">
                      {ns}
                    </Badge>
                  ))}
                </div>
              </div>
            )}

            {result.note && (
              <div className="flex gap-2">
                <span className="text-muted-foreground w-24 shrink-0 font-medium">备注</span>
                <span className="text-muted-foreground break-all">{result.note}</span>
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
          <p>• <strong>域名</strong>：输入不含 https:// 的裸域名（如 example.com）</p>
          <p>• 显示注册商、注册日期、到期日期和 NS 记录</p>
          <p>• 部分域名后缀的 WHOIS 信息可能受限</p>
        </CardContent>
      </Card>
    </div>
  )
}
