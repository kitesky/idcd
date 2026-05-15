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
import { getDKIMInfo, type DKIMInfo } from "@/lib/api"

export default function DkimInfoClient() {
  const [query, setQuery] = useState("")
  const [selector, setSelector] = useState("")
  const [loading, setLoading] = useState(false)
  const [result, setResult] = useState<DKIMInfo | null>(null)
  const [error, setError] = useState("")

  const handleSubmit = async () => {
    const q = query.trim()
    if (!q || loading) return
    try {
      setLoading(true)
      setError("")
      setResult(null)
      const sel = selector.trim() || undefined
      const data = await getDKIMInfo(q, sel)
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
        <h1 className="text-3xl font-bold">DKIM 记录查询</h1>
        <p className="text-muted-foreground mt-2">
          查询域名的 DKIM（域名密钥标识邮件）签名记录
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>查询配置</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="dkim-query">域名</Label>
            <Input
              id="dkim-query"
              placeholder="example.com"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && !loading && handleSubmit()}
              disabled={loading}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="dkim-selector">Selector（可选，默认 default）</Label>
            <div className="flex gap-2">
              <Input
                id="dkim-selector"
                placeholder="default"
                value={selector}
                onChange={(e) => setSelector(e.target.value)}
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
              <CardTitle>DKIM 记录</CardTitle>
              <Badge variant={result.found ? "default" : "secondary"}>
                {result.found ? "Found" : "Not Found"}
              </Badge>
            </div>
          </CardHeader>
          <CardContent className="space-y-3 text-sm">
            {(
              [
                ["域名", result.domain],
                ["Selector", result.selector],
              ] as [string, string][]
            ).map(([k, v]) => (
              <div key={k} className="flex gap-2">
                <span className="text-muted-foreground w-20 shrink-0 font-medium">{k}</span>
                <span className="font-mono break-all">{v || "-"}</span>
              </div>
            ))}
            {result.found && result.record && (
              <div className="space-y-1">
                <span className="text-muted-foreground font-medium">记录内容</span>
                <pre className="bg-muted rounded p-3 text-xs font-mono break-all whitespace-pre-wrap overflow-x-auto max-h-48">
                  {result.record}
                </pre>
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
          <p>• <strong>Selector</strong>：DKIM 选择器，常见值有 default、google、mail 等</p>
          <p>• DKIM 记录包含公钥，用于验证邮件签名</p>
        </CardContent>
      </Card>
    </div>
  )
}
