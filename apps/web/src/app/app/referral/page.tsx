"use client"

import { useEffect, useState } from "react"
import { Gift } from "lucide-react"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { apiRequest } from "@/lib/api"

// ─── Types ────────────────────────────────────────────────────────────────────

interface ReferralCode {
  code: string
  referral_url: string
}

interface Reward {
  id: string
  referred_user_id: string
  status: "pending" | "credited"
  amount: string
  currency: string
  created_at: string
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function ReferralPage() {
  const [codeData, setCodeData] = useState<ReferralCode | null>(null)
  const [rewards, setRewards] = useState<Reward[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    async function fetchAll() {
      setLoading(true)
      setError(null)
      try {
        const [codeRes, rewardsRes] = await Promise.all([
          apiRequest<{ data: ReferralCode }>("/v1/referral/code"),
          apiRequest<{ data: { rewards: Reward[] } }>("/v1/referral/rewards"),
        ])
        setCodeData(codeRes.data)
        setRewards(rewardsRes.data.rewards)
      } catch (err: unknown) {
        setError(
          err instanceof Error ? err.message : "加载失败，请稍后重试"
        )
      } finally {
        setLoading(false)
      }
    }

    fetchAll()
  }, [])

  function handleCopy() {
    if (!codeData) return
    navigator.clipboard.writeText(codeData.referral_url).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    })
  }

  // ── Derived stats ──
  const totalPending = rewards
    .filter((r) => r.status === "pending")
    .reduce((sum, r) => sum + parseFloat(r.amount), 0)
    .toFixed(2)

  const totalCredited = rewards
    .filter((r) => r.status === "credited")
    .reduce((sum, r) => sum + parseFloat(r.amount), 0)
    .toFixed(2)

  // ── Loading ──
  if (loading) {
    return (
      <div className="flex flex-col gap-6" data-testid="referral-page">
        <div>
          <div className="flex items-center gap-2 mb-1">
            <Gift className="h-5 w-5 text-primary" />
            <h1 className="text-2xl font-semibold tracking-tight">推荐计划</h1>
          </div>
          <p className="text-sm text-muted-foreground">
            每推荐一位成功付费用户，获得 ¥10 账户余额
          </p>
        </div>
        <Card>
          <CardHeader>
            <Skeleton className="h-4 w-24" />
          </CardHeader>
          <CardContent>
            <Skeleton className="h-10 w-48" />
          </CardContent>
        </Card>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <Card key={i}>
              <CardHeader className="pb-2">
                <Skeleton className="h-4 w-16" />
              </CardHeader>
              <CardContent>
                <Skeleton className="h-8 w-12" />
              </CardContent>
            </Card>
          ))}
        </div>
        <Card>
          <CardHeader>
            <Skeleton className="h-4 w-20" />
          </CardHeader>
          <CardContent>
            <Skeleton className="h-32 w-full" />
          </CardContent>
        </Card>
      </div>
    )
  }

  // ── Error ──
  if (error) {
    return (
      <div className="flex flex-col gap-6" data-testid="referral-page">
        <div>
          <div className="flex items-center gap-2 mb-1">
            <Gift className="h-5 w-5 text-primary" />
            <h1 className="text-2xl font-semibold tracking-tight">推荐计划</h1>
          </div>
        </div>
        <Alert variant="destructive" data-testid="referral-error">
          <AlertTitle>加载失败</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-6">
      {/* Header */}
      <div>
        <div className="flex items-center gap-2 mb-1">
          <Gift className="h-5 w-5 text-primary" />
          <h1 className="text-2xl font-semibold tracking-tight">推荐计划</h1>
        </div>
        <p className="text-sm text-muted-foreground">
          每推荐一位成功付费用户，获得 ¥10 账户余额
        </p>
      </div>

      {/* Referral code card */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">你的推荐码</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <span
            className="font-mono text-3xl font-bold tracking-widest text-primary"
            data-testid="referral-code"
          >
            {codeData?.code ?? "—"}
          </span>
          <Button
            variant="outline"
            aria-label="复制推荐链接"
            onClick={handleCopy}
            data-testid="copy-button"
            disabled={!codeData}
          >
            {copied ? "已复制!" : "复制链接"}
          </Button>
        </CardContent>
      </Card>

      {/* Stats row */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              已推荐人数
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold" data-testid="total-referrals">
              {rewards.length}
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              待结算
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold" data-testid="total-pending">
              ¥{totalPending}
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              已结算
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold" data-testid="total-credited">
              ¥{totalCredited}
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Rewards table */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">奖励记录</CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          {rewards.length === 0 ? (
            <p className="px-6 py-4 text-sm text-muted-foreground" data-testid="no-rewards">
              暂无奖励记录
            </p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>被推荐用户</TableHead>
                  <TableHead>注册时间</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead className="text-right">金额</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {rewards.map((reward) => (
                  <TableRow key={reward.id}>
                    <TableCell className="font-mono text-sm">
                      {reward.referred_user_id}
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {new Date(reward.created_at).toLocaleDateString("zh-CN")}
                    </TableCell>
                    <TableCell>
                      <Badge
                        variant={reward.status === "credited" ? "success" : "default"}
                        data-testid={`status-badge-${reward.id}`}
                      >
                        {reward.status}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right font-medium">
                      ¥{reward.amount}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
