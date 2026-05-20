"use client"

import { useState, useEffect, useMemo } from "react"
import { ArrowDown, ArrowUp, Minus, Info, Globe } from "lucide-react"
import { useTranslations } from "next-intl"
import {
  Tabs,
  TabsList,
  TabsTrigger,
  TabsContent,
} from "@/components/ui/tabs"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Badge } from "@/components/ui/badge"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Skeleton } from "@/components/ui/skeleton"
import { apiRequest } from "@/lib/api"
import {
  getLatencyVariant,
  type CdnEntry,
} from "./leaderboard-data"

type T = ReturnType<typeof useTranslations<"leaderboard">>

function Sparkline({ values }: { values: number[] }) {
  if (!values?.length) {
    return (
      <div className="flex h-6 w-14 items-end gap-px opacity-20">
        {Array.from({ length: 7 }).map((_, i) => (
          <div key={i} className="flex-1 rounded-sm bg-muted-foreground/40" style={{ height: "50%" }} />
        ))}
      </div>
    )
  }
  const max = Math.max(...values)
  const min = Math.min(...values)
  const range = max - min || 1

  return (
    <div
      className="flex items-end gap-px h-6 w-14"
      aria-label="latency trend"
      role="img"
    >
      {values.map((v, i) => {
        const heightPct = ((max - v) / range) * 70 + 30
        const isLast = i === values.length - 1
        return (
          <div
            key={i}
            className={`flex-1 rounded-sm ${isLast ? "bg-primary" : "bg-muted-foreground/40"}`}
            style={{ height: `${heightPct}%` }}
          />
        )
      })}
    </div>
  )
}

function ChangeIndicator({ change, flat }: { change: number; flat: string }) {
  if (change < 0) {
    return (
      <span className="flex items-center gap-0.5 text-xs text-green-500 font-medium">
        <ArrowDown className="h-3 w-3" />
        {Math.abs(change)}ms
      </span>
    )
  }
  if (change > 0) {
    return (
      <span className="flex items-center gap-0.5 text-xs text-red-500 font-medium">
        <ArrowUp className="h-3 w-3" />
        {change}ms
      </span>
    )
  }
  return (
    <span className="flex items-center gap-0.5 text-xs text-muted-foreground">
      <Minus className="h-3 w-3" />
      {flat}
    </span>
  )
}

function LatencyBadge({ ms }: { ms: number }) {
  if (!ms) return <span className="text-xs text-muted-foreground">—</span>
  const variant = getLatencyVariant(ms)
  return <Badge variant={variant}>{ms}ms</Badge>
}

function CdnTable({ data, t }: { data: CdnEntry[]; t: T }) {
  const sorted = [...data].sort((a, b) => a.globalP50 - b.globalP50)

  return (
    <div className="overflow-x-auto rounded-md border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className="w-12 text-center">{t("rankHeader")}</TableHead>
            <TableHead>{t("table.provider")}</TableHead>
            <TableHead className="text-center">{t("table.globalP50")}</TableHead>
            <TableHead className="hidden md:table-cell text-center">{t("table.chinaP50")}</TableHead>
            <TableHead className="hidden md:table-cell text-center">{t("table.overseasP50")}</TableHead>
            <TableHead className="hidden md:table-cell text-center w-20">{t("table.trend")}</TableHead>
            <TableHead className="text-center">{t("table.change")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {sorted.map((cdn) => (
            <TableRow key={cdn.name}>
              <TableCell className="text-center font-mono font-semibold text-muted-foreground">
                #{cdn.rank}
              </TableCell>
              <TableCell className="font-medium">{cdn.name}</TableCell>
              <TableCell className="text-center">
                <LatencyBadge ms={cdn.globalP50} />
              </TableCell>
              <TableCell className="hidden md:table-cell text-center">
                <LatencyBadge ms={cdn.chinaP50} />
              </TableCell>
              <TableCell className="hidden md:table-cell text-center">
                <LatencyBadge ms={cdn.overseasP50} />
              </TableCell>
              <TableCell className="hidden md:table-cell">
                <div className="flex justify-center items-center py-3">
                  <Sparkline values={cdn.trend} />
                </div>
              </TableCell>
              <TableCell className="text-center">
                <ChangeIndicator change={cdn.change} flat={t("flat")} />
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

function EmptyState({ title, description }: { title: string; description: string }) {
  return (
    <div className="text-center py-16 text-muted-foreground">
      <Globe className="mx-auto mb-4 h-12 w-12 opacity-30" />
      <p className="text-lg font-medium">{title}</p>
      <p className="text-sm mt-1">{description}</p>
    </div>
  )
}

function getSelectedMonthDefault(): string {
  const now = new Date()
  const year = now.getFullYear()
  const month = String(now.getMonth() + 1).padStart(2, '0')
  return `${year}-${month}`
}

export function LeaderboardClient() {
  const t = useTranslations("leaderboard")
  const selectedMonth = useMemo(() => getSelectedMonthDefault(), [])
  const [data, setData] = useState<CdnEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [sampleCount, setSampleCount] = useState(0)
  const monthNumber = parseInt(selectedMonth.split('-')[1] ?? '1', 10)

  useEffect(() => {
    type ApiEntry = {
      rank: number; name: string; target: string
      avg_latency_ms: number; p50_latency_ms: number; p95_latency_ms: number
      uptime_pct: number; check_count: number
    }
    // eslint-disable-next-line react-hooks/set-state-in-effect -- fetch 触发的初始 loading 同步设置是标准模式
    setLoading(true)
    setError(null)
    const errMsg = t("loadFailedDesc")
    apiRequest<{ data: { entries: ApiEntry[]; total: number } }>(
      `/v1/leaderboard/cdn?month=${selectedMonth}`
    )
      .then((json) => {
        const raw = json.data?.entries ?? []
        setData(raw.map(e => ({
          rank: e.rank,
          name: e.name,
          shortName: e.name.replace(/ CDN$/, ''),
          globalP50: e.p50_latency_ms,
          chinaP50: e.p50_latency_ms,
          overseasP50: e.p50_latency_ms,
          trend: [],
          change: 0,
        })))
        setSampleCount(json.data?.total ?? 0)
      })
      .catch(() => setError(errMsg))
      .finally(() => setLoading(false))
  }, [selectedMonth, t])

  function renderCdnContent() {
    if (loading) {
      return (
        <div className="space-y-2">
          {Array.from({ length: 5 }).map((_, i) => (
            <Skeleton key={i} className="h-12 w-full" />
          ))}
        </div>
      )
    }
    if (error) {
      return (
        <Alert variant="destructive">
          <Info className="h-4 w-4" />
          <AlertTitle>{t("loadFailedTitle")}</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )
    }
    if (data.length === 0) {
      return <EmptyState title={t("emptyTitle")} description={t("emptyCdn")} />
    }
    return <CdnTable data={data} t={t} />
  }

  return (
    <div className="space-y-6">
      <Tabs defaultValue="cdn" className="w-full">
        <TabsList className="mb-4">
          <TabsTrigger value="cdn">{t("tabs.cdn")}</TabsTrigger>
          <TabsTrigger value="regions">{t("tabs.regions")}</TabsTrigger>
          <TabsTrigger value="availability">{t("tabs.availability")}</TabsTrigger>
        </TabsList>

        <TabsContent value="cdn" className="space-y-4">
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <Info className="h-4 w-4 shrink-0" />
            <span>{t("info.cdn", { month: monthNumber })}</span>
          </div>
          {!loading && !error && data.length > 0 && sampleCount > 0 && (
            <p className="text-xs text-muted-foreground">
              {t("sampleCount", { count: sampleCount.toLocaleString() })}
            </p>
          )}
          {renderCdnContent()}
        </TabsContent>

        <TabsContent value="regions" className="space-y-4">
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <Info className="h-4 w-4 shrink-0" />
            <span>{t("info.regions")}</span>
          </div>
          <EmptyState title={t("emptyTitle")} description={t("emptyRegions")} />
        </TabsContent>

        <TabsContent value="availability" className="space-y-4">
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <Info className="h-4 w-4 shrink-0" />
            <span>{t("info.availability")}</span>
          </div>
          <EmptyState title={t("emptyTitle")} description={t("emptyAvailability")} />
        </TabsContent>
      </Tabs>

      <div className="space-y-4 pt-4 border-t">
        <p className="text-sm text-muted-foreground">
          <strong>{t("disclaimer.methodologyLabel")}</strong>
          {t("disclaimer.methodology")}
        </p>
        <Alert>
          <Info className="h-4 w-4" />
          <AlertTitle>{t("disclaimer.title")}</AlertTitle>
          <AlertDescription>
            {t("disclaimer.notice")}
          </AlertDescription>
        </Alert>
      </div>
    </div>
  )
}
