"use client"

import { useState } from "react"
import { useRouter } from "next/navigation"
import { useTranslations } from "next-intl"
import { toast } from "sonner"
import { apiRequest } from "@/lib/api"
import {
  ArrowLeft,
  ArrowRight,
  Check,
  ChevronDown,
  ChevronRight,
  AlertCircle,
  Cpu,
  Wrench,
  Database,
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
import { Textarea } from "@/components/ui/textarea"
import { type MonitorType, TYPE_LABELS, MONITOR_TYPES, AGENT_OBS_TYPES } from "../types"

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

const AGENT_OBS_ICONS: Record<string, React.ComponentType<{ className?: string }>> = {
  llm_endpoint: Cpu,
  tool_api: Wrench,
  rag: Database,
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
  sslExpiryDays: string  // 到期前 N 天告警
  // agent obs (M21/M22/M23)
  agentObsEndpointUrl: string
  agentObsModelName: string
  agentObsLatencySlaMs: string
  agentObsPayloadTemplate: string
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
  sslExpiryDays: "30",
  agentObsEndpointUrl: "",
  agentObsModelName: "",
  agentObsLatencySlaMs: "5000",
  agentObsPayloadTemplate: "",
}

function StepIndicator({
  currentStep,
  totalSteps,
  stepLabels,
}: {
  currentStep: number
  totalSteps: number
  stepLabels: string[]
}) {
  return (
    <div className="flex items-center gap-2 mb-8">
      {stepLabels.map((label, i) => (
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
  const t = useTranslations("monitors")
  const [step, setStep] = useState(0)
  const [form, setForm] = useState<FormState>(DEFAULT_FORM)
  const [showAdvanced, setShowAdvanced] = useState(false)
  const [saved, setSaved] = useState(false)
  const [quotaExceeded, setQuotaExceeded] = useState<QuotaExceededState | null>(null)

  const STEP_LABELS = [
    t("new.step0"),
    t("new.step1"),
    t("new.step2"),
    t("new.step3"),
  ]

  const TYPE_DESCRIPTIONS: Record<MonitorType, string> = {
    http: t("new.typeDescriptions.http"),
    https: t("new.typeDescriptions.https"),
    ping: t("new.typeDescriptions.ping"),
    tcp: t("new.typeDescriptions.tcp"),
    dns: t("new.typeDescriptions.dns"),
    ssl_expiry: t("new.typeDescriptions.ssl_expiry"),
    domain_expiry: t("new.typeDescriptions.domain_expiry"),
    icp_change: t("new.typeDescriptions.icp_change"),
    keyword: t("new.typeDescriptions.keyword"),
    llm_endpoint: t("new.typeDescriptions.llm_endpoint"),
    tool_api: t("new.typeDescriptions.tool_api"),
    rag: t("new.typeDescriptions.rag"),
  }

  const TARGET_PLACEHOLDERS: Record<MonitorType, string> = {
    http: "http://example.com/api/health",
    https: "https://example.com",
    ping: t("new.targetPlaceholders.ping"),
    tcp: "example.com:8080",
    dns: "example.com",
    ssl_expiry: "example.com",
    domain_expiry: "example.com",
    icp_change: "example.com",
    keyword: "https://example.com",
    llm_endpoint: "https://api.openai.com/v1/chat/completions",
    tool_api: "https://tool.example.com/api",
    rag: "https://rag.example.com/query",
  }

  function update<K extends keyof FormState>(key: K, value: FormState[K]) {
    setForm((prev) => ({ ...prev, [key]: value }))
  }

  function canProceed(): boolean {
    if (step === 0) return form.type !== ""
    if (step === 1) return form.name.trim() !== "" && form.target.trim() !== ""
    return true
  }

  function buildConfig(f: FormState): Record<string, unknown> {
    const cfg: Record<string, unknown> = {}

    const timeoutMs = parseInt(f.timeoutMs, 10)
    if (!isNaN(timeoutMs) && timeoutMs > 0) cfg.timeout_ms = timeoutMs

    switch (f.type) {
      case "http":
      case "https": {
        const code = parseInt(f.assertStatusCode, 10)
        if (!isNaN(code) && code > 0) cfg.assert_status_code = code
        if (f.keywordMatch.trim()) cfg.keyword = f.keywordMatch.trim()
        break
      }
      case "keyword": {
        const code = parseInt(f.assertStatusCode, 10)
        if (!isNaN(code) && code > 0) cfg.assert_status_code = code
        if (f.keywordMatch.trim()) cfg.keyword = f.keywordMatch.trim()
        break
      }
      case "ping": {
        const threshold = parseFloat(f.packetLossThreshold)
        if (!isNaN(threshold)) cfg.packet_loss_threshold = threshold
        break
      }
      case "tcp": {
        const port = parseInt(f.port, 10)
        if (!isNaN(port) && port > 0) cfg.port = port
        break
      }
      case "dns": {
        if (f.expectedIp.trim()) cfg.expected_ip = f.expectedIp.trim()
        break
      }
      case "ssl_expiry":
      case "domain_expiry": {
        const days = parseInt(f.sslExpiryDays, 10)
        if (!isNaN(days) && days > 0) cfg.expiry_warning_days = days
        break
      }
      case "llm_endpoint": {
        if (f.agentObsEndpointUrl.trim()) cfg.endpoint_url = f.agentObsEndpointUrl.trim()
        if (f.agentObsModelName.trim()) cfg.model_name = f.agentObsModelName.trim()
        const latencySla = parseInt(f.agentObsLatencySlaMs, 10)
        if (!isNaN(latencySla) && latencySla > 0) cfg.latency_sla_ms = latencySla
        if (f.agentObsPayloadTemplate.trim()) cfg.payload_template = f.agentObsPayloadTemplate.trim()
        break
      }
    }

    return cfg
  }

  async function handleCreate() {
    setQuotaExceeded(null)

    try {
      const created = await apiRequest<{ id: string }>("/v1/monitors", {
        method: "POST",
        body: JSON.stringify({
          name: form.name,
          type: form.type,
          target: form.target,
          interval_s: form.intervalSeconds,
          node_count: form.concurrentNodes,
          config: buildConfig(form),
        }),
      })

      setSaved(true)
      setTimeout(() => {
        const id = created?.id
        router.push(id ? `/app/monitors/${id}` : "/app/monitors")
      }, 1500)
    } catch (err) {
      const message = err instanceof Error ? err.message : t("error.createFailed")

      const planMatch = message.match(/您的\s+(\S+)\s+档/)
      if (planMatch) {
        const planKey = planMatch[1]!.toLowerCase()
        const usedMatch = message.match(/(\d+)\s+个监控项/)
        const usedCount = usedMatch ? parseInt(usedMatch[1]!, 10) : 0
        const limitMatch = message.match(/上限\s+(\d+)/)
        const limitCount = limitMatch
          ? parseInt(limitMatch[1]!, 10)
          : PLAN_MONITOR_LIMITS[planKey] ?? 3

        setQuotaExceeded({ currentPlan: planKey, usedCount, limitCount })
        return
      }

      toast.error(t("error.createFailed") + ": " + message)
    }
  }

  return (
    <div className="max-w-2xl">
      <StepIndicator currentStep={step} totalSteps={STEP_LABELS.length} stepLabels={STEP_LABELS} />

      {/* Step 0: 类型选择 */}
      {step === 0 && (
        <div className="space-y-4">
          <div>
            <h2 className="text-xl font-semibold">{t("new.selectType")}</h2>
            <p className="text-sm text-muted-foreground mt-1">
              {t("new.selectTypeDesc")}
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

          <div className="space-y-2">
            <p className="text-sm font-medium text-muted-foreground">{t("new.agentMonitor")}</p>
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
              {AGENT_OBS_TYPES.map((type) => {
                const Icon = AGENT_OBS_ICONS[type]
                return (
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
                        <div className="flex items-center gap-1.5">
                          {Icon && <Icon className="h-3.5 w-3.5 text-muted-foreground" />}
                          <Badge variant="outline" className="text-xs">
                            {TYPE_LABELS[type]}
                          </Badge>
                        </div>
                        {form.type === type && (
                          <Check className="h-4 w-4 text-primary shrink-0" />
                        )}
                      </div>
                      <p className="mt-2 text-xs text-muted-foreground leading-relaxed">
                        {TYPE_DESCRIPTIONS[type]}
                      </p>
                    </CardContent>
                  </Card>
                )
              })}
            </div>
          </div>
        </div>
      )}

      {/* Step 1: 基础配置 */}
      {step === 1 && (
        <div className="space-y-4">
          <div>
            <h2 className="text-xl font-semibold">{t("new.basicConfig")}</h2>
            <p className="text-sm text-muted-foreground mt-1">
              {t("new.basicConfigDesc")}
            </p>
          </div>
          <Card>
            <CardContent className="space-y-4 pt-6">
              <div className="space-y-2">
                <Label htmlFor="monitor-name">{t("new.monitorName")}</Label>
                <Input
                  id="monitor-name"
                  placeholder={t("new.monitorNamePlaceholder")}
                  value={form.name}
                  onChange={(e) => update("name", e.target.value)}
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="monitor-target">{t("new.targetAddress")}</Label>
                <Input
                  id="monitor-target"
                  placeholder={
                    form.type
                      ? TARGET_PLACEHOLDERS[form.type]
                      : t("new.targetAddressDefault")
                  }
                  value={form.target}
                  onChange={(e) => update("target", e.target.value)}
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="monitor-interval">{t("new.checkInterval")}</Label>
                <Select
                  value={String(form.intervalSeconds)}
                  onValueChange={(v) => update("intervalSeconds", Number(v))}
                >
                  <SelectTrigger id="monitor-interval">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="60">{t("new.interval1m")}</SelectItem>
                    <SelectItem value="300">{t("new.interval5m")}</SelectItem>
                    <SelectItem value="1800">{t("new.interval30m")}</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-2">
                <Label htmlFor="monitor-nodes">{t("new.concurrentNodes")}</Label>
                <Select
                  value={String(form.concurrentNodes)}
                  onValueChange={(v) => update("concurrentNodes", Number(v))}
                >
                  <SelectTrigger id="monitor-nodes">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="1">{t("new.node1")}</SelectItem>
                    <SelectItem value="3">{t("new.node3")}</SelectItem>
                    <SelectItem value="5">{t("new.node5")}</SelectItem>
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
            <h2 className="text-xl font-semibold">{t("new.advancedConfig")}</h2>
            <p className="text-sm text-muted-foreground mt-1">
              {t("new.advancedConfigDesc")}
            </p>
          </div>

          <Button
            type="button"
            variant="outline"
            className="w-full justify-between"
            onClick={() => setShowAdvanced((v) => !v)}
          >
            <span>{t("new.expandAdvanced")}</span>
            <ChevronDown
              className={[
                "h-4 w-4 transition-transform",
                showAdvanced ? "rotate-180" : "",
              ].join(" ")}
            />
          </Button>

          {showAdvanced && (
            <Card>
              <CardContent className="space-y-4 pt-6">
                {(form.type === "http" || form.type === "https") && (
                  <>
                    <div className="space-y-2">
                      <Label htmlFor="assert-status">{t("new.assertStatus")}</Label>
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
                      <Label htmlFor="keyword-match">{t("new.keywordMatch")}</Label>
                      <Input
                        id="keyword-match"
                        placeholder={t("new.keywordPlaceholder")}
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
                      <Label htmlFor="timeout-ms">{t("new.timeoutMs")}</Label>
                      <Input
                        id="timeout-ms"
                        placeholder="5000"
                        value={form.timeoutMs}
                        onChange={(e) => update("timeoutMs", e.target.value)}
                      />
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="packet-loss">{t("new.packetLoss")}</Label>
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
                    <Label htmlFor="tcp-port">{t("new.port")}</Label>
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
                    <Label htmlFor="expected-ip">{t("new.expectedIp")}</Label>
                    <Input
                      id="expected-ip"
                      placeholder="104.21.0.1"
                      value={form.expectedIp}
                      onChange={(e) => update("expectedIp", e.target.value)}
                    />
                  </div>
                )}

                {(form.type === "ssl_expiry" || form.type === "domain_expiry") && (
                  <div className="space-y-2">
                    <Label htmlFor="ssl-expiry-days">
                      {t("new.sslExpiryDays")}
                    </Label>
                    <Input
                      id="ssl-expiry-days"
                      type="number"
                      placeholder="30"
                      min="1"
                      max="365"
                      value={form.sslExpiryDays}
                      onChange={(e) => update("sslExpiryDays", e.target.value)}
                    />
                    <p className="text-xs text-muted-foreground">
                      {t("new.sslExpiryDaysHint")}
                    </p>
                  </div>
                )}

                {form.type === "llm_endpoint" && (
                  <>
                    <div className="space-y-2">
                      <Label htmlFor="agent-obs-endpoint">{t("new.endpointUrl")} <span className="text-destructive">*</span></Label>
                      <Input
                        id="agent-obs-endpoint"
                        data-testid="agent-obs-endpoint-url"
                        placeholder="https://api.openai.com/v1/chat/completions"
                        value={form.agentObsEndpointUrl}
                        onChange={(e) => update("agentObsEndpointUrl", e.target.value)}
                      />
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="agent-obs-model">{t("new.modelName")}</Label>
                      <Input
                        id="agent-obs-model"
                        data-testid="agent-obs-model-name"
                        placeholder="gpt-4"
                        value={form.agentObsModelName}
                        onChange={(e) => update("agentObsModelName", e.target.value)}
                      />
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="agent-obs-latency">{t("new.latencySla")}</Label>
                      <Input
                        id="agent-obs-latency"
                        data-testid="agent-obs-latency-sla"
                        placeholder="5000"
                        value={form.agentObsLatencySlaMs}
                        onChange={(e) => update("agentObsLatencySlaMs", e.target.value)}
                      />
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="agent-obs-payload">{t("new.payloadTemplate")}</Label>
                      <Textarea
                        id="agent-obs-payload"
                        data-testid="agent-obs-payload-template"
                        placeholder='{"model":"gpt-4","messages":[{"role":"user","content":"ping"}]}'
                        value={form.agentObsPayloadTemplate}
                        onChange={(e) => update("agentObsPayloadTemplate", e.target.value)}
                        rows={4}
                      />
                    </div>
                  </>
                )}

                {form.type !== "http" &&
                  form.type !== "https" &&
                  form.type !== "ping" &&
                  form.type !== "tcp" &&
                  form.type !== "dns" &&
                  form.type !== "ssl_expiry" &&
                  form.type !== "domain_expiry" &&
                  form.type !== "llm_endpoint" && (
                    <p className="text-sm text-muted-foreground">
                      {t("new.noAdvancedConfig")}
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
            <h2 className="text-xl font-semibold">{t("new.confirmTitle")}</h2>
            <p className="text-sm text-muted-foreground mt-1">
              {t("new.confirmDesc")}
            </p>
          </div>

          <Card>
            <CardHeader>
              <CardTitle className="text-base">{t("detail.configSummary")}</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              <div className="flex justify-between text-sm">
                <span className="text-muted-foreground">{t("detail.monitorType")}</span>
                <Badge variant="outline">
                  {form.type ? TYPE_LABELS[form.type] : "-"}
                </Badge>
              </div>
              <div className="flex justify-between text-sm">
                <span className="text-muted-foreground">{t("detail.monitorName")}</span>
                <span className="font-medium">{form.name || "-"}</span>
              </div>
              <div className="flex justify-between text-sm">
                <span className="text-muted-foreground">{t("detail.targetUrl")}</span>
                <span className="font-mono text-xs max-w-[200px] truncate">
                  {form.target || "-"}
                </span>
              </div>
              <div className="flex justify-between text-sm">
                <span className="text-muted-foreground">{t("detail.checkInterval")}</span>
                <span>
                  {form.intervalSeconds === 60
                    ? t("new.interval1m")
                    : form.intervalSeconds === 300
                      ? t("new.interval5m")
                      : t("new.interval30m")}
                </span>
              </div>
              <div className="flex justify-between text-sm">
                <span className="text-muted-foreground">{t("detail.nodeCount")}</span>
                <span>{form.concurrentNodes}</span>
              </div>
            </CardContent>
          </Card>

          {/* Quota exceeded alert */}
          {quotaExceeded && (
            <Alert variant="destructive" data-testid="quota-exceeded-alert">
              <AlertCircle className="h-4 w-4" />
              <AlertTitle>{t("new.quotaTitle")}</AlertTitle>
              <AlertDescription className="mt-2 space-y-3">
                <p>
                  {(() => {
                    const nextPlan = quotaExceeded.currentPlan === "free" ? "Pro" : null
                    const nextLimit =
                      quotaExceeded.currentPlan === "free" ? PLAN_MONITOR_LIMITS["pro"] : null
                    if (nextPlan && nextLimit) {
                      return t("new.quotaUsed", {
                        plan: PLAN_LABELS[quotaExceeded.currentPlan] ?? quotaExceeded.currentPlan,
                        limit: quotaExceeded.limitCount,
                        nextPlan,
                        nextLimit,
                      })
                    }
                    return t("new.quotaUpgrade")
                  })()}
                </p>
                <Button
                  variant="default"
                  size="sm"
                  onClick={() => router.push("/app/billing")}
                >
                  {t("new.upgradePro")}
                </Button>
              </AlertDescription>
            </Alert>
          )}

          {saved && (
            <div className="flex items-center gap-2 rounded-md bg-success/10 px-4 py-3 text-sm text-success">
              <Check className="h-4 w-4" />
              {t("new.createSuccess")}
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
          {step === 0 ? t("new.cancel") : t("new.prev")}
        </Button>

        {step < STEP_LABELS.length - 1 ? (
          <Button onClick={() => setStep((s) => s + 1)} disabled={!canProceed()}>
            {t("new.next")}
            <ArrowRight className="ml-2 h-4 w-4" />
          </Button>
        ) : (
          <Button onClick={handleCreate} disabled={saved}>
            <Check className="mr-2 h-4 w-4" />
            {t("new.createMonitor")}
          </Button>
        )}
      </div>
    </div>
  )
}
