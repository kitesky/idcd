"use client"

import { useState } from "react"
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
import {
  CDN_DATA,
  REGION_LATENCY_DATA,
  ISP_AVAILABILITY_DATA,
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
  const t = T[lang]
  return (
    <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
      {REGION_LATENCY_DATA.map((region) => (
        <Card key={region.continent}>
          <CardHeader className="pb-3">
            <CardTitle className="text-base flex items-center gap-2">
              <span>{t.continentName(region.continent, region.continentEn)}</span>
            </CardTitle>
          </CardHeader>
          <CardContent className="pt-0">
            <div className="space-y-2">
              {region.countries.map((country) => (
                <div
                  key={country.nameEn}
                  className="flex items-center justify-between text-sm"
                >
                  <span className="text-muted-foreground truncate pr-2">
                    {t.countryName(country.name, country.nameEn)}
                  </span>
                  <div className="flex items-center gap-2 shrink-0">
                    <Badge
                      variant={getLatencyVariant(country.p50)}
                      className="text-xs"
                    >
                      P50 {country.p50}ms
                    </Badge>
                    <span className="text-xs text-muted-foreground w-16 text-right">
                      P95 {country.p95}ms
                    </span>
                  </div>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      ))}
    </div>
  )
}

function IspAvailabilityTable({ lang }: { lang: Lang }) {
  const t = T[lang]
  return (
    <div className="overflow-x-auto rounded-md border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className="w-12 text-center">{t.table.rank}</TableHead>
            <TableHead>{t.table.isp}</TableHead>
            <TableHead className="hidden md:table-cell">{t.table.region}</TableHead>
            <TableHead className="text-center">{t.table.availability30d}</TableHead>
            <TableHead className="hidden md:table-cell text-center">{t.table.sla}</TableHead>
            <TableHead className="hidden md:table-cell text-center">{t.table.datacenterCount}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {ISP_AVAILABILITY_DATA.map((isp) => (
            <TableRow key={isp.isp}>
              <TableCell className="text-center font-mono font-semibold text-muted-foreground">
                #{isp.rank}
              </TableCell>
              <TableCell className="font-medium">{isp.isp}</TableCell>
              <TableCell className="hidden md:table-cell text-muted-foreground">{isp.region}</TableCell>
              <TableCell className="text-center">
                <Badge
                  variant={
                    isp.availability30d >= 99.9
                      ? "success"
                      : isp.availability30d >= 99.5
                        ? "warning"
                        : "destructive"
                  }
                >
                  {isp.availability30d.toFixed(2)}%
                </Badge>
              </TableCell>
              <TableCell className="hidden md:table-cell text-center text-sm text-muted-foreground">
                {isp.sla.toFixed(1)}%
              </TableCell>
              <TableCell className="hidden md:table-cell text-center text-sm">
                {isp.datacenterCount}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

export function LeaderboardClient() {
  const [lang, setLang] = useState<Lang>('zh')
  const t = T[lang]
  const month = new Date().getMonth() + 1

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
            <span>{t.info.cdn(month)}</span>
          </div>
          <CdnTable data={CDN_DATA} lang={lang} />
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
