"use client"

import { useEffect, useState } from "react"
import { Gift } from "lucide-react"
import { useTranslations, useLocale } from "next-intl"
import { bcp47Of } from "@/i18n/registry"
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
  uses_count: number
  url: string
}

interface Reward {
  id: string
  referred_user_id: string
  status: "pending" | "credited"
  amount: string
  currency: string
  created_at: string
}

type ReferralT = ReturnType<typeof useTranslations<"billing.referral">>

// ─── Helpers ──────────────────────────────────────────────────────────────────

function statusBadge(status: Reward["status"], id: string, t: ReferralT) {
  if (status === "credited") {
    return (
      <Badge variant="success" data-testid={`status-badge-${id}`}>
        {t("status.credited")}
      </Badge>
    )
  }
  return (
    <Badge variant="default" data-testid={`status-badge-${id}`}>
      {t("status.pending")}
    </Badge>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function ReferralPage() {
  const t = useTranslations("billing.referral")
  const locale = useLocale()
  const [codeData, setCodeData] = useState<ReferralCode | null>(null)
  const [rewards, setRewards] = useState<Reward[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    async function fetchAll() {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- 进入页面时重置 loading，随后异步 fetch
      setLoading(true)
      setError(null)
      try {
        const [codeRes, rewardsRes] = await Promise.all([
          apiRequest<{ data: ReferralCode }>("/v1/referral/code", { method: "POST" }),
          apiRequest<{ data: { rewards: Reward[] } }>("/v1/referral/rewards"),
        ])
        setCodeData(codeRes.data)
        setRewards(rewardsRes.data.rewards)
      } catch (err: unknown) {
        setError(
          err instanceof Error ? err.message : t("loadFailed")
        )
      } finally {
        setLoading(false)
      }
    }

    fetchAll()
  // eslint-disable-next-line react-hooks/exhaustive-deps -- t 来自 i18n hook，引用稳定但 lint 不识别
  }, [])

  function handleCopy() {
    if (!codeData) return
    navigator.clipboard.writeText(codeData.url).then(() => {
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
            <h1 className="text-2xl font-semibold tracking-tight">{t("programTitle")}</h1>
          </div>
          <p className="text-sm text-muted-foreground">{t("programDesc")}</p>
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
            <h1 className="text-2xl font-semibold tracking-tight">{t("programTitle")}</h1>
          </div>
        </div>
        <Alert variant="destructive" data-testid="referral-error">
          <AlertTitle>{t("loadError")}</AlertTitle>
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
          <h1 className="text-2xl font-semibold tracking-tight">{t("programTitle")}</h1>
        </div>
        <p className="text-sm text-muted-foreground">{t("programDesc")}</p>
      </div>

      {/* Referral code card */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("yourCode")}</CardTitle>
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
            aria-label={t("copyAriaLabel")}
            onClick={handleCopy}
            data-testid="copy-button"
            disabled={!codeData}
          >
            {copied ? t("copied") : t("copy")}
          </Button>
        </CardContent>
      </Card>

      {/* Stats row */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              {t("totalReferrals")}
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
              {t("pending")}
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
              {t("credited")}
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
          <CardTitle className="text-base">{t("rewardHistory")}</CardTitle>
        </CardHeader>
        <CardContent className="p-0 overflow-x-auto">
          {rewards.length === 0 ? (
            <p className="px-6 py-4 text-sm text-muted-foreground" data-testid="no-rewards">
              {t("noRewards")}
            </p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("referredUser")}</TableHead>
                  <TableHead>{t("registeredAt")}</TableHead>
                  <TableHead>{t("statusLabel")}</TableHead>
                  <TableHead className="text-right">{t("amount")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {rewards.map((reward) => (
                  <TableRow key={reward.id}>
                    <TableCell className="font-mono text-sm">
                      {reward.referred_user_id}
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {new Date(reward.created_at).toLocaleDateString(bcp47Of(locale))}
                    </TableCell>
                    <TableCell>{statusBadge(reward.status, reward.id, t)}</TableCell>
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
