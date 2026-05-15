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
import { getDMARCInfo, type DMARCInfo } from "@/lib/api"

export default function DmarcInfoClient() {
  const [query, setQuery] = useState("")
  const [loading, setLoading] = useState(false)
  const [result, setResult] = useState<DMARCInfo | null>(null)
  const [error, setError] = useState("")

  const handleSubmit = async () => {
    const q = query.trim()
    if (!q || loading) return
    try {
      setLoading(true)
      setError("")
      setResult(null)
      const data = await getDMARCInfo(q)
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
        <h1 className="text-3xl font-bold">DMARC 记录查询</h1>
        <p className="text-muted-foreground mt-2">
          查询域名的 DMARC 策略记录，检查邮件认证配置
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>查询配置</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="dmarc-query">域名</Label>
            <div className="flex gap-2">
              <Input
                id="dmarc-query"
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
            <div className="flex items-center justify-between">
              <CardTitle>DMARC 记录</CardTitle>
              <Badge variant={result.found ? "default" : "secondary"}>
                {result.found ? "Found" : "Not Found"}
              </Badge>
            </div>
          </CardHeader>
          <CardContent className="space-y-3 text-sm">
            <div className="flex gap-2">
              <span className="text-muted-foreground w-16 shrink-0 font-medium">域名</span>
              <span className="font-mono break-all">{result.domain}</span>
            </div>
            {result.found && result.record && (
              <div className="space-y-1">
                <span className="text-muted-foreground font-medium">记录内容</span>
                <pre className="bg-muted rounded p-3 text-xs font-mono break-all whitespace-pre-wrap overflow-x-auto">
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
          <p>• DMARC 记录位于 _dmarc.example.com TXT 记录</p>
          <p>• 配合 SPF 和 DKIM 共同防止邮件欺骗</p>
        </CardContent>
      </Card>
    </div>
  )
}
