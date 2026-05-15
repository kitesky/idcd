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
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui"
import { getMXInfo, type MXInfo } from "@/lib/api"

export default function MxInfoClient() {
  const [query, setQuery] = useState("")
  const [loading, setLoading] = useState(false)
  const [result, setResult] = useState<MXInfo | null>(null)
  const [error, setError] = useState("")

  const handleSubmit = async () => {
    const q = query.trim()
    if (!q || loading) return
    try {
      setLoading(true)
      setError("")
      setResult(null)
      const data = await getMXInfo(q)
      setResult(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : "查询失败")
    } finally {
      setLoading(false)
    }
  }

  const sortedRecords = result?.records
    ? [...result.records].sort((a, b) => a.priority - b.priority)
    : []

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">MX 记录查询</h1>
        <p className="text-muted-foreground mt-2">
          查询域名的 MX 邮件交换记录，显示邮件服务器优先级和主机名
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>查询配置</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="mx-query">域名</Label>
            <div className="flex gap-2">
              <Input
                id="mx-query"
                placeholder="gmail.com"
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
              <CardTitle>MX 记录</CardTitle>
              <Badge variant="secondary">{result.domain}</Badge>
            </div>
          </CardHeader>
          <CardContent>
            {sortedRecords.length > 0 ? (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="w-24">优先级</TableHead>
                    <TableHead>邮件服务器</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {sortedRecords.map((rec) => (
                    <TableRow key={`${rec.priority}-${rec.host}`}>
                      <TableCell className="font-mono">{rec.priority}</TableCell>
                      <TableCell className="font-mono break-all">{rec.host}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            ) : (
              <p className="text-sm text-muted-foreground">未找到 MX 记录</p>
            )}
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader>
          <CardTitle>使用说明</CardTitle>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground space-y-2">
          <p>• <strong>域名</strong>：输入不含 https:// 的裸域名（如 gmail.com）</p>
          <p>• 优先级数值越小，邮件服务器优先级越高</p>
          <p>• MX 记录决定接收邮件时连接哪个服务器</p>
        </CardContent>
      </Card>
    </div>
  )
}
