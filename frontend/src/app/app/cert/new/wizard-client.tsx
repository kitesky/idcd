"use client"

import { useEffect, useMemo, useState } from "react"
import Link from "next/link"
import { useRouter } from "next/navigation"
import { toast } from "sonner"
import {
  AlertCircle,
  ArrowLeft,
  ArrowRight,
  Check,
  ChevronRight,
} from "lucide-react"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Label } from "@/components/ui/label"
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Separator } from "@/components/ui/separator"
import { Textarea } from "@/components/ui/textarea"
import { createOrder, listDnsCredentials } from "../cert-api"
import {
  CA_OPTIONS,
  DNS_PROVIDER_LABELS,
  isIdn,
  isWildcard,
  parseSanInput,
  toPunycode,
  type CaProvider,
  type ChallengeMode,
  type DnsCredential,
} from "../types"

const STEP_LABELS = ["域名", "CA 选择", "验证方式", "确认"] as const

interface StepIndicatorProps {
  currentStep: number
}

// StepIndicator renders the numbered chip + label sequence for the wizard.
// shadcn does not ship a Stepper primitive — we compose Separator + numbered
// circles + chevrons per the design system rules.
function StepIndicator({ currentStep }: StepIndicatorProps) {
  return (
    <div className="mb-6 flex flex-wrap items-center gap-2">
      {STEP_LABELS.map((label, i) => (
        <div key={label} className="flex items-center gap-2">
          <div
            data-testid={`wizard-step-${i}`}
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
          {i < STEP_LABELS.length - 1 && (
            <ChevronRight className="h-4 w-4 text-muted-foreground" />
          )}
        </div>
      ))}
    </div>
  )
}

interface WizardState {
  sanInput: string
  ca: CaProvider
  challenge: ChallengeMode
  dnsCredentialId: string
}

const DEFAULT_STATE: WizardState = {
  sanInput: "",
  ca: "letsencrypt",
  challenge: "dns01-auto",
  dnsCredentialId: "",
}

export function WizardClient() {
  const router = useRouter()
  const [step, setStep] = useState(0)
  const [state, setState] = useState<WizardState>(DEFAULT_STATE)
  const [submitting, setSubmitting] = useState(false)
  const [creds, setCreds] = useState<DnsCredential[]>([])
  const [credsLoading, setCredsLoading] = useState(true)

  useEffect(() => {
    let mounted = true
    listDnsCredentials()
      .then((cs) => {
        if (!mounted) return
        setCreds(cs)
        // Auto-select the first healthy credential.
        const healthy = cs.find((c) => c.health === "healthy")
        if (healthy) {
          setState((prev) =>
            prev.dnsCredentialId ? prev : { ...prev, dnsCredentialId: healthy.id },
          )
        }
      })
      .finally(() => {
        if (mounted) setCredsLoading(false)
      })
    return () => {
      mounted = false
    }
  }, [])

  const sans = useMemo(() => parseSanInput(state.sanInput), [state.sanInput])
  const hasWildcard = sans.some(isWildcard)
  const idnHosts = sans.filter(isIdn)

  const availableCas = useMemo(() => {
    if (!hasWildcard) return CA_OPTIONS
    return CA_OPTIONS.filter((c) => c.supportsWildcard)
  }, [hasWildcard])

  // Derived: the effective CA falls back to the first available option when
  // the user toggled a wildcard that disqualifies the previously chosen CA
  // (lego limitation surfaced as a UX hint).
  const effectiveCa: CaProvider =
    availableCas.find((c) => c.id === state.ca)?.id ??
    availableCas[0]?.id ??
    "letsencrypt"

  function update<K extends keyof WizardState>(key: K, value: WizardState[K]) {
    setState((prev) => ({ ...prev, [key]: value }))
  }

  function canProceed(): boolean {
    if (step === 0) return sans.length > 0
    if (step === 1) return availableCas.length > 0
    if (step === 2) {
      if (state.challenge === "dns01-auto") return state.dnsCredentialId !== ""
      return true
    }
    return true
  }

  async function submit() {
    setSubmitting(true)
    try {
      const sanForSubmit = sans.map(toPunycode)
      const created = await createOrder({
        san: sanForSubmit,
        ca: effectiveCa,
        challenge: state.challenge,
        dnsCredentialId:
          state.challenge === "dns01-auto" ? state.dnsCredentialId : undefined,
      })
      toast.success("订单已创建")
      router.push(`/app/cert/orders/${created.id}`)
    } catch (err) {
      const message = err instanceof Error ? err.message : "未知错误"
      toast.error(`提交失败：${message}`)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="max-w-3xl">
      <StepIndicator currentStep={step} />

      <Card>
        <CardHeader>
          <CardTitle className="text-base">{STEP_LABELS[step]}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          {step === 0 && (
            <>
              <div className="space-y-2">
                <Label htmlFor="san-input">域名（SAN）</Label>
                <Textarea
                  id="san-input"
                  data-testid="san-input"
                  placeholder={"每行一个，或用逗号分隔\nexample.com\nwww.example.com\n*.dev.example.com"}
                  rows={6}
                  value={state.sanInput}
                  onChange={(e) => update("sanInput", e.target.value)}
                />
                <p className="text-xs text-muted-foreground">
                  支持通配符（*.example.com）和国际化域名（IDN，将自动转 Punycode）。
                </p>
              </div>
              {sans.length > 0 && (
                <div className="space-y-2">
                  <p className="text-sm font-medium">已识别 {sans.length} 个 SAN：</p>
                  <div className="flex flex-wrap gap-2" data-testid="san-preview">
                    {sans.map((s) => (
                      <Badge
                        key={s}
                        variant={isWildcard(s) ? "info" : "outline"}
                        className="font-mono text-xs"
                      >
                        {isIdn(s) ? `${s} → ${toPunycode(s)}` : s}
                      </Badge>
                    ))}
                  </div>
                  {idnHosts.length > 0 && (
                    <Alert>
                      <AlertCircle className="h-4 w-4" />
                      <AlertTitle>检测到 IDN 域名</AlertTitle>
                      <AlertDescription>
                        以下域名将以 Punycode 形式提交给 CA：
                        {idnHosts.map((h) => ` ${toPunycode(h)}`).join(",")}
                      </AlertDescription>
                    </Alert>
                  )}
                </div>
              )}
            </>
          )}

          {step === 1 && (
            <>
              {hasWildcard && (
                <Alert>
                  <AlertCircle className="h-4 w-4" />
                  <AlertTitle>包含通配符 SAN</AlertTitle>
                  <AlertDescription>
                    通配符证书仅支持以下 CA。Buypass Go 暂不支持通配符，已隐藏。
                  </AlertDescription>
                </Alert>
              )}
              <RadioGroup
                value={effectiveCa}
                onValueChange={(v) => update("ca", v as CaProvider)}
                className="grid gap-3"
              >
                {availableCas.map((ca) => (
                  <Label
                    key={ca.id}
                    htmlFor={`ca-${ca.id}`}
                    className="flex cursor-pointer items-start gap-3 rounded-md border p-4 has-[[data-state=checked]]:border-primary"
                  >
                    <RadioGroupItem
                      id={`ca-${ca.id}`}
                      value={ca.id}
                      className="mt-1"
                    />
                    <div className="flex-1 space-y-1">
                      <div className="flex items-center justify-between">
                        <span className="font-medium">{ca.label}</span>
                        <Badge variant="outline" className="text-[10px]">
                          {ca.validityDays} 天
                        </Badge>
                      </div>
                      <p className="text-xs text-muted-foreground">
                        速率限制：{ca.rateLimit}
                        {!ca.supportsWildcard && " · 不支持通配符"}
                      </p>
                    </div>
                  </Label>
                ))}
              </RadioGroup>
            </>
          )}

          {step === 2 && (
            <>
              <RadioGroup
                value={state.challenge}
                onValueChange={(v) => update("challenge", v as ChallengeMode)}
                className="grid gap-3"
              >
                <Label
                  htmlFor="ch-auto"
                  className="flex cursor-pointer items-start gap-3 rounded-md border p-4 has-[[data-state=checked]]:border-primary"
                >
                  <RadioGroupItem id="ch-auto" value="dns01-auto" className="mt-1" />
                  <div className="flex-1 space-y-1">
                    <span className="font-medium">DNS-01 自动</span>
                    <p className="text-xs text-muted-foreground">
                      使用已登记的 DNS 凭据自动写入 TXT，签发完成后自动清理。
                    </p>
                  </div>
                </Label>
                <Label
                  htmlFor="ch-manual"
                  className="flex cursor-pointer items-start gap-3 rounded-md border p-4 has-[[data-state=checked]]:border-primary"
                >
                  <RadioGroupItem
                    id="ch-manual"
                    value="dns01-manual"
                    className="mt-1"
                  />
                  <div className="flex-1 space-y-1">
                    <span className="font-medium">DNS-01 手动</span>
                    <p className="text-xs text-muted-foreground">
                      系统返回 TXT 记录值，你自行添加并在订单详情页确认。
                    </p>
                  </div>
                </Label>
              </RadioGroup>

              {state.challenge === "dns01-auto" && (
                <div className="space-y-2">
                  <Label htmlFor="dns-cred">DNS 凭据</Label>
                  {credsLoading ? (
                    <p className="text-sm text-muted-foreground">加载中…</p>
                  ) : creds.length === 0 ? (
                    <Alert>
                      <AlertCircle className="h-4 w-4" />
                      <AlertTitle>没有可用的 DNS 凭据</AlertTitle>
                      <AlertDescription>
                        请先到{" "}
                        <Link
                          href="/app/cert/dns-credentials"
                          className="text-primary underline-offset-4 hover:underline"
                        >
                          DNS 凭据
                        </Link>{" "}
                        页面添加。
                      </AlertDescription>
                    </Alert>
                  ) : (
                    <Select
                      value={state.dnsCredentialId}
                      onValueChange={(v) => update("dnsCredentialId", v)}
                    >
                      <SelectTrigger id="dns-cred" data-testid="dns-cred-trigger">
                        <SelectValue placeholder="选择一条 DNS 凭据" />
                      </SelectTrigger>
                      <SelectContent>
                        {creds.map((c) => (
                          <SelectItem key={c.id} value={c.id}>
                            {c.displayName}（{DNS_PROVIDER_LABELS[c.provider]}）
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  )}
                </div>
              )}
            </>
          )}

          {step === 3 && (
            <>
              <SummaryRow label="SAN">
                <div className="flex flex-wrap justify-end gap-1">
                  {sans.map((s) => (
                    <Badge key={s} variant="outline" className="font-mono text-xs">
                      {toPunycode(s)}
                    </Badge>
                  ))}
                </div>
              </SummaryRow>
              <Separator />
              <SummaryRow label="CA">
                <span>{CA_OPTIONS.find((c) => c.id === effectiveCa)?.label}</span>
              </SummaryRow>
              <Separator />
              <SummaryRow label="验证方式">
                <span>
                  {state.challenge === "dns01-auto"
                    ? "DNS-01 自动"
                    : "DNS-01 手动"}
                </span>
              </SummaryRow>
              {state.challenge === "dns01-auto" && (
                <>
                  <Separator />
                  <SummaryRow label="DNS 凭据">
                    <span>
                      {creds.find((c) => c.id === state.dnsCredentialId)
                        ?.displayName ?? "—"}
                    </span>
                  </SummaryRow>
                </>
              )}
            </>
          )}
        </CardContent>
      </Card>

      <div className="mt-6 flex items-center justify-between">
        <Button
          variant="outline"
          onClick={() => (step === 0 ? router.push("/app/cert") : setStep((s) => s - 1))}
        >
          <ArrowLeft className="mr-2 h-4 w-4" />
          {step === 0 ? "取消" : "上一步"}
        </Button>
        {step < STEP_LABELS.length - 1 ? (
          <Button
            data-testid="wizard-next"
            onClick={() => setStep((s) => s + 1)}
            disabled={!canProceed()}
          >
            下一步
            <ArrowRight className="ml-2 h-4 w-4" />
          </Button>
        ) : (
          <Button
            data-testid="wizard-submit"
            onClick={submit}
            disabled={submitting || !canProceed()}
          >
            <Check className="mr-2 h-4 w-4" />
            {submitting ? "提交中…" : "提交申请"}
          </Button>
        )}
      </div>
    </div>
  )
}

function SummaryRow({
  label,
  children,
}: {
  label: string
  children: React.ReactNode
}) {
  return (
    <div className="flex items-start justify-between gap-4 text-sm">
      <span className="text-muted-foreground">{label}</span>
      <div className="text-right">{children}</div>
    </div>
  )
}
