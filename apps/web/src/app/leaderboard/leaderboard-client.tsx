"use client"

import { ArrowDown, ArrowUp, Minus, Info } from "lucide-react"
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
import {
  CDN_DATA,
  REGION_LATENCY_DATA,
  ISP_AVAILABILITY_DATA,
  getLatencyVariant,
  type CdnEntry,
} from "./leaderboard-data"

// Simple CSS sparkline using inline bar chart
function Sparkline({ values }: { values: number[] }) {
  const max = Math.max(...values)
  const min = Math.min(...values)
  const range = max - min || 1

  return (
    <div
      className="flex items-end gap-px h-6 w-14"
      aria-label="延迟趋势图"
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

function ChangeIndicator({ change }: { change: number }) {
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
      持平
    </span>
  )
}

function LatencyBadge({ ms }: { ms: number }) {
  const variant = getLatencyVariant(ms)
  return <Badge variant={variant}>{ms}ms</Badge>
}

function CdnTable({ data }: { data: CdnEntry[] }) {
  const sorted = [...data].sort((a, b) => a.globalP50 - b.globalP50)

  return (
    <div className="rounded-md border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className="w-12 text-center">排名</TableHead>
            <TableHead>CDN 名称</TableHead>
            <TableHead className="text-center">全球 P50</TableHead>
            <TableHead className="text-center">中国大陆 P50</TableHead>
            <TableHead className="text-center">海外 P50</TableHead>
            <TableHead className="text-center w-20">24h 趋势</TableHead>
            <TableHead className="text-center">变化</TableHead>
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
              <TableCell className="text-center">
                <LatencyBadge ms={cdn.chinaP50} />
              </TableCell>
              <TableCell className="text-center">
                <LatencyBadge ms={cdn.overseasP50} />
              </TableCell>
              <TableCell className="flex justify-center items-center py-3">
                <Sparkline values={cdn.trend} />
              </TableCell>
              <TableCell className="text-center">
                <ChangeIndicator change={cdn.change} />
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

function RegionLatencyCards() {
  return (
    <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
      {REGION_LATENCY_DATA.map((region) => (
        <Card key={region.continent}>
          <CardHeader className="pb-3">
            <CardTitle className="text-base flex items-center gap-2">
              <span>{region.continent}</span>
              <span className="text-xs font-normal text-muted-foreground">
                {region.continentEn}
              </span>
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
                    {country.name}
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

function IspAvailabilityTable() {
  return (
    <div className="rounded-md border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className="w-12 text-center">排名</TableHead>
            <TableHead>运营商 / ISP</TableHead>
            <TableHead>地区</TableHead>
            <TableHead className="text-center">30日可用率</TableHead>
            <TableHead className="text-center">SLA 承诺</TableHead>
            <TableHead className="text-center">机房数</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {ISP_AVAILABILITY_DATA.map((isp) => (
            <TableRow key={isp.isp}>
              <TableCell className="text-center font-mono font-semibold text-muted-foreground">
                #{isp.rank}
              </TableCell>
              <TableCell className="font-medium">{isp.isp}</TableCell>
              <TableCell className="text-muted-foreground">{isp.region}</TableCell>
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
              <TableCell className="text-center text-sm text-muted-foreground">
                {isp.sla.toFixed(1)}%
              </TableCell>
              <TableCell className="text-center text-sm">
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
  return (
    <div className="space-y-6">
      <Tabs defaultValue="cdn" className="w-full">
        <TabsList className="mb-4">
          <TabsTrigger value="cdn">CDN 响应速度</TabsTrigger>
          <TabsTrigger value="regions">全球节点延迟</TabsTrigger>
          <TabsTrigger value="availability">可用性统计</TabsTrigger>
        </TabsList>

        <TabsContent value="cdn" className="space-y-4">
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <Info className="h-4 w-4 shrink-0" />
            <span>
              数据采集自 {new Date().getMonth() + 1} 月全月探测任务，按全球 P50 升序排列。P50
              为中位响应时间，越低越好。
            </span>
          </div>
          <CdnTable data={CDN_DATA} />
        </TabsContent>

        <TabsContent value="regions" className="space-y-4">
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <Info className="h-4 w-4 shrink-0" />
            <span>
              各大陆 Top 5 国家/地区平均延迟，来自分布式真实节点探测数据，每月更新。
            </span>
          </div>
          <RegionLatencyCards />
        </TabsContent>

        <TabsContent value="availability" className="space-y-4">
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <Info className="h-4 w-4 shrink-0" />
            <span>
              近 30 天各 ISP / 运营商可用性统计，基于每 5 分钟一次的健康检测。
            </span>
          </div>
          <IspAvailabilityTable />
        </TabsContent>
      </Tabs>

      {/* Bottom disclaimer */}
      <div className="space-y-4 pt-4 border-t">
        <p className="text-sm text-muted-foreground">
          <strong>数据采集说明：</strong>
          所有延迟数据均来自 idcd 全球真实探测节点，采用 HTTP GET 方式对各 CDN 标准测速端点发起请求，
          记录从 TCP 握手到首字节时间（TTFB）。每个节点每 5 分钟探测一次，月度数据取 P50（中位数），
          以消除网络抖动影响。可用性数据基于各运营商 DNS 解析成功率与 TCP 连通率综合计算。
        </p>
        <Alert>
          <Info className="h-4 w-4" />
          <AlertTitle>数据声明</AlertTitle>
          <AlertDescription>
            本排行榜数据来自 idcd
            真实分布式探测节点，仅代表探测节点到各 CDN 接入点的网络性能，
            不构成商业推荐，亦不代表与任何 CDN 厂商存在商业合作关系。
            各厂商在不同地区、不同业务场景下的实际表现可能存在差异。
          </AlertDescription>
        </Alert>
      </div>
    </div>
  )
}
