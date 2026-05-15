"use client"

import { useCallback, useEffect, useState } from "react"
import { useRouter, useSearchParams } from "next/navigation"
import { toast } from "sonner"
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  CardDescription,
} from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Separator } from "@/components/ui/separator"
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group"
import { Label } from "@/components/ui/label"
import { Skeleton } from "@/components/ui/skeleton"
import {
  CheckCircle2,
  XCircle,
  AlertCircle,
  FileText,
  Loader2,
} from "lucide-react"
import {
  getSubscription,
  getInvoices,
  subscribePlan,
  cancelSubscription,
  type Subscription,
  type Invoice,
} from "@/lib/api"

// ── Plan metadata ─────────────────────────────────────────────────────────────

interface PlanMeta {
  id: string
  name: string
  price: string
  features: {
    monitors: string
    frequency: string
    nodes: string
    alertChannels: string
    statusPages: string
    apiCallsPerDay: string
    customDomain: boolean
  }
}

const PLANS: PlanMeta[] = [
  {
    id: "free",
    name: "Free",
    price: "免费",
    features: {
      monitors: "3",
      frequency: "5 分钟",
      nodes: "1",
      alertChannels: "1",
      statusPages: "0",
      apiCallsPerDay: "100",
      customDomain: false,
    },
  },
  {
    id: "pro",
    name: "Pro",
    price: "¥99 / 月",
    features: {
      monitors: "50",
      frequency: "1 分钟",
      nodes: "5",
      alertChannels: "5",
      statusPages: "3",
      apiCallsPerDay: "5,000",
      customDomain: true,
    },
  },
  {
    id: "team",
    name: "Team",
    price: "¥299 / 月",
    features: {
      monitors: "200",
      frequency: "1 分钟",
      nodes: "10",
      alertChannels: "20",
      statusPages: "10",
      apiCallsPerDay: "50,000",
      customDomain: true,
    },
  },
  {
    id: "business",
    name: "Business",
    price: "¥999 / 月",
    features: {
      monitors: "无限",
      frequency: "30 秒",
      nodes: "20",
      alertChannels: "无限",
      statusPages: "无限",
      apiCallsPerDay: "无限",
      customDomain: true,
    },
  },
]

const STATUS_LABELS: Record<string, string> = {
  active: "已激活",
  pending: "待支付",
  cancelled: "已取消",
  past_due: "账单逾期",
}

// ── Helpers ───────────────────────────────────────────────────────────────────

interface FeatureRowProps {
  label: string
  values: (string | boolean)[]
}

function FeatureRow({ label, values }: FeatureRowProps) {
  return (
    <TableRow>
      <TableCell className="text-sm font-medium text-muted-foreground">
        {label}
      </TableCell>
      {values.map((val, i) => (
        <TableCell key={i} className="text-center text-sm">
          {typeof val === "boolean" ? (
            val ? (
              <CheckCircle2 className="mx-auto h-4 w-4 text-success" />
            ) : (
              <XCircle className="mx-auto h-4 w-4 text-muted-foreground" />
            )
          ) : (
            val
          )}
        </TableCell>
      ))}
    </TableRow>
  )
}

function formatAmount(cents: number, currency: string) {
  const symbol = currency === "CNY" ? "¥" : "$"
  return `${symbol}${(cents / 100).toFixed(2)}`
}

function formatDate(iso?: string) {
  if (!iso) return "—"
  return new Date(iso).toLocaleDateString("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  })
}

// ── Main component ────────────────────────────────────────────────────────────

export function BillingClient() {
  const router = useRouter()
  const searchParams = useSearchParams()

  const [subscription, setSubscription] = useState<Subscription | null>(null)
  const [invoices, setInvoices] = useState<Invoice[]>([])
  const [loadingSub, setLoadingSub] = useState(true)
  const [loadingInv, setLoadingInv] = useState(true)

  const [upgradeTarget, setUpgradeTarget] = useState<PlanMeta | null>(null)
  const [channel, setChannel] = useState<"alipay" | "wechat_pay">("alipay")
  const [subscribing, setSubscribing] = useState(false)
  const [cancelling, setCancelling] = useState(false)

  const currentPlanId =
    subscription?.status === "active" ? subscription.plan : "free"
  const currentPlan = PLANS.find((p) => p.id === currentPlanId) ?? PLANS[0]!
  const isActivePaid =
    subscription?.status === "active" && subscription?.plan !== "free"

  const loadData = useCallback(async () => {
    const [sub, inv] = await Promise.allSettled([
      getSubscription(),
      getInvoices(),
    ])
    setSubscription(sub.status === "fulfilled" ? sub.value : null)
    setInvoices(
      inv.status === "fulfilled" ? (inv.value.invoices ?? []) : []
    )
    setLoadingSub(false)
    setLoadingInv(false)
  }, [])

  useEffect(() => {
    loadData()
  }, [loadData])

  // Handle ?success=1 redirect back from payment gateway
  useEffect(() => {
    if (searchParams.get("success") === "1") {
      toast.success("支付成功！订阅正在激活，稍后刷新页面查看最新状态。")
      router.replace("/app/billing")
    }
  }, [searchParams, router])

  function handleUpgrade(plan: PlanMeta) {
    setUpgradeTarget(plan)
    setChannel("alipay")
  }

  async function handleConfirmUpgrade() {
    if (!upgradeTarget) return
    setSubscribing(true)
    try {
      const result = await subscribePlan(upgradeTarget.id, channel)
      setUpgradeTarget(null)
      window.open(result.pay_url, "_blank", "noopener,noreferrer")
      toast.info("支付页面已在新标签页打开，完成支付后请刷新此页面。")
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "发起支付失败，请重试")
    } finally {
      setSubscribing(false)
    }
  }

  async function handleCancel() {
    if (!confirm("确认取消订阅？当前订阅周期结束前仍可正常使用。")) return
    setCancelling(true)
    try {
      await cancelSubscription()
      toast.success("订阅已取消")
      await loadData()
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "取消订阅失败")
    } finally {
      setCancelling(false)
    }
  }

  return (
    <div className="space-y-8" data-testid="billing-page">
      {/* ── 当前订阅 ── */}
      <Card data-testid="current-plan-card">
        <CardHeader>
          <div className="flex items-center gap-3">
            <CardTitle>当前订阅</CardTitle>
            {loadingSub ? (
              <Skeleton className="h-5 w-16" />
            ) : (
              <Badge variant="secondary" data-testid="current-plan-badge">
                {currentPlan.name}
              </Badge>
            )}
            {subscription?.status && subscription.status !== "active" && (
              <Badge variant="destructive" className="text-xs">
                {STATUS_LABELS[subscription.status] ?? subscription.status}
              </Badge>
            )}
          </div>
          {loadingSub ? (
            <Skeleton className="h-4 w-48 mt-1" />
          ) : (
            <CardDescription>
              {isActivePaid
                ? `${currentPlan.name} 版，到期：${formatDate(subscription?.current_period_end)}`
                : "您目前使用的是免费版本"}
            </CardDescription>
          )}
        </CardHeader>
        <CardContent className="space-y-4">
          {loadingSub ? (
            <div className="space-y-2">
              <Skeleton className="h-4 w-full" />
              <Skeleton className="h-4 w-3/4" />
              <Skeleton className="h-4 w-5/6" />
            </div>
          ) : (
            <div className="rounded-lg bg-muted/40 px-4 py-3 text-sm space-y-1.5">
              <div className="flex justify-between">
                <span className="text-muted-foreground">监控数上限</span>
                <span className="font-medium">{currentPlan.features.monitors} 个</span>
              </div>
              <div className="flex justify-between">
                <span className="text-muted-foreground">最小检测频率</span>
                <span className="font-medium">{currentPlan.features.frequency}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-muted-foreground">并发节点数</span>
                <span className="font-medium">{currentPlan.features.nodes} 个</span>
              </div>
            </div>
          )}
          {!loadingSub && (
            <div className="flex flex-wrap gap-2">
              {!isActivePaid && (
                <Button
                  size="lg"
                  className="sm:w-auto"
                  data-testid="upgrade-button"
                  onClick={() => handleUpgrade(PLANS[1]!)}
                >
                  升级到 Pro
                </Button>
              )}
              {isActivePaid && (
                <Button
                  variant="outline"
                  size="sm"
                  data-testid="cancel-button"
                  disabled={cancelling}
                  onClick={handleCancel}
                >
                  {cancelling && (
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  )}
                  取消订阅
                </Button>
              )}
            </div>
          )}
        </CardContent>
      </Card>

      {/* ── 账单逾期提示 ── */}
      {subscription?.status === "past_due" && (
        <Alert variant="destructive" data-testid="past-due-alert">
          <AlertCircle className="h-4 w-4" />
          <AlertTitle>账单逾期</AlertTitle>
          <AlertDescription>
            您的订阅账单逾期未付，部分功能可能受限。请重新订阅以恢复服务。
          </AlertDescription>
        </Alert>
      )}

      {/* ── 定价对比表格 ── */}
      <Card data-testid="pricing-table">
        <CardHeader>
          <CardTitle>方案对比</CardTitle>
        </CardHeader>
        <CardContent className="overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-40">功能</TableHead>
                {PLANS.map((plan) => (
                  <TableHead key={plan.id} className="text-center min-w-[120px]">
                    <div className="space-y-1">
                      <div className="font-semibold">
                        {plan.name}
                        {plan.id === currentPlanId && (
                          <Badge variant="secondary" className="ml-2 text-xs">
                            当前
                          </Badge>
                        )}
                      </div>
                      <div className="text-xs text-muted-foreground font-normal">
                        {plan.price}
                      </div>
                    </div>
                  </TableHead>
                ))}
              </TableRow>
            </TableHeader>
            <TableBody>
              <FeatureRow
                label="监控数量"
                values={PLANS.map((p) => p.features.monitors)}
              />
              <FeatureRow
                label="最小频率"
                values={PLANS.map((p) => p.features.frequency)}
              />
              <FeatureRow
                label="并发节点数"
                values={PLANS.map((p) => p.features.nodes)}
              />
              <FeatureRow
                label="告警通道"
                values={PLANS.map((p) => p.features.alertChannels)}
              />
              <FeatureRow
                label="状态页"
                values={PLANS.map((p) => p.features.statusPages)}
              />
              <FeatureRow
                label="API 调用 / 天"
                values={PLANS.map((p) => p.features.apiCallsPerDay)}
              />
              <FeatureRow
                label="自定义域名"
                values={PLANS.map((p) => p.features.customDomain)}
              />
              <TableRow>
                <TableCell />
                {PLANS.map((plan) => (
                  <TableCell key={plan.id} className="text-center py-4">
                    {loadingSub ? (
                      <Skeleton className="mx-auto h-8 w-24" />
                    ) : plan.id === currentPlanId ? (
                      <Button
                        variant="secondary"
                        size="sm"
                        disabled
                        data-testid={`plan-button-${plan.id}`}
                      >
                        当前方案
                      </Button>
                    ) : (
                      <Button
                        variant="outline"
                        size="sm"
                        data-testid={`plan-button-${plan.id}`}
                        onClick={() => handleUpgrade(plan)}
                      >
                        升级到 {plan.name}
                      </Button>
                    )}
                  </TableCell>
                ))}
              </TableRow>
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      <Separator />

      {/* ── 发票列表 ── */}
      <div data-testid="invoice-section">
        <h2 className="text-lg font-semibold mb-4">发票记录</h2>
        {loadingInv ? (
          <Card>
            <CardContent className="p-4 space-y-2">
              <Skeleton className="h-8 w-full" />
              <Skeleton className="h-8 w-full" />
              <Skeleton className="h-8 w-3/4" />
            </CardContent>
          </Card>
        ) : invoices.length === 0 ? (
          <Card>
            <CardContent className="flex flex-col items-center justify-center py-12 text-center">
              <FileText className="h-10 w-10 text-muted-foreground/50 mb-3" />
              <p
                className="text-sm text-muted-foreground"
                data-testid="empty-invoice-text"
              >
                暂无发票记录
              </p>
              <p className="text-xs text-muted-foreground/70 mt-1">
                升级到付费方案后，发票将在此处显示
              </p>
            </CardContent>
          </Card>
        ) : (
          <Card>
            <CardContent className="p-0">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>金额</TableHead>
                    <TableHead>状态</TableHead>
                    <TableHead>支付时间</TableHead>
                    <TableHead>渠道</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {invoices.map((inv) => (
                    <TableRow key={inv.id} data-testid={`invoice-row-${inv.id}`}>
                      <TableCell className="font-medium">
                        {formatAmount(inv.amount_cents, inv.currency)}
                      </TableCell>
                      <TableCell>
                        <Badge
                          variant={inv.status === "paid" ? "default" : "secondary"}
                        >
                          {inv.status === "paid"
                            ? "已支付"
                            : inv.status === "refunded"
                              ? "已退款"
                              : "退款失败"}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-muted-foreground text-sm">
                        {formatDate(inv.paid_at ?? inv.created_at)}
                      </TableCell>
                      <TableCell className="text-muted-foreground text-sm capitalize">
                        {inv.provider}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        )}
      </div>

      {/* ── 升级支付弹窗 ── */}
      <Dialog
        open={!!upgradeTarget}
        onOpenChange={(o) => !o && setUpgradeTarget(null)}
      >
        <DialogContent data-testid="upgrade-dialog">
          <DialogHeader>
            <DialogTitle>升级到 {upgradeTarget?.name}</DialogTitle>
            <DialogDescription>
              {upgradeTarget?.price} · 选择支付方式后跳转完成支付
            </DialogDescription>
          </DialogHeader>

          <div className="py-2">
            <p className="text-sm font-medium mb-3">选择支付方式</p>
            <RadioGroup
              value={channel}
              onValueChange={(v) => setChannel(v as "alipay" | "wechat_pay")}
              className="space-y-2"
            >
              <div className="flex items-center space-x-2 rounded-lg border px-3 py-2.5 cursor-pointer hover:bg-muted/40">
                <RadioGroupItem value="alipay" id="alipay" />
                <Label htmlFor="alipay" className="cursor-pointer flex-1">
                  支付宝
                </Label>
              </div>
              <div className="flex items-center space-x-2 rounded-lg border px-3 py-2.5 cursor-pointer hover:bg-muted/40">
                <RadioGroupItem value="wechat_pay" id="wechat_pay" />
                <Label htmlFor="wechat_pay" className="cursor-pointer flex-1">
                  微信支付
                </Label>
              </div>
            </RadioGroup>
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setUpgradeTarget(null)}>
              取消
            </Button>
            <Button
              data-testid="confirm-upgrade-button"
              disabled={subscribing}
              onClick={handleConfirmUpgrade}
            >
              {subscribing && (
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              )}
              去支付
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
