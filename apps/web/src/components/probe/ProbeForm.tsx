"use client"

import { useState } from "react"
import { Card, CardContent, CardHeader, CardTitle, Input, Button, Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui"

export type ProbeType = "http" | "ping" | "tcp" | "dns" | "traceroute"

interface ProbeFormProps {
  type: ProbeType
  onSubmit: (target: string, params: Record<string, any>) => void
  loading?: boolean
}

export default function ProbeForm({ type, onSubmit, loading = false }: ProbeFormProps) {
  const [target, setTarget] = useState("")
  const [method, setMethod] = useState("GET")
  const [followRedirect, setFollowRedirect] = useState(true)
  const [count, setCount] = useState("4")
  const [recordType, setRecordType] = useState("A")

  const getPlaceholder = () => {
    switch (type) {
      case "http":
        return "https://example.com"
      case "ping":
        return "example.com 或 1.1.1.1"
      case "tcp":
        return "example.com:443 或 1.1.1.1:80"
      case "dns":
        return "example.com"
      case "traceroute":
        return "example.com 或 1.1.1.1"
      default:
        return "请输入目标地址"
    }
  }

  const validate = (): boolean => {
    if (!target.trim()) {
      return false
    }

    // Basic validation
    if (type === "http" && !target.match(/^https?:\/\/.+/)) {
      return false
    }

    if (type === "tcp" && !target.includes(":")) {
      return false
    }

    return true
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!validate() || loading) return

    const params: Record<string, any> = {}

    if (type === "http") {
      params.method = method
      params.follow_redirect = followRedirect
    }

    if (type === "ping") {
      params.count = parseInt(count, 10)
    }

    if (type === "dns") {
      params.record_type = recordType
    }

    onSubmit(target, params)
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>拨测参数</CardTitle>
      </CardHeader>
      <CardContent>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <label className="text-sm font-medium">目标地址</label>
            <Input
              type="text"
              placeholder={getPlaceholder()}
              value={target}
              onChange={(e) => setTarget(e.target.value)}
              disabled={loading}
            />
          </div>

          {type === "http" && (
            <>
              <div className="space-y-2">
                <label className="text-sm font-medium">请求方法</label>
                <Select value={method} onValueChange={setMethod} disabled={loading}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="GET">GET</SelectItem>
                    <SelectItem value="POST">POST</SelectItem>
                    <SelectItem value="HEAD">HEAD</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="flex items-center gap-2">
                <input
                  type="checkbox"
                  id="followRedirect"
                  checked={followRedirect}
                  onChange={(e) => setFollowRedirect(e.target.checked)}
                  disabled={loading}
                  className="w-4 h-4"
                />
                <label htmlFor="followRedirect" className="text-sm cursor-pointer">
                  跟随重定向
                </label>
              </div>
            </>
          )}

          {type === "ping" && (
            <div className="space-y-2">
              <label className="text-sm font-medium">发送次数</label>
              <Select value={count} onValueChange={setCount} disabled={loading}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="4">4 次</SelectItem>
                  <SelectItem value="10">10 次</SelectItem>
                  <SelectItem value="20">20 次</SelectItem>
                </SelectContent>
              </Select>
            </div>
          )}

          {type === "dns" && (
            <div className="space-y-2">
              <label className="text-sm font-medium">记录类型</label>
              <Select value={recordType} onValueChange={setRecordType} disabled={loading}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="A">A (IPv4)</SelectItem>
                  <SelectItem value="AAAA">AAAA (IPv6)</SelectItem>
                  <SelectItem value="MX">MX (邮件)</SelectItem>
                  <SelectItem value="TXT">TXT (文本)</SelectItem>
                  <SelectItem value="CNAME">CNAME (别名)</SelectItem>
                  <SelectItem value="NS">NS (域名服务器)</SelectItem>
                </SelectContent>
              </Select>
            </div>
          )}

          <Button type="submit" disabled={loading || !validate()} className="w-full">
            {loading ? "拨测中..." : "开始拨测"}
          </Button>
        </form>
      </CardContent>
    </Card>
  )
}
