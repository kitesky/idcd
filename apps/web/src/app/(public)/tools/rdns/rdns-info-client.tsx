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
import { getRDNSInfo, type RDNSInfo } from "@/lib/api"

export default function RdnsInfoClient() {
  const [query, setQuery] = useState("")
  const [loading, setLoading] = useState(false)
  const [result, setResult] = useState<RDNSInfo | null>(null)
  const [error, setError] = useState("")

  const handleSubmit = async () => {
    const q = query.trim()
    if (!q || loading) return
    try {
      setLoading(true)
      setError("")
      setResult(null)
      const data = await getRDNSInfo(q)
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
        <h1 className="text-3xl font-bold">反向 DNS 查询</h1>
        <p className="text-muted-foreground mt-2">
          查询 IP 地址对应的反向 DNS（PTR 记录）主机名
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>查询配置</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="rdns-query">IP 地址</Label>
            <div className="flex gap-2">
              <Input
                id="rdns-query"
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
            <CardTitle>反向 DNS 结果</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3 text-sm">
            <div className="flex gap-2">
              <span className="text-muted-foreground w-24 shrink-0 font-medium">IP</span>
              <span className="font-mono break-all">{result.ip}</span>
            </div>
            <div className="flex gap-2 items-start">
              <span className="text-muted-foreground w-24 shrink-0 font-medium">主机名</span>
              {result.hostnames && result.hostnames.length > 0 ? (
                <div className="space-y-1">
                  {result.hostnames.map((h) => (
                    <div key={h} className="font-mono break-all">{h}</div>
                  ))}
                </div>
              ) : (
                <span className="text-muted-foreground">未找到 PTR 记录</span>
              )}
            </div>
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader>
          <CardTitle>使用说明</CardTitle>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground space-y-2">
          <p>• <strong>IP 地址</strong>：输入 IPv4 或 IPv6 地址</p>
          <p>• 通过 PTR 记录反向解析 IP 对应的主机名</p>
          <p>• 部分 IP 可能没有配置 PTR 记录</p>
        </CardContent>
      </Card>
    </div>
  )
}
