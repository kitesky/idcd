"use client"

import { useState } from "react"
import { useRouter } from "next/navigation"
import {
  ArrowLeft,
  ArrowRight,
  Check,
  ChevronDown,
  ChevronRight,
  AlertCircle,
} from "lucide-react"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { type MonitorType, TYPE_LABELS, MONITOR_TYPES } from "../mock-data"

// PLAN_LABELS maps plan identifiers to user-facing display names.
const PLAN_LABELS: Record<string, string> = {
  free: "Free",
  pro: "Pro",
  team: "Team",
  business: "Business",
}

// PLAN_MONITOR_LIMITS maps plan identifiers to their monitor count limits.
const PLAN_MONITOR_LIMITS: Record<string, number> = {
  free: 3,
  pro: 50,
  team: 200,
  business: 0, // unlimited
}

const STEP_LABELS = ["类型选择", "基础配置", "高级配置", "确认创建"]

const TYPE_DESCRIPTIONS: Record<MonitorType, string> = {
  http: "检测 HTTP 接口可用性和响应状态",
  https: "检测 HTTPS 接口含证书",
  ping: "ICMP Ping 检测节点可达性",
  tcp: "TCP 端口连通性检测",
  dns: "DNS 解析正确性验证",
  ssl_expiry: "SSL 证书到期时间监控",
  domain_expiry: "域名到期时间监控",
  icp_change: "ICP 备案信息变更监控",
  keyword: "页面关键字存在性检测",
}

const TARGET_PLACEHOLDERS: Record<MonitorType, string> = {
  http: "http://example.com/api/health",
  https: "https://example.com",
  ping: "8.8.8.8 或 example.com",
  tcp: "example.com:8080",
  dns: "example.com",
  ssl_expiry: "example.com",
  domain_expiry: "example.com",
  icp_change: "example.com",
  keyword: "https://example.com",
}

interface FormState {
  type: MonitorType | ""
  name: string
  target: string
  intervalSeconds: number
  concurrentNodes: number
  // advanced
  assertStatusCode: string
  keywordMatch: string
  timeoutMs: string
  packetLossThreshold: string
  port: string
  expectedIp: string
}

const DEFAULT_FORM: FormState = {
  type: "",
  name: "",
  target: "",
  intervalSeconds: 300,
  concurrentNodes: 3,
  assertStatusCode: "200",
  keywordMatch: "",
  timeoutMs: "5000",
  packetLossThreshold: "10",
  port: "80",
  expectedIp: "",
}

function StepIndicator({
  currentStep,
  totalSteps,
}: {
  currentStep: number
  totalSteps: number
}) {
  return (
    <div className="flex items-center gap-2 mb-8">
      {STEP_LABELS.map((label, i) => (
        <div key={i} className="flex items-center gap-2">
          <div
            className={[
              "flex h-8 w-8 items-center justify-center rounded-full text-sm font-medium transition-colors",
              i < currentStep
                ? "bg-primary text-primary-foreground"
                : i === currentStep
                  ? "bg-primary text-primary-foreground ring-2 ring-primary ring-offset-2"
                  : "bg-muted text-muted-foreground",
            ].join(" ")}
          >
            {i < currentStep ? <Check className="h-4 w-4" /> : i + 1}
          </div>
          <span
            className={[
              "hidden text-sm sm:block",
              i === currentStep ? "font-medium" : "text-muted-foreground",
            ].join(" ")}
          >
            {label}
          </span>
          {i < totalSteps - 1 && (
            <ChevronRight className="h-4 w-4 text-muted-foreground" />
          )}
        </div>
      ))}
    </div>
  )
}

// QuotaExceededState holds the data needed to render a quota exceeded alert.
interface QuotaExceededState {
  currentPlan: string
  usedCount: number
  limitCount: number
}

export function NewMonitorClient() {
  const router = useRouter()
  const [step, setStep] = useState(0)
  const [form, setForm] = useState<FormState>(DEFAULT_FORM)
  const [showAdvanced, setShowAdvanced] = useState(false)
  const [saved, setSaved] = useState(false)
  const [quotaExceeded, setQuotaExceeded] = useState<QuotaExceededState | null>(null)

  function update<K extends keyof FormState>(key: K, value: FormState[K]) {
    setForm((prev) => ({ ...prev, [key]: value }))
  }

  function canProceed(): boolean {
    if (step === 0) return form.type !== ""
    if (step === 1) return form.name.trim() !== "" && form.target.trim() !== ""
    return true
  }

  async function handleCreate() {
    // Clear any previous quota error.
    setQuotaExceeded(null)

    try {
      const res = await fetch("/api/v1/monitors", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          name: form.name,
          type: form.type,
          target: form.target,
          interval_s: form.intervalSeconds,
          node_count: form.concurrentNodes,
        }),
      })

      if (res.status === 402) {
        // Quota exceeded — parse the plan information from the response.
        const data = await res.json().catch(() => ({}))
        const message: string = data.message ?? ""
        // Attempt to parse plan/used/limit from the error message.
        // Fallback: show generic "free" exceeded message.
        const planMatch = message.match(/您的\s+(\S+)\s+档/)
        const planKey = planMatch ? planMatch[1].toLowerCase() : "free"
        const usedMatch = message.match(/(\d+)\s+个监控项/)
        const usedCount = usedMatch ? parseInt(usedMatch[1], 10) : 0
        const limitMatch = message.match(/上限\s+(\d+)/)
        const limitCount = limitMatch
          ? parseInt(limitMatch[1], 10)
          : PLAN_MONITOR_LIMITS[planKey] ?? 3

        setQuotaExceeded({
          currentPlan: planKey,
          usedCount,
          limitCount,
        })
        return
      }

      if (!res.ok) {
        // Non-quota error — handle generically (could be extended).
        return
      }

      setSaved(true)
      setTimeout(() => {
        router.push("/app/monitors")
      }, 1500)
    } catch {
      // Network error — silent fail in this stub implementation.
    }
  }

  return (
    <div className="max-w-2xl">
      <StepIndicator currentStep={step} totalSteps={STEP_LABELS.length} />

      {/* Step 0: 类型选择 */}
      {step === 0 && (
        <div className="space-y-4">
          <div>
            <h2 className="text-xl font-semibold">选择监控类型</h2>
            <p className="text-sm text-muted-foreground mt-1">
              根据您的需求选择合适的监控类型
            </p>
          </div>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
            {MONITOR_TYPES.map((type) => (
              <Card
                key={type}
                data-testid={`type-card-${type}`}
                className={[
                  "cursor-pointer transition-all hover:border-primary hover:shadow-md",
                  form.type === type ? "border-primary ring-2 ring-primary" : "",
                ].join(" ")}
                onClick={() => update("type", type)}
              >
                <CardContent className="p-4">
                  <div className="flex items-start justify-between">
                    <Badge variant="outline" className="text-xs">
                      {TYPE_LABELS[type]}
                    </Badge>
                    {form.type === type && (
                      <Check className="h-4 w-4 text-primary shrink-0" />
                    )}
                  </div>
                  <p className="mt-2 text-xs text-muted-foreground leading-relaxed">
                    {TYPE_DESCRIPTIONS[type]}
                  </p>
                </CardContent>
              </Card>
            ))}
          </div>
        </div>
      )}

      {/* Step 1: 基础配置 */}
      {step === 1 && (
        <div className="space-y-4">
          <div>
            <h2 className="text-xl font-semibold">基础配置</h2>
            <p className="text-sm text-muted-foreground mt-1">
              配置监控的基本参数
            </p>
          </div>
          <Card>
            <CardContent className="space-y-4 pt-6">
              <div className="space-y-2">
                <Label htmlFor="monitor-name">监控名称</Label>
                <Input
                  id="monitor-name"
                  placeholder="例如：主站可用性监控"
                  value={form.name}
                  onChange={(e) => update("name", e.target.value)}
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="monitor-target">目标地址</Label>
                <Input
                  id="monitor-target"
                  placeholder={
                    form.type
                      ? TARGET_PLACEHOLDERS[form.type]
                      : "请先选择监控类型"
                  }
                  value={form.target}
                  onChange={(e) => update("target", e.target.value)}
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="monitor-interval">检测频率</Label>
                <Select
                  value={String(form.intervalSeconds)}
                  onValueChange={(v) => update("intervalSeconds", Number(v))}
                >
                  <SelectTrigger id="monitor-interval">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="60">每 1 分钟</SelectItem>
                    <SelectItem value="300">每 5 分钟</SelectItem>
                    <SelectItem value="1800">每 30 分钟</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-2">
                <Label htmlFor="monitor-nodes">并发节点数</Label>
                <Select
                  value={String(form.concurrentNodes)}
                  onValueChange={(v) => update("concurrentNodes", Number(v))}
                >
                  <SelectTrigger id="monitor-nodes">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="1">1 个节点</SelectItem>
                    <SelectItem value="3">3 个节点</SelectItem>
                    <SelectItem value="5">5 个节点</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </CardContent>
          </Card>
        </div>
      )}

      {/* Step 2: 高级配置 */}
      {step === 2 && (
        <div className="space-y-4">
          <div>
            <h2 className="text-xl font-semibold">高级配置</h2>
            <p className="text-sm text-muted-foreground mt-1">
              可选配置，不填写则使用默认值
            </p>
          </div>

          <button
            type="button"
            className="flex w-full items-center justify-between rounded-md border p-4 text-sm font-medium hover:bg-muted/50 transition-colors"
            onClick={() => setShowAdvanced((v) => !v)}
          >
            <span>展开高级配置</span>
            <ChevronDown
              className={[
                "h-4 w-4 transition-transform",
                showAdvanced ? "rotate-180" : "",
              ].join(" ")}
            />
          </button>

          {showAdvanced && (
            <Card>
              <CardContent className="space-y-4 pt-6">
                {(form.type === "http" || form.type === "https") && (
                  <>
                    <div className="space-y-2">
                      <Label htmlFor="assert-status">断言状态码</Label>
                      <Input
                        id="assert-status"
                        placeholder="200"
                        value={form.assertStatusCode}
                        onChange={(e) =>
                          update("assertStatusCode", e.target.value)
                        }
                      />
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="keyword-match">关键字匹配</Label>
                      <Input
                        id="keyword-match"
                        placeholder="页面中必须包含的文字"
                        value={form.keywordMatch}
                        onChange={(e) =>
                          update("keywordMatch", e.target.value)
                        }
                      />
                    </div>
                  </>
                )}

                {form.type === "ping" && (
                  <>
                    <div className="space-y-2">
                      <Label htmlFor="timeout-ms">超时时间 (ms)</Label>
                      <Input
                        id="timeout-ms"
                        placeholder="5000"
                        value={form.timeoutMs}
                        onChange={(e) => update("timeoutMs", e.target.value)}
                      />
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="packet-loss">丢包阈值 (%)</Label>
                      <Input
                        id="packet-loss"
                        placeholder="10"
                        value={form.packetLossThreshold}
                        onChange={(e) =>
                          update("packetLossThreshold", e.target.value)
                        }
                      />
                    </div>
                  </>
                )}

                {form.type === "tcp" && (
                  <div className="space-y-2">
                    <Label htmlFor="tcp-port">端口</Label>
                    <Input
                      id="tcp-port"
                      placeholder="80"
                      value={form.port}
                      onChange={(e) => update("port", e.target.value)}
                    />
                  </div>
                )}

                {form.type === "dns" && (
                  <div className="space-y-2">
                    <Label htmlFor="expected-ip">预期 IP</Label>
                    <Input
                      id="expected-ip"
                      placeholder="104.21.0.1"
                      value={form.expectedIp}
                      onChange={(e) => update("expectedIp", e.target.value)}
                    />
                  </div>
                )}

                {form.type !== "http" &&
                  form.type !== "https" &&
                  form.type !== "ping" &&
                  form.type !== "tcp" &&
                  form.type !== "dns" && (
                    <p className="text-sm text-muted-foreground">
                      此类型暂无额外高级配置项
                    </p>
                  )}
              </CardContent>
            </Card>
          )}
        </div>
      )}

      {/* Step 3: 确认创建 */}
      {step === 3 && (
        <div className="space-y-4">
          <div>
            <h2 className="text-xl font-semibold">确认并创建</h2>
            <p className="text-sm text-muted-foreground mt-1">
              请确认以下配置信息
            </p>
          </div>

          <Card>
            <CardHeader>
              <CardTitle className="text-base">配置摘要</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              <div className="flex justify-between text-sm">
                <span className="text-muted-foreground">监控类型</span>
                <Badge variant="outline">
                  {form.type ? TYPE_LABELS[form.type] : "-"}
                </Badge>
              </div>
              <div className="flex justify-between text-sm">
                <span className="text-muted-foreground">监控名称</span>
                <span className="font-medium">{form.name || "-"}</span>
              </div>
              <div className="flex justify-between text-sm">
                <span className="text-muted-foreground">目标地址</span>
                <span className="font-mono text-xs max-w-[200px] truncate">
                  {form.target || "-"}
                </span>
              </div>
              <div className="flex justify-between text-sm">
                <span className="text-muted-foreground">检测频率</span>
                <span>
                  {form.intervalSeconds === 60
                    ? "每 1 分钟"
                    : form.intervalSeconds === 300
                      ? "每 5 分钟"
                      : "每 30 分钟"}
                </span>
              </div>
              <div className="flex justify-between text-sm">
                <span className="text-muted-foreground">并发节点数</span>
                <span>{form.concurrentNodes} 个节点</span>
              </div>
            </CardContent>
          </Card>

          {/* Quota exceeded alert */}
          {quotaExceeded && (
            <Alert variant="destructive" data-testid="quota-exceeded-alert">
              <AlertCircle className="h-4 w-4" />
              <AlertTitle>配额已用完</AlertTitle>
              <AlertDescription className="mt-2 space-y-3">
                <p>
                  您的{" "}
                  <strong>
                    {PLAN_LABELS[quotaExceeded.currentPlan] ?? quotaExceeded.currentPlan}
                  </strong>{" "}
                  档已用完 {quotaExceeded.limitCount} 个监控项。
                  {(() => {
                    const nextPlan = quotaExceeded.currentPlan === "free" ? "Pro" : null
                    const nextLimit =
                      quotaExceeded.currentPlan === "free" ? PLAN_MONITOR_LIMITS["pro"] : null
                    if (nextPlan && nextLimit) {
                      return `升级 ${nextPlan} 可管理 ${nextLimit} 个监控项。`
                    }
                    return "请升级套餐以管理更多监控项。"
                  })()}
                </p>
                <Button
                  variant="default"
                  size="sm"
                  onClick={() => router.push("/app/billing")}
                >
                  升级 Pro
                </Button>
              </AlertDescription>
            </Alert>
          )}

          {saved && (
            <div className="flex items-center gap-2 rounded-md bg-success/10 px-4 py-3 text-sm text-success">
              <Check className="h-4 w-4" />
              监控创建成功！正在跳转...
            </div>
          )}
        </div>
      )}

      {/* 导航按钮 */}
      <div className="mt-8 flex items-center justify-between">
        <Button
          variant="outline"
          onClick={() => (step === 0 ? router.push("/app/monitors") : setStep((s) => s - 1))}
        >
          <ArrowLeft className="mr-2 h-4 w-4" />
          {step === 0 ? "取消" : "上一步"}
        </Button>

        {step < STEP_LABELS.length - 1 ? (
          <Button onClick={() => setStep((s) => s + 1)} disabled={!canProceed()}>
            下一步
            <ArrowRight className="ml-2 h-4 w-4" />
          </Button>
        ) : (
          <Button onClick={handleCreate} disabled={saved}>
            <Check className="mr-2 h-4 w-4" />
            创建监控
          </Button>
        )}
      </div>
    </div>
  )
}
