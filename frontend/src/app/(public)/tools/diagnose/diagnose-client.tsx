"use client"

import { useState, useEffect, useRef } from "react"
import { useRouter } from "next/navigation"
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  Badge,
  Button,
  Input,
  Label,
  Progress,
} from "@/components/ui"
import { Loader2, CheckCircle2, XCircle, Circle, Clock } from "lucide-react"

type CheckStatus = "pending" | "running" | "done" | "error"

interface CheckItem {
  key: string
  label: string
  description: string
  status: CheckStatus
  summary?: string
  error?: string
}

const INITIAL_CHECKS: CheckItem[] = [
  { key: "dns", label: "DNS 解析", description: "A 记录查询", status: "pending" },
  { key: "http", label: "HTTP 可达性", description: "HTTPS 状态检测", status: "pending" },
  { key: "ping", label: "Ping 延迟", description: "ICMP 往返时延", status: "pending" },
  { key: "traceroute", label: "路由追踪", description: "Traceroute 路径", status: "pending" },
  { key: "ssl", label: "SSL 证书", description: "证书有效性", status: "pending" },
  { key: "icp", label: "ICP 备案", description: "工信部备案查询", status: "pending" },
  { key: "whois", label: "WHOIS", description: "域名注册信息", status: "pending" },
]

export default function DiagnoseClient() {
  const router = useRouter()
  const [domain, setDomain] = useState("")
  const [running, setRunning] = useState(false)
  const [checks, setChecks] = useState<CheckItem[]>(INITIAL_CHECKS)
  const esRef = useRef<EventSource | null>(null)

  useEffect(() => {
    return () => {
      esRef.current?.close()
    }
  }, [])

  const updateCheck = (key: string, updates: Partial<CheckItem>) => {
    setChecks(prev => prev.map(c => (c.key === key ? { ...c, ...updates } : c)))
  }

  const handleDiagnose = () => {
    const trimmed = domain.trim()
    if (!trimmed || running) return

    const cleanDomain = trimmed.replace(/^https?:\/\//, "").replace(/\/$/, "")

    setChecks(INITIAL_CHECKS)
    setRunning(true)

    esRef.current?.close()
    const es = new EventSource(`/api/diagnose/stream?domain=${encodeURIComponent(cleanDomain)}`)
    esRef.current = es

    es.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data as string)
        if (data.type === "check_start") {
          updateCheck(data.key as string, { status: "running" })
        } else if (data.type === "check_done") {
          updateCheck(data.key as string, { status: "done", summary: data.summary as string })
        } else if (data.type === "check_error") {
          updateCheck(data.key as string, { status: "error", error: data.error as string })
        } else if (data.type === "complete") {
          es.close()
          setRunning(false)
          router.push(`/r/${data.reportId as string}`)
        }
      } catch {
        // ignore parse errors
      }
    }

    es.onerror = () => {
      es.close()
      setRunning(false)
    }
  }

  const completedCount = checks.filter(
    c => c.status === "done" || c.status === "error"
  ).length
  const progressValue = (completedCount / INITIAL_CHECKS.length) * 100
  const hasStarted = checks.some(c => c.status !== "pending")

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">一键网络诊断</h1>
        <p className="text-muted-foreground mt-2">
          输入域名，通过 SSE 实时推送 DNS、HTTP、Ping、Traceroute、SSL、ICP 备案、WHOIS 七项检测进度
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

      {hasStarted && (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Clock className="h-5 w-5" />
              诊断进度
              <span className="ml-auto text-sm font-normal text-muted-foreground">
                {completedCount} / {INITIAL_CHECKS.length}
              </span>
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <Progress value={progressValue} />

            <div className="space-y-3">
              {checks.map((check) => (
                <div key={check.key} className="flex items-start gap-3">
                  <div className="mt-0.5 shrink-0">
                    {check.status === "pending" && (
                      <Circle className="h-4 w-4 text-muted-foreground" />
                    )}
                    {check.status === "running" && (
                      <Loader2 className="h-4 w-4 animate-spin text-blue-500" />
                    )}
                    {check.status === "done" && (
                      <CheckCircle2 className="h-4 w-4 text-green-500" />
                    )}
                    {check.status === "error" && (
                      <XCircle className="h-4 w-4 text-red-500" />
                    )}
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 flex-wrap">
                      <span className="font-medium text-sm">{check.label}</span>
                      <span className="text-xs text-muted-foreground">{check.description}</span>
                      <div className="ml-auto">
                        {check.status === "pending" && (
                          <Badge variant="outline" className="text-muted-foreground text-xs">
                            等待中
                          </Badge>
                        )}
                        {check.status === "running" && (
                          <Badge variant="info" className="text-xs">检测中</Badge>
                        )}
                        {check.status === "done" && (
                          <Badge variant="success" className="text-xs">完成</Badge>
                        )}
                        {check.status === "error" && (
                          <Badge variant="destructive" className="text-xs">失败</Badge>
                        )}
                      </div>
                    </div>
                    {check.summary && (
                      <p className="text-xs text-muted-foreground mt-0.5">{check.summary}</p>
                    )}
                    {check.error && (
                      <p className="text-xs text-red-500 mt-0.5">{check.error}</p>
                    )}
                  </div>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader>
          <CardTitle>使用说明</CardTitle>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground space-y-2">
          <p>• <strong>实时推送</strong>：通过 Server-Sent Events (SSE) 逐项实时推送检测进度</p>
          <p>• <strong>DNS 解析</strong>：检查域名的 A 记录解析结果</p>
          <p>• <strong>HTTP 可达性</strong>：测试网站 HTTPS 访问状态和响应时间</p>
          <p>• <strong>Ping 延迟</strong>：测量网络延迟和丢包率</p>
          <p>• <strong>路由追踪</strong>：追踪到目标服务器的路由路径</p>
          <p>• <strong>SSL 证书</strong>：检查 HTTPS 证书有效性和到期时间</p>
          <p>• <strong>ICP 备案</strong>：查询工信部 ICP 备案信息</p>
          <p>• <strong>WHOIS</strong>：查询域名注册信息和到期日期</p>
        </CardContent>
      </Card>
    </div>
  )
}
