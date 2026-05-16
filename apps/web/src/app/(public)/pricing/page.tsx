"use client"

import { useState } from "react"
import Link from "next/link"
import { Check, Zap, Building2, Users, Star } from "lucide-react"
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

interface PlanFeature {
  label: string
  value: string
}

interface Plan {
  id: string
  name: string
  icon: React.ElementType
  monthlyPrice: number | null
  yearlyPrice: number | null
  priceLabel?: string
  description: string
  badge?: string
  features: PlanFeature[]
  cta: string
  ctaHref: string
  ctaVariant: "default" | "outline"
  highlighted: boolean
}

const PLANS: Plan[] = [
  {
    id: "free",
    name: "Free",
    icon: Zap,
    monthlyPrice: 0,
    yearlyPrice: 0,
    description: "个人项目和小型站点的入门选择",
    features: [
      { label: "监控数量", value: "5 个" },
      { label: "检测频率", value: "10 分钟" },
      { label: "探测节点", value: "3 个节点" },
      { label: "告警通道", value: "Email" },
      { label: "API 配额", value: "1,000 次/月" },
      { label: "团队成员", value: "1 人" },
      { label: "状态页", value: "不支持" },
      { label: "MCP Units", value: "独立配额池" },
    ],
    cta: "免费开始",
    ctaHref: "/auth/register",
    ctaVariant: "outline",
    highlighted: false,
  },
  {
    id: "pro",
    name: "Pro",
    icon: Star,
    monthlyPrice: 29,
    yearlyPrice: 24,
    description: "专业个人用户，更高频率和更多监控",
    badge: "最受欢迎",
    features: [
      { label: "监控数量", value: "50 个" },
      { label: "检测频率", value: "1 分钟" },
      { label: "探测节点", value: "全部节点" },
      { label: "告警通道", value: "Email + Webhook" },
      { label: "API 配额", value: "10 万次/月" },
      { label: "团队成员", value: "1 人" },
      { label: "状态页", value: "1 个" },
      { label: "MCP Units", value: "独立配额池" },
    ],
    cta: "开始试用",
    ctaHref: "/auth/register",
    ctaVariant: "default",
    highlighted: true,
  },
  {
    id: "team",
    name: "Team",
    icon: Users,
    monthlyPrice: 99,
    yearlyPrice: 79,
    description: "小团队协作，支持企微和钉钉告警",
    features: [
      { label: "监控数量", value: "200 个" },
      { label: "检测频率", value: "30 秒" },
      { label: "探测节点", value: "全部节点" },
      { label: "告警通道", value: "Email + Webhook + 企微/钉钉" },
      { label: "API 配额", value: "100 万次/月" },
      { label: "团队成员", value: "10 人" },
      { label: "状态页", value: "5 个" },
      { label: "MCP Units", value: "独立配额池" },
    ],
    cta: "开始试用",
    ctaHref: "/auth/register",
    ctaVariant: "outline",
    highlighted: false,
  },
  {
    id: "business",
    name: "Business",
    icon: Building2,
    monthlyPrice: 299,
    yearlyPrice: 239,
    description: "企业级监控，私有节点和电话告警",
    features: [
      { label: "监控数量", value: "1,000 个" },
      { label: "检测频率", value: "10 秒" },
      { label: "探测节点", value: "全部节点 + 私有节点" },
      { label: "告警通道", value: "Email + Webhook + 企微/钉钉 + 电话" },
      { label: "API 配额", value: "无限" },
      { label: "团队成员", value: "50 人" },
      { label: "状态页", value: "无限" },
      { label: "MCP Units", value: "独立配额池" },
    ],
    cta: "开始试用",
    ctaHref: "/auth/register",
    ctaVariant: "outline",
    highlighted: false,
  },
]

function PriceDisplay({
  plan,
  cycle,
}: {
  plan: Plan
  cycle: BillingCycle
}) {
  const price = cycle === "monthly" ? plan.monthlyPrice : plan.yearlyPrice
  const originalMonthly = plan.monthlyPrice

  if (price === null) {
    return (
      <div className="mt-4">
        <span className="text-3xl font-bold">联系我们</span>
      </div>
    )
  }

  if (price === 0) {
    return (
      <div className="mt-4">
        <span className="text-3xl font-bold">¥0</span>
        <span className="ml-1 text-sm text-muted-foreground">/月</span>
      </div>
    )
  }

  return (
    <div className="mt-4">
      <div className="flex items-baseline gap-1">
        <span className="text-3xl font-bold">¥{price}</span>
        <span className="text-sm text-muted-foreground">/月</span>
      </div>
      {cycle === "yearly" && originalMonthly && (
        <p className="mt-1 text-xs text-muted-foreground">
          原价{" "}
          <span className="line-through">¥{originalMonthly}/月</span>
          {" · "}年付 ¥{price * 12}/年
        </p>
      )}
    </div>
  )
}

export default function PricingPage() {
  const [cycle, setCycle] = useState<BillingCycle>("monthly")

  return (
    <div className="flex flex-col">
      <section className="py-16 text-center">
        <div className="mx-auto max-w-screen-xl px-4 sm:px-6 lg:px-8">
          <h1 className="text-4xl font-bold tracking-tight">简单透明的定价</h1>
          <p className="mt-4 text-lg text-muted-foreground">
            从免费开始，随业务增长灵活升级。无隐藏收费，随时取消。
          </p>

          <div className="mt-8 flex justify-center">
            <Tabs
              value={cycle}
              onValueChange={(v) => setCycle(v as BillingCycle)}
            >
              <TabsList>
                <TabsTrigger value="monthly">月付</TabsTrigger>
                <TabsTrigger value="yearly">
                  年付
                  <Badge variant="outline" className="ml-2 text-xs">
                    省最多 20%
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
            {PLANS.map((plan) => {
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
                  {plan.badge && (
                    <div className="absolute -top-3 left-1/2 -translate-x-1/2">
                      <Badge className="px-3 py-1">{plan.badge}</Badge>
                    </div>
                  )}
                  <CardHeader className="pb-0">
                    <div className="flex items-center gap-2">
                      <div className="flex h-8 w-8 items-center justify-center rounded-md bg-primary/10">
                        <Icon className="h-4 w-4 text-primary" />
                      </div>
                      <span className="font-semibold">{plan.name}</span>
                    </div>
                    <PriceDisplay plan={plan} cycle={cycle} />
                    <p className="mt-2 text-sm text-muted-foreground">
                      {plan.description}
                    </p>
                  </CardHeader>

                  <CardContent className="pt-4">
                    <ul className="space-y-2.5">
                      {plan.features.map((feature) => (
                        <li
                          key={feature.label}
                          className="flex items-start gap-2 text-sm"
                        >
                          <Check className="mt-0.5 h-4 w-4 shrink-0 text-primary" />
                          <span className="text-muted-foreground">
                            {feature.label}：
                          </span>
                          <Badge
                            variant="outline"
                            className="ml-auto shrink-0 text-xs font-normal"
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
                      <Link href={plan.ctaHref}>{plan.cta}</Link>
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
                  <p className="font-semibold">Enterprise · 企业定制</p>
                  <p className="text-sm text-muted-foreground">
                    私有部署、专属 SLA、合规审计、定制集成方案
                  </p>
                </div>
              </div>
              <Button variant="outline" asChild className="shrink-0">
                <Link href="mailto:hello@idcd.com">联系销售团队</Link>
              </Button>
            </CardContent>
          </Card>

          <p className="mt-8 text-center text-sm text-muted-foreground">
            所有付费计划含 14 天免费试用 · 随时取消 · 无需信用卡
          </p>
        </div>
      </section>
    </div>
  )
}
