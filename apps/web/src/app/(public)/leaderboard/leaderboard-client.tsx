"use client"

import { useState, useEffect, useRef } from "react"
import { ArrowDown, ArrowUp, Minus, Info, Globe } from "lucide-react"
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
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { apiRequest } from "@/lib/api"
import {
  getLatencyVariant,
  type CdnEntry,
} from "./leaderboard-data"

type Lang = 'zh' | 'en'

const T = {
  zh: {
    tabs: {
      cdn: 'CDN 响应速度',
      regions: '全球节点延迟',
      availability: '可用性统计',
    },
    table: {
      rank: '排名',
      provider: 'CDN 名称',
      globalP50: '全球 P50',
      chinaP50: '中国大陆 P50',
      overseasP50: '海外 P50',
      trend: '24h 趋势',
      change: '变化',
      isp: '运营商 / ISP',
      region: '地区',
      availability30d: '30日可用率',
      sla: 'SLA 承诺',
      datacenterCount: '机房数',
    },
    info: {
      cdn: (month: number) => `数据采集自 ${month} 月全月探测任务，按全球 P50 升序排列。P50 为中位响应时间，越低越好。`,
      regions: '各大陆 Top 5 国家/地区平均延迟，来自分布式真实节点探测数据，每月更新。',
      availability: '近 30 天各 ISP / 运营商可用性统计，基于每 5 分钟一次的健康检测。',
    },
    flat: '持平',
    disclaimerTitle: '数据声明',
    disclaimerMethodology: '所有延迟数据均来自 idcd 全球真实探测节点，采用 HTTP GET 方式对各 CDN 标准测速端点发起请求，记录从 TCP 握手到首字节时间（TTFB）。每个节点每 5 分钟探测一次，月度数据取 P50（中位数），以消除网络抖动影响。可用性数据基于各运营商 DNS 解析成功率与 TCP 连通率综合计算。',
    disclaimerNotice: '本排行榜数据来自 idcd 真实分布式探测节点，仅代表探测节点到各 CDN 接入点的网络性能，不构成商业推荐，亦不代表与任何 CDN 厂商存在商业合作关系。各厂商在不同地区、不同业务场景下的实际表现可能存在差异。',
    countryName: (name: string, _nameEn: string) => name,
    continentName: (zh: string, _en: string) => zh,
  },
  en: {
    tabs: {
      cdn: 'CDN Response Speed',
      regions: 'Global Node Latency',
      availability: 'Availability Stats',
    },
    table: {
      rank: 'Rank',
      provider: 'Provider',
      globalP50: 'Global P50',
      chinaP50: 'China P50',
      overseasP50: 'Overseas P50',
      trend: '24h Trend',
      change: 'Change',
      isp: 'ISP / Carrier',
      region: 'Region',
      availability30d: '30-Day Uptime',
      sla: 'SLA Target',
      datacenterCount: 'PoPs',
    },
    info: {
      cdn: (month: number) => `Data collected across all probes in month ${month}, sorted by global P50 ascending. P50 is the median response time — lower is better.`,
      regions: 'Average latency for top 5 countries per continent, sourced from distributed real-node probe data, updated monthly.',
      availability: '30-day availability statistics per ISP/carrier, based on health checks running every 5 minutes.',
    },
    flat: 'Unchanged',
    disclaimerTitle: 'Data Disclosure',
    disclaimerMethodology: "All latency data is collected from idcd's global real probe nodes using HTTP GET requests to each CDN's standard benchmark endpoint, measuring TCP-handshake-to-first-byte time (TTFB). Each node probes every 5 minutes; monthly figures use P50 (median) to eliminate jitter. Availability is derived from DNS resolution success rate and TCP reachability.",
    disclaimerNotice: 'This leaderboard reflects network performance measured from idcd probe nodes to CDN points of presence. It does not constitute a commercial recommendation nor imply any partnership with CDN vendors. Actual performance may vary by region and workload.',
    countryName: (_name: string, nameEn: string) => nameEn,
    continentName: (_zh: string, en: string) => en,
  },
}

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

function CdnTable({ data, lang }: { data: CdnEntry[]; lang: Lang }) {
  const sorted = [...data].sort((a, b) => a.globalP50 - b.globalP50)
  const t = T[lang]

  return (
    <div className="overflow-x-auto rounded-md border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className="w-12 text-center">{t.table.rank}</TableHead>
            <TableHead>{t.table.provider}</TableHead>
            <TableHead className="text-center">{t.table.globalP50}</TableHead>
            <TableHead className="hidden md:table-cell text-center">{t.table.chinaP50}</TableHead>
            <TableHead className="hidden md:table-cell text-center">{t.table.overseasP50}</TableHead>
            <TableHead className="hidden md:table-cell text-center w-20">{t.table.trend}</TableHead>
            <TableHead className="text-center">{t.table.change}</TableHead>
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
                <ChangeIndicator change={cdn.change} flat={t.flat} />
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

function RegionLatencyCards({ lang }: { lang: Lang }) {
  return (
    <div className="text-center py-16 text-muted-foreground">
      <Globe className="mx-auto mb-4 h-12 w-12 opacity-30" />
      <p className="text-lg font-medium">{lang === 'zh' ? '本月数据采集中' : 'Data collection in progress'}</p>
      <p className="text-sm mt-1">
        {lang === 'zh'
          ? '全球节点延迟数据将在积累足够探测记录后显示，预计 30 天内上线'
          : 'Global node latency data will be displayed once enough probe records are collected, expected within 30 days'}
      </p>
    </div>
  )
}

function IspAvailabilityTable({ lang }: { lang: Lang }) {
  return (
    <div className="text-center py-16 text-muted-foreground">
      <Globe className="mx-auto mb-4 h-12 w-12 opacity-30" />
      <p className="text-lg font-medium">{lang === 'zh' ? '本月数据采集中' : 'Data collection in progress'}</p>
      <p className="text-sm mt-1">
        {lang === 'zh'
          ? '可用性统计数据将在积累足够探测记录后显示，预计 30 天内上线'
          : 'Availability statistics will be displayed once enough probe records are collected, expected within 30 days'}
      </p>
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
  const [lang, setLang] = useState<Lang>('zh')
  const langRef = useRef<Lang>(lang)
  langRef.current = lang
  const [selectedMonth, setSelectedMonth] = useState<string>(getSelectedMonthDefault)
  const [data, setData] = useState<CdnEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [sampleCount, setSampleCount] = useState(0)
  const t = T[lang]
  const monthNumber = parseInt(selectedMonth.split('-')[1] ?? '1', 10)

  useEffect(() => {
    setLoading(true)
    setError(null)
    type ApiEntry = {
      rank: number; name: string; target: string
      avg_latency_ms: number; p50_latency_ms: number; p95_latency_ms: number
      uptime_pct: number; check_count: number
    }
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
      .catch(() => setError(langRef.current === 'zh' ? '数据加载失败，请稍后重试' : 'Failed to load data, please try again later'))
      .finally(() => setLoading(false))
  }, [selectedMonth])

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
          <AlertTitle>{lang === 'zh' ? '加载失败' : 'Load Failed'}</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )
    }
    if (data.length === 0) {
      return (
        <div className="text-center py-16 text-muted-foreground">
          <Globe className="mx-auto mb-4 h-12 w-12 opacity-30" />
          <p className="text-lg font-medium">{lang === 'zh' ? '本月数据采集中' : 'Data collection in progress'}</p>
          <p className="text-sm mt-1">
            {lang === 'zh'
              ? 'CDN 排行榜将在积累足够节点数据后显示，预计 30 天内上线'
              : 'CDN leaderboard will be displayed once enough node data is collected, expected within 30 days'}
          </p>
        </div>
      )
    }
    return <CdnTable data={data} lang={lang} />
  }

  return (
    <div className="space-y-6">
      <div className="flex justify-end">
        <div className="flex items-center gap-1 rounded-md border p-0.5">
          <Button
            variant={lang === 'zh' ? 'secondary' : 'ghost'}
            size="sm"
            className="h-7 px-2 text-xs"
            onClick={() => setLang('zh')}
            aria-label="切换为中文"
          >
            中文
          </Button>
          <Button
            variant={lang === 'en' ? 'secondary' : 'ghost'}
            size="sm"
            className="h-7 px-2 text-xs"
            onClick={() => setLang('en')}
            aria-label="Switch to English"
          >
            <Globe className="h-3 w-3 mr-1" />
            EN
          </Button>
        </div>
      </div>

      <Tabs defaultValue="cdn" className="w-full">
        <TabsList className="mb-4">
          <TabsTrigger value="cdn">{t.tabs.cdn}</TabsTrigger>
          <TabsTrigger value="regions">{t.tabs.regions}</TabsTrigger>
          <TabsTrigger value="availability">{t.tabs.availability}</TabsTrigger>
        </TabsList>

        <TabsContent value="cdn" className="space-y-4">
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <Info className="h-4 w-4 shrink-0" />
            <span>{t.info.cdn(monthNumber)}</span>
          </div>
          {!loading && !error && data.length > 0 && sampleCount > 0 && (
            <p className="text-xs text-muted-foreground">
              {lang === 'zh' ? `本月采样数：${sampleCount.toLocaleString()}` : `Sample count: ${sampleCount.toLocaleString()}`}
            </p>
          )}
          {renderCdnContent()}
        </TabsContent>

        <TabsContent value="regions" className="space-y-4">
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <Info className="h-4 w-4 shrink-0" />
            <span>{t.info.regions}</span>
          </div>
          <RegionLatencyCards lang={lang} />
        </TabsContent>

        <TabsContent value="availability" className="space-y-4">
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <Info className="h-4 w-4 shrink-0" />
            <span>{t.info.availability}</span>
          </div>
          <IspAvailabilityTable lang={lang} />
        </TabsContent>
      </Tabs>

      <div className="space-y-4 pt-4 border-t">
        <p className="text-sm text-muted-foreground">
          <strong>{lang === 'zh' ? '数据采集说明：' : 'Methodology: '}</strong>
          {t.disclaimerMethodology}
        </p>
        <Alert>
          <Info className="h-4 w-4" />
          <AlertTitle>{t.disclaimerTitle}</AlertTitle>
          <AlertDescription>
            {t.disclaimerNotice}
          </AlertDescription>
        </Alert>
      </div>
    </div>
  )
}
