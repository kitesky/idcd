"use client"

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
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Separator } from "@/components/ui/separator"
import { CheckCircle2, XCircle, Info, FileText } from "lucide-react"

interface Plan {
  id: string
  name: string
  price: string
  current: boolean
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

const PLANS: Plan[] = [
  {
    id: "free",
    name: "Free",
    price: "免费",
    current: true,
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
    current: false,
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
    current: false,
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
    current: false,
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

export function BillingClient() {
  const currentPlan = PLANS.find((p) => p.current)!

  return (
    <div className="space-y-8" data-testid="billing-page">
      {/* ── 当前订阅 ── */}
      <Card data-testid="current-plan-card">
        <CardHeader>
          <div className="flex items-center gap-3">
            <CardTitle>当前订阅</CardTitle>
            <Badge variant="secondary" data-testid="current-plan-badge">
              {currentPlan.name}
            </Badge>
          </div>
          <CardDescription>您目前使用的是免费版本</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="rounded-lg bg-muted/40 px-4 py-3 text-sm space-y-1.5">
            <div className="flex justify-between">
              <span className="text-muted-foreground">监控数上限</span>
              <span className="font-medium">3 个</span>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground">最小检测频率</span>
              <span className="font-medium">5 分钟</span>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground">并发节点数</span>
              <span className="font-medium">1 个</span>
            </div>
          </div>
          <Button size="lg" className="w-full sm:w-auto" data-testid="upgrade-button">
            升级到 Pro
          </Button>
        </CardContent>
      </Card>

      {/* ── Paddle 占位提示 ── */}
      <Alert data-testid="paddle-notice">
        <Info className="h-4 w-4" />
        <AlertTitle>支付功能即将上线</AlertTitle>
        <AlertDescription>
          目前 Pro 公测免费使用，正式收费前会提前通知您。
        </AlertDescription>
      </Alert>

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
                        {plan.current && (
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
                    <Button
                      variant={plan.current ? "secondary" : "outline"}
                      size="sm"
                      disabled={plan.current}
                      data-testid={`plan-button-${plan.id}`}
                    >
                      {plan.current ? "当前方案" : `升级到 ${plan.name}`}
                    </Button>
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
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-12 text-center">
            <FileText className="h-10 w-10 text-muted-foreground/50 mb-3" />
            <p className="text-sm text-muted-foreground">暂无发票记录</p>
            <p className="text-xs text-muted-foreground/70 mt-1">
              升级到付费方案后，发票将在此处显示
            </p>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
