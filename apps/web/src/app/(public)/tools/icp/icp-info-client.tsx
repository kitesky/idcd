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
import { getICPInfo, type ICPInfo } from "@/lib/api"

export default function IcpInfoClient() {
  const [query, setQuery] = useState("")
  const [loading, setLoading] = useState(false)
  const [result, setResult] = useState<ICPInfo | null>(null)
  const [error, setError] = useState("")

  const handleSubmit = async () => {
    const q = query.trim()
    if (!q || loading) return
    try {
      setLoading(true)
      setError("")
      setResult(null)
      const data = await getICPInfo(q)
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
        <h1 className="text-3xl font-bold">ICP 备案查询</h1>
        <p className="text-muted-foreground mt-2">
          查询域名的 ICP 备案号、主办单位、备案类型和备案时间
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>查询配置</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="icp-query">域名</Label>
            <div className="flex gap-2">
              <Input
                id="icp-query"
                placeholder="baidu.com"
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
              <CardTitle>ICP 备案信息</CardTitle>
              {result.icp_number ? (
                <Badge variant="default">{result.icp_number}</Badge>
              ) : (
                <Badge variant="secondary">未备案</Badge>
              )}
            </div>
          </CardHeader>
          <CardContent className="space-y-3 text-sm">
            {(
              [
                ["域名", result.domain],
                ["备案号", result.icp_number],
                ["主办单位", result.company],
                ["备案类型", result.type],
                ["备案时间", result.filed_at],
              ] as [string, string][]
            ).map(([k, v]) => (
              <div key={k} className="flex gap-2">
                <span className="text-muted-foreground w-24 shrink-0 font-medium">{k}</span>
                <span className="font-mono break-all">{v || "-"}</span>
              </div>
            ))}
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
          <p>• <strong>域名</strong>：输入不含 https:// 的裸域名（如 baidu.com）</p>
          <p>• 查询工业和信息化部 ICP/IP 地址/域名信息备案管理系统数据</p>
          <p>• 在中国大陆提供互联网信息服务的网站须完成 ICP 备案</p>
        </CardContent>
      </Card>
    </div>
  )
}
