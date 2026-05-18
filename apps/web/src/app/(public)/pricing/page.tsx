"use client"

import { useState } from "react"
import Link from "next/link"
import { Check, Zap, Building2, Users, Star } from "lucide-react"
import { useTranslations } from "next-intl"
import {
  Card,
  CardHeader,
  CardContent,
  CardFooter,
  Badge,
  Button,
  Tabs,
  TabsList,
  TabsTrigger,
  TabsContent,
} from "@/components/ui"

type BillingCycle = "monthly" | "yearly"

interface PlanFeatureEntry {
  featureKey: string
  value: string
}

interface PlanConfig {
  id: string
  icon: React.ElementType
  monthlyPrice: number | null
  yearlyPrice: number | null
  features: PlanFeatureEntry[]
  ctaHref: string
  ctaVariant: "default" | "outline"
  highlighted: boolean
  hasBadge: boolean
}

const PLAN_CONFIGS: PlanConfig[] = [
  {
    id: "free",
    icon: Zap,
    monthlyPrice: 0,
    yearlyPrice: 0,
    features: [
      { featureKey: "monitors", value: "5 个" },
      { featureKey: "checkFreq", value: "10 分钟" },
      { featureKey: "probeNodes", value: "3 个节点" },
      { featureKey: "alertChannels", value: "Email" },
      { featureKey: "apiQuota", value: "1,000 次/月" },
      { featureKey: "teamMembers", value: "1 人" },
      { featureKey: "statusPages", value: "不支持" },
      { featureKey: "mcpUnits", value: "独立配额池" },
    ],
    ctaHref: "/auth/register",
    ctaVariant: "outline",
    highlighted: false,
    hasBadge: false,
  },
  {
    id: "pro",
    icon: Star,
    monthlyPrice: 29,
    yearlyPrice: 24,
    features: [
      { featureKey: "monitors", value: "50 个" },
      { featureKey: "checkFreq", value: "1 分钟" },
      { featureKey: "probeNodes", value: "全部节点" },
      { featureKey: "alertChannels", value: "Email + Webhook" },
      { featureKey: "apiQuota", value: "10 万次/月" },
      { featureKey: "teamMembers", value: "1 人" },
      { featureKey: "statusPages", value: "1 个" },
      { featureKey: "mcpUnits", value: "独立配额池" },
    ],
    ctaHref: "/auth/register",
    ctaVariant: "default",
    highlighted: true,
    hasBadge: true,
  },
  {
    id: "team",
    icon: Users,
    monthlyPrice: 99,
    yearlyPrice: 79,
    features: [
      { featureKey: "monitors", value: "200 个" },
      { featureKey: "checkFreq", value: "30 秒" },
      { featureKey: "probeNodes", value: "全部节点" },
      { featureKey: "alertChannels", value: "Email + Webhook + 企微/钉钉" },
      { featureKey: "apiQuota", value: "100 万次/月" },
      { featureKey: "teamMembers", value: "10 人" },
      { featureKey: "statusPages", value: "5 个" },
      { featureKey: "mcpUnits", value: "独立配额池" },
    ],
    ctaHref: "/auth/register",
    ctaVariant: "outline",
    highlighted: false,
    hasBadge: false,
  },
  {
    id: "business",
    icon: Building2,
    monthlyPrice: 299,
    yearlyPrice: 239,
    features: [
      { featureKey: "monitors", value: "1,000 个" },
      { featureKey: "checkFreq", value: "10 秒" },
      { featureKey: "probeNodes", value: "全部节点 + 私有节点" },
      { featureKey: "alertChannels", value: "Email + Webhook + 企微/钉钉 + 电话" },
      { featureKey: "apiQuota", value: "无限" },
      { featureKey: "teamMembers", value: "50 人" },
      { featureKey: "statusPages", value: "无限" },
      { featureKey: "mcpUnits", value: "独立配额池" },
    ],
    ctaHref: "/auth/register",
    ctaVariant: "outline",
    highlighted: false,
    hasBadge: false,
  },
]

function PriceDisplay({
  plan,
  cycle,
  t,
}: {
  plan: PlanConfig
  cycle: BillingCycle
  t: ReturnType<typeof useTranslations>
}) {
  const price = cycle === "monthly" ? plan.monthlyPrice : plan.yearlyPrice
  const originalMonthly = plan.monthlyPrice

  if (price === null) {
    return (
      <div className="mt-4">
        <span className="text-3xl font-bold">{t("billing.contactUs")}</span>
      </div>
    )
  }

  if (price === 0) {
    return (
      <div className="mt-4">
        <span className="text-3xl font-bold">¥0</span>
        <span className="ml-1 text-sm text-muted-foreground">{t("billing.perMonth")}</span>
      </div>
    )
  }

  return (
    <div className="mt-4">
      <div className="flex items-baseline gap-1">
        <span className="text-3xl font-bold">¥{price}</span>
        <span className="text-sm text-muted-foreground">{t("billing.perMonth")}</span>
      </div>
      {cycle === "yearly" && originalMonthly && (
        <p className="mt-1 text-xs text-muted-foreground">
          {t("billing.originalPrice")}{" "}
          <span className="line-through">¥{originalMonthly}{t("billing.perMonth")}</span>
          {" · "}
          {t("billing.yearlyTotal", { price: String(price * 12) })}
        </p>
      )}
    </div>
  )
}

export default function PricingPage() {
  const t = useTranslations("pricing")
  const [cycle, setCycle] = useState<BillingCycle>("monthly")

  return (
    <div className="flex flex-col">
      <section className="py-16 text-center">
        <div className="mx-auto max-w-screen-xl px-4 sm:px-6 lg:px-8">
          <h1 className="text-4xl font-bold tracking-tight">{t("title")}</h1>
          <p className="mt-4 text-lg text-muted-foreground">
            {t("subtitle")}
          </p>

          <div className="mt-8 flex justify-center">
            <Tabs
              value={cycle}
              onValueChange={(v) => setCycle(v as BillingCycle)}
            >
              <TabsList>
                <TabsTrigger value="monthly">{t("billing.monthly")}</TabsTrigger>
                <TabsTrigger value="yearly">
                  {t("billing.yearly")}
                  <Badge variant="outline" className="ml-2 text-xs">
                    {t("billing.yearlySave")}
                  </Badge>
                </TabsTrigger>
              </TabsList>

              <TabsContent value="monthly" />
              <TabsContent value="yearly" />
            </Tabs>
          </div>
        </div>
      </section>

      <section className="pb-16">
        <div className="mx-auto max-w-screen-xl px-4 sm:px-6 lg:px-8">
          <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-4">
            {PLAN_CONFIGS.map((plan) => {
              const Icon = plan.icon
              return (
                <Card
                  key={plan.id}
                  className={
                    plan.highlighted
                      ? "relative border-primary shadow-lg"
                      : "relative"
                  }
                >
                  {plan.hasBadge && (
                    <div className="absolute -top-3 left-1/2 -translate-x-1/2">
                      <Badge className="px-3 py-1">{t(`plans.${plan.id}.badge` as never)}</Badge>
                    </div>
                  )}
                  <CardHeader className="pb-0">
                    <div className="flex items-center gap-2">
                      <div className="flex h-8 w-8 items-center justify-center rounded-md bg-primary/10">
                        <Icon className="h-4 w-4 text-primary" />
                      </div>
                      <span className="font-semibold">{t(`plans.${plan.id}.name` as never)}</span>
                    </div>
                    <PriceDisplay plan={plan} cycle={cycle} t={t} />
                    <p className="mt-2 text-sm text-muted-foreground">
                      {t(`plans.${plan.id}.description` as never)}
                    </p>
                  </CardHeader>

                  <CardContent className="pt-4">
                    <ul className="space-y-2.5">
                      {plan.features.map((feature) => (
                        <li
                          key={feature.featureKey}
                          className="flex items-center justify-between gap-2 text-sm"
                        >
                          <div className="flex shrink-0 items-center gap-1.5">
                            <Check className="h-4 w-4 text-primary" />
                            <span className="whitespace-nowrap text-muted-foreground">
                              {t(`features.${feature.featureKey}` as never)}
                            </span>
                          </div>
                          <Badge
                            variant="outline"
                            className="text-right text-xs font-normal"
                          >
                            {feature.value}
                          </Badge>
                        </li>
                      ))}
                    </ul>
                  </CardContent>

                  <CardFooter className="pt-2">
                    <Button
                      variant={plan.ctaVariant}
                      className="w-full"
                      asChild
                    >
                      <Link href={plan.ctaHref}>{t(`plans.${plan.id}.cta` as never)}</Link>
                    </Button>
                  </CardFooter>
                </Card>
              )
            })}
          </div>

          <Card className="mt-8">
            <CardContent className="flex flex-col gap-4 py-6 sm:flex-row sm:items-center sm:justify-between">
              <div className="flex items-center gap-3">
                <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-md bg-primary/10">
                  <Building2 className="h-5 w-5 text-primary" />
                </div>
                <div>
                  <p className="font-semibold">{t("enterprise.name")}</p>
                  <p className="text-sm text-muted-foreground">
                    {t("enterprise.description")}
                  </p>
                </div>
              </div>
              <Button variant="outline" asChild className="shrink-0">
                <Link href="mailto:hello@idcd.com">{t("enterprise.cta")}</Link>
              </Button>
            </CardContent>
          </Card>

          <p className="mt-8 text-center text-sm text-muted-foreground">
            {t("trial")}
          </p>
        </div>
      </section>
    </div>
  )
}
