"use client"

import { useState } from "react"
import { Card, CardContent, CardHeader, CardTitle, Button, Input, Label } from "@idcd/ui"
import { Loader2, CheckCircle2, XCircle, Circle, Clock } from "lucide-react"
import {
  probeDns,
  probeHttp,
  probePing,
  probeTraceroute,
  getSSLInfo,
  getWhoisInfo,
  type ProbeResult,
  type SSLInfo,
  type WhoisInfo
} from "@/lib/api"

type CheckStatus = "pending" | "running" | "done" | "error"

interface CheckItem {
  name: string
  status: CheckStatus
  result?: any
  error?: string
}

export default function DiagnoseClient() {
  const [domain, setDomain] = useState("")
  const [running, setRunning] = useState(false)
  const [checks, setChecks] = useState<CheckItem[]>([
    { name: "DNS 解析", status: "pending" },
    { name: "HTTPS 可达性", status: "pending" },
    { name: "Ping 延迟", status: "pending" },
    { name: "Traceroute", status: "pending" },
    { name: "SSL 证书", status: "pending" },
    { name: "WHOIS 信息", status: "pending" }
  ])

  const updateCheckStatus = (index: number, updates: Partial<CheckItem>) => {
    setChecks(prev => prev.map((check, i) =>
      i === index ? { ...check, ...updates } : check
    ))
  }

  const handleDiagnose = async () => {
    if (!domain.trim()) return

    setRunning(true)

    // 重置所有检查项为 running
    setChecks([
      { name: "DNS 解析", status: "running" },
      { name: "HTTPS 可达性", status: "running" },
      { name: "Ping 延迟", status: "running" },
      { name: "Traceroute", status: "running" },
      { name: "SSL 证书", status: "running" },
      { name: "WHOIS 信息", status: "running" }
    ])

    const cleanDomain = domain.trim().replace(/^https?:\/\//, '').replace(/\/$/, '')

    // 并发执行所有检查
    const checkFunctions = [
      // DNS 解析
      async () => {
        try {
          const result = await probeDns({ target: cleanDomain, type: "A" })
          updateCheckStatus(0, { status: "done", result })
        } catch (err) {
          updateCheckStatus(0, {
            status: "error",
            error: err instanceof Error ? err.message : "检测失败"
          })
        }
      },
      // HTTPS 可达性
      async () => {
        try {
          const result = await probeHttp({ target: `https://${cleanDomain}` })
          updateCheckStatus(1, { status: "done", result })
        } catch (err) {
          updateCheckStatus(1, {
            status: "error",
            error: err instanceof Error ? err.message : "检测失败"
          })
        }
      },
      // Ping 延迟
      async () => {
        try {
          const result = await probePing({ target: cleanDomain })
          updateCheckStatus(2, { status: "done", result })
        } catch (err) {
          updateCheckStatus(2, {
            status: "error",
            error: err instanceof Error ? err.message : "检测失败"
          })
        }
      },
      // Traceroute
      async () => {
        try {
          const result = await probeTraceroute({ target: cleanDomain })
          updateCheckStatus(3, { status: "done", result })
        } catch (err) {
          updateCheckStatus(3, {
            status: "error",
            error: err instanceof Error ? err.message : "检测失败"
          })
        }
      },
      // SSL 证书
      async () => {
        try {
          const result = await getSSLInfo(cleanDomain)
          updateCheckStatus(4, { status: "done", result })
        } catch (err) {
          updateCheckStatus(4, {
            status: "error",
            error: err instanceof Error ? err.message : "检测失败"
          })
        }
      },
      // WHOIS 信息
      async () => {
        try {
          const result = await getWhoisInfo(cleanDomain)
          updateCheckStatus(5, { status: "done", result })
        } catch (err) {
          updateCheckStatus(5, {
            status: "error",
            error: err instanceof Error ? err.message : "检测失败"
          })
        }
      }
    ]

    await Promise.allSettled(checkFunctions.map(fn => fn()))
    setRunning(false)
  }

  const getStatusIcon = (status: CheckStatus) => {
    switch (status) {
      case "pending":
        return <Circle className="h-5 w-5 text-muted-foreground" />
      case "running":
        return <Loader2 className="h-5 w-5 animate-spin text-blue-500" />
      case "done":
        return <CheckCircle2 className="h-5 w-5 text-green-500" />
      case "error":
        return <XCircle className="h-5 w-5 text-red-500" />
    }
  }

  const renderCheckSummary = (check: CheckItem) => {
    if (check.status !== "done" || !check.result) return null

    switch (check.name) {
      case "DNS 解析": {
        const result = check.result as ProbeResult
        const ips = result.results
          ?.filter(r => r.success && r.details?.answers)
          ?.flatMap(r => r.details.answers.map((a: any) => a.data))
          ?.filter((v, i, a) => a.indexOf(v) === i) // 去重
        return ips && ips.length > 0 ? (
          <div className="text-sm text-muted-foreground ml-10 mt-1">
            解析到: {ips.join(", ")}
          </div>
        ) : null
      }
      case "HTTPS 可达性": {
        const result = check.result as ProbeResult
        const avgLatency = result.results
          ?.filter(r => r.success && r.latency_ms)
          ?.reduce((sum, r) => sum + (r.latency_ms || 0), 0)
        const count = result.results?.filter(r => r.success).length || 0
        const statusCode = result.results?.[0]?.details?.status_code
        return (
          <div className="text-sm text-muted-foreground ml-10 mt-1">
            {statusCode && `状态码: ${statusCode}`}
            {count > 0 && ` | 平均响应时间: ${(avgLatency! / count).toFixed(0)}ms`}
          </div>
        )
      }
      case "Ping 延迟": {
        const result = check.result as ProbeResult
        const avgLatency = result.results
          ?.filter(r => r.success && r.latency_ms)
          ?.reduce((sum, r) => sum + (r.latency_ms || 0), 0)
        const count = result.results?.filter(r => r.success).length || 0
        const lossRate = result.results
          ? ((result.results.length - count) / result.results.length * 100).toFixed(1)
          : 0
        return count > 0 ? (
          <div className="text-sm text-muted-foreground ml-10 mt-1">
            平均 RTT: {(avgLatency! / count).toFixed(0)}ms | 丢包率: {lossRate}%
          </div>
        ) : null
      }
      case "SSL 证书": {
        const result = check.result as SSLInfo
        return (
          <div className="text-sm text-muted-foreground ml-10 mt-1">
            颁发机构: {result.issuer} | 剩余 {result.days_remaining} 天到期
          </div>
        )
      }
      case "WHOIS 信息": {
        const result = check.result as WhoisInfo
        return (
          <div className="text-sm text-muted-foreground ml-10 mt-1">
            注册商: {result.registrar}
            {result.expiration_date && ` | 到期日: ${new Date(result.expiration_date).toLocaleDateString()}`}
          </div>
        )
      }
      case "Traceroute": {
        const result = check.result as ProbeResult
        const hops = result.results?.[0]?.details?.hops?.length
        return hops ? (
          <div className="text-sm text-muted-foreground ml-10 mt-1">
            跳数: {hops}
          </div>
        ) : null
      }
      default:
        return null
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">一键网络诊断</h1>
        <p className="text-muted-foreground mt-2">
          输入域名，自动进行 DNS、HTTPS、Ping、Traceroute、SSL、WHOIS 六项全面检测
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>诊断配置</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="domain">目标域名</Label>
            <div className="flex gap-2">
              <Input
                id="domain"
                placeholder="example.com"
                value={domain}
                onChange={(e) => setDomain(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && !running && handleDiagnose()}
                disabled={running}
              />
              <Button
                onClick={handleDiagnose}
                disabled={!domain.trim() || running}
                className="min-w-[120px]"
              >
                {running ? (
                  <>
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    诊断中
                  </>
                ) : (
                  "开始诊断"
                )}
              </Button>
            </div>
            <p className="text-xs text-muted-foreground">
              支持输入域名（如 example.com）或完整 URL
            </p>
          </div>
        </CardContent>
      </Card>

      {checks.some(c => c.status !== "pending") && (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Clock className="h-5 w-5" />
              诊断进度
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            {checks.map((check, index) => (
              <div key={index} className="space-y-1">
                <div className="flex items-center gap-3">
                  {getStatusIcon(check.status)}
                  <span className={
                    check.status === "error"
                      ? "text-red-500"
                      : check.status === "done"
                      ? "text-foreground"
                      : "text-muted-foreground"
                  }>
                    {check.name}
                  </span>
                </div>
                {check.status === "error" && check.error && (
                  <div className="text-sm text-red-500 ml-10">
                    错误: {check.error}
                  </div>
                )}
                {renderCheckSummary(check)}
              </div>
            ))}
          </CardContent>
        </Card>
      )}

      {checks.every(c => c.status === "done" || c.status === "error") &&
       checks.some(c => c.status !== "pending") && (
        <Card>
          <CardHeader>
            <CardTitle>诊断完成</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-sm text-muted-foreground mb-4">
              所有检测项已完成。{checks.filter(c => c.status === "done").length} 项成功，
              {checks.filter(c => c.status === "error").length} 项失败。
            </p>
            <div className="flex gap-2">
              <Button variant="outline" onClick={() => {
                setChecks([
                  { name: "DNS 解析", status: "pending" },
                  { name: "HTTPS 可达性", status: "pending" },
                  { name: "Ping 延迟", status: "pending" },
                  { name: "Traceroute", status: "pending" },
                  { name: "SSL 证书", status: "pending" },
                  { name: "WHOIS 信息", status: "pending" }
                ])
              }}>
                重新诊断
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader>
          <CardTitle>使用说明</CardTitle>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground space-y-2">
          <p>• <strong>一键诊断</strong>：自动从全球多个节点并发执行 6 项检测</p>
          <p>• <strong>实时反馈</strong>：每项检测完成后立即显示结果摘要</p>
          <p>• <strong>DNS 解析</strong>：检查域名的 A 记录解析结果</p>
          <p>• <strong>HTTPS 可达性</strong>：测试网站的 HTTPS 访问状态和响应时间</p>
          <p>• <strong>Ping 延迟</strong>：测量网络延迟和丢包率</p>
          <p>• <strong>Traceroute</strong>：追踪到目标服务器的路由路径</p>
          <p>• <strong>SSL 证书</strong>：检查 HTTPS 证书的有效性和到期时间</p>
          <p>• <strong>WHOIS 信息</strong>：查询域名注册信息和到期日期</p>
        </CardContent>
      </Card>
    </div>
  )
}
