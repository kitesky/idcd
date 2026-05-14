"use client"

import { useState } from "react"
import { Gift } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"

// ─── Mock data ────────────────────────────────────────────────────────────────

const MOCK_CODE = "IDCD-XYZ789"

const MOCK_REWARDS = [
  {
    id: "rwd-001",
    referred_email: "alice@example.com",
    created_at: "2026-05-10T10:00:00Z",
    status: "credited",
    reward_amount: "10.00",
  },
  {
    id: "rwd-002",
    referred_email: "bob@corp.com",
    created_at: "2026-05-12T14:00:00Z",
    status: "pending",
    reward_amount: "10.00",
  },
  {
    id: "rwd-003",
    referred_email: "charlie@startup.io",
    created_at: "2026-05-14T09:00:00Z",
    status: "pending",
    reward_amount: "10.00",
  },
]

// ─── Derived stats ────────────────────────────────────────────────────────────

const totalPending = MOCK_REWARDS.filter((r) => r.status === "pending")
  .reduce((sum, r) => sum + parseFloat(r.reward_amount), 0)
  .toFixed(2)

const totalCredited = MOCK_REWARDS.filter((r) => r.status === "credited")
  .reduce((sum, r) => sum + parseFloat(r.reward_amount), 0)
  .toFixed(2)

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function ReferralPage() {
  const [copied, setCopied] = useState(false)

  function handleCopy() {
    const url = `https://idcd.com/?ref=${MOCK_CODE}`
    navigator.clipboard.writeText(url).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    })
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
            {MOCK_CODE}
          </span>
          <Button
            variant="outline"
            aria-label="复制推荐链接"
            onClick={handleCopy}
            data-testid="copy-button"
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
            <p className="text-2xl font-bold">{MOCK_REWARDS.length}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              待结算
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold">¥{totalPending}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              已结算
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold">¥{totalCredited}</p>
          </CardContent>
        </Card>
      </div>

      {/* Rewards table */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">奖励记录</CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>被推荐邮箱</TableHead>
                <TableHead>注册时间</TableHead>
                <TableHead>状态</TableHead>
                <TableHead className="text-right">金额</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {MOCK_REWARDS.map((reward) => (
                <TableRow key={reward.id}>
                  <TableCell className="font-mono text-sm">
                    {reward.referred_email}
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {new Date(reward.created_at).toLocaleDateString("zh-CN")}
                  </TableCell>
                  <TableCell>
                    <Badge
                      variant={
                        reward.status === "credited" ? "success" : "default"
                      }
                      data-testid={`status-badge-${reward.id}`}
                    >
                      {reward.status}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-right font-medium">
                    ¥{reward.reward_amount}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  )
}
