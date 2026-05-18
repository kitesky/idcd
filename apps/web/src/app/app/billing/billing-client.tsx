"use client"

import { useCallback, useEffect, useState } from "react"
import { useRouter, useSearchParams } from "next/navigation"
import { useTranslations, useLocale } from "next-intl"
import { bcp47Of } from "@/i18n/registry"
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

type Translator = (
  key: string,
  params?: Record<string, string | number | boolean | Date | null | undefined>,
) => string

/**
 * Build the plan catalog with locale-aware labels. `price`, frequency and
 * "unlimited" values come from `billing.plan.prices.*` / `freqLabels.*` /
 * `unlimited` so each locale can format currency words appropriately.
 */
function getPlans(t: Translator): PlanMeta[] {
  const unlimited = t("plan.unlimited")
  return [
    {
      id: "free",
      name: "Free",
      price: t("plan.prices.free"),
      features: {
        monitors: "3",
        frequency: t("plan.freqLabels.min5"),
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
      price: t("plan.prices.pro"),
      features: {
        monitors: "50",
        frequency: t("plan.freqLabels.min1"),
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
      price: t("plan.prices.team"),
      features: {
        monitors: "200",
        frequency: t("plan.freqLabels.min1"),
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
      price: t("plan.prices.business"),
      features: {
        monitors: unlimited,
        frequency: t("plan.freqLabels.sec30"),
        nodes: "20",
        alertChannels: unlimited,
        statusPages: unlimited,
        apiCallsPerDay: unlimited,
        customDomain: true,
      },
    },
  ]
}

function getStatusLabels(t: Translator): Record<string, string> {
  return {
    active: t("plan.statusActive"),
    pending: t("plan.statusPending"),
    cancelled: t("plan.statusCancelled"),
    past_due: t("plan.statusPastDue"),
  }
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

function formatDate(iso: string | undefined, bcp47: string) {
  if (!iso) return "—"
  return new Date(iso).toLocaleDateString(bcp47, {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  })
}

// ── Main component ────────────────────────────────────────────────────────────

export function BillingClient() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const t = useTranslations("billing")
  const locale = useLocale()
  const bcp47 = bcp47Of(locale)
  const PLANS = getPlans(t as never)
  const STATUS_LABELS = getStatusLabels(t as never)

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
    // eslint-disable-next-line react-hooks/set-state-in-effect -- loadData 内部 await 后 setState；初次挂载触发
    void loadData()
  }, [loadData])

  // Handle ?success=1 redirect back from payment gateway
  useEffect(() => {
    if (searchParams.get("success") === "1") {
      toast.success(t("plan.paySuccessShort"))
      router.replace("/app/billing")
    }
  }, [searchParams, router, t])

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
      toast.info(t("plan.payTabOpenedShort"))
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("plan.payFailed"))
    } finally {
      setSubscribing(false)
    }
  }

  async function handleCancel() {
    if (!confirm(t("plan.cancelConfirm"))) return
    setCancelling(true)
    try {
      await cancelSubscription()
      toast.success(t("plan.cancelSuccess"))
      await loadData()
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("plan.cancelFailed"))
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
            <CardTitle>{t("plan.current")}</CardTitle>
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
                ? t("plan.summarySub", { plan: currentPlan.name, date: formatDate(subscription?.current_period_end, bcp47) })
                : t("plan.freePlan")}
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
                <span className="text-muted-foreground">{t("plan.monitors")}</span>
                <span className="font-medium">{currentPlan.features.monitors} {t("plan.unit")}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-muted-foreground">{t("plan.frequency")}</span>
                <span className="font-medium">{currentPlan.features.frequency}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-muted-foreground">{t("plan.nodes")}</span>
                <span className="font-medium">{currentPlan.features.nodes} {t("plan.unit")}</span>
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
                  {t("plan.upgrade")}
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
                  {t("plan.cancel")}
                </Button>
              )}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Past-due alert */}
      {subscription?.status === "past_due" && (
        <Alert variant="destructive" data-testid="past-due-alert">
          <AlertCircle className="h-4 w-4" />
          <AlertTitle>{t("plan.pastDueTitle")}</AlertTitle>
          <AlertDescription>
            {t("plan.pastDueBlock")}
          </AlertDescription>
        </Alert>
      )}

      {/* Pricing comparison table */}
      <Card data-testid="pricing-table">
        <CardHeader>
          <CardTitle>{t("plan.comparison")}</CardTitle>
        </CardHeader>
        <CardContent className="overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-40">{t("plan.comparisonFeature")}</TableHead>
                {PLANS.map((plan) => (
                  <TableHead key={plan.id} className="text-center min-w-[120px]">
                    <div className="space-y-1">
                      <div className="font-semibold">
                        {plan.name}
                        {plan.id === currentPlanId && (
                          <Badge variant="secondary" className="ml-2 text-xs">
                            {t("plan.currentBadge2")}
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
                label={t("plan.monitorCount")}
                values={PLANS.map((p) => p.features.monitors)}
              />
              <FeatureRow
                label={t("plan.minFrequency")}
                values={PLANS.map((p) => p.features.frequency)}
              />
              <FeatureRow
                label={t("plan.nodeCount")}
                values={PLANS.map((p) => p.features.nodes)}
              />
              <FeatureRow
                label={t("plan.alertChannels")}
                values={PLANS.map((p) => p.features.alertChannels)}
              />
              <FeatureRow
                label={t("plan.statusPages")}
                values={PLANS.map((p) => p.features.statusPages)}
              />
              <FeatureRow
                label={t("plan.apiCallsPerDay")}
                values={PLANS.map((p) => p.features.apiCallsPerDay)}
              />
              <FeatureRow
                label={t("plan.customDomain")}
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
                        {t("plan.currentPlanBtn")}
                      </Button>
                    ) : (
                      <Button
                        variant="outline"
                        size="sm"
                        data-testid={`plan-button-${plan.id}`}
                        onClick={() => handleUpgrade(plan)}
                      >
                        {t("plan.upgradeToPlan", { plan: plan.name })}
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

      {/* Invoice list */}
      <div data-testid="invoice-section">
        <h2 className="text-lg font-semibold mb-4">{t("invoices.title")}</h2>
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
                {t("invoices.empty")}
              </p>
              <p className="text-xs text-muted-foreground/70 mt-1">
                {t("invoices.emptyUpgradeHint")}
              </p>
            </CardContent>
          </Card>
        ) : (
          <Card>
            <CardContent className="p-0">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("invoices.amount")}</TableHead>
                    <TableHead>{t("invoices.statusHeader")}</TableHead>
                    <TableHead>{t("invoices.paidAt")}</TableHead>
                    <TableHead>{t("invoices.channel")}</TableHead>
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
                            ? t("invoices.status.paid")
                            : inv.status === "refunded"
                              ? t("invoices.status.refunded")
                              : t("invoices.status.refundFailed")}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-muted-foreground text-sm">
                        {formatDate(inv.paid_at ?? inv.created_at, bcp47)}
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
            <DialogTitle>{t("plan.upgradeToPlan", { plan: upgradeTarget?.name ?? "" })}</DialogTitle>
            <DialogDescription>
              {t("plan.upgradeDialogDesc", { price: upgradeTarget?.price ?? "" })}
            </DialogDescription>
          </DialogHeader>

          <div className="py-2">
            <p className="text-sm font-medium mb-3">{t("plan.paymentMethod")}</p>
            <RadioGroup
              value={channel}
              onValueChange={(v) => setChannel(v as "alipay" | "wechat_pay")}
              className="space-y-2"
            >
              <div className="flex items-center space-x-2 rounded-lg border px-3 py-2.5 cursor-pointer hover:bg-muted/40">
                <RadioGroupItem value="alipay" id="alipay" />
                <Label htmlFor="alipay" className="cursor-pointer flex-1">
                  {t("plan.alipay")}
                </Label>
              </div>
              <div className="flex items-center space-x-2 rounded-lg border px-3 py-2.5 cursor-pointer hover:bg-muted/40">
                <RadioGroupItem value="wechat_pay" id="wechat_pay" />
                <Label htmlFor="wechat_pay" className="cursor-pointer flex-1">
                  {t("plan.wechatPay")}
                </Label>
              </div>
            </RadioGroup>
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setUpgradeTarget(null)}>
              {t("plan.cancelBtn")}
            </Button>
            <Button
              data-testid="confirm-upgrade-button"
              disabled={subscribing}
              onClick={handleConfirmUpgrade}
            >
              {subscribing && (
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              )}
              {t("plan.payBtn")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
