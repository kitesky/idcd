"use client"

import { useState, useMemo } from "react"
import dynamic from "next/dynamic"
import { Search, Globe, Wifi, WifiOff, Server } from "lucide-react"
import { useTranslations } from "next-intl"

const NodesWorldMap = dynamic(
  () => import("@/components/nodes/NodesWorldMap").then(m => m.NodesWorldMap),
  { ssr: false, loading: () => <div className="h-56 sm:h-72 bg-muted/30 rounded-md animate-pulse" /> }
)
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import {
  type NodeEntry,
  mapStatus,
  formatIP,
  aggregateStats,
  filterNodes,
} from "@/lib/nodes-utils"

const COUNTRY_NAMES: Record<string, string> = {
  CN: "中国大陆",
  HK: "香港",
  TW: "台湾",
  JP: "日本",
  SG: "新加坡",
  KR: "韩国",
  US: "美国",
  DE: "德国",
  GB: "英国",
  AU: "澳大利亚",
}

interface NodesClientProps {
  nodes: NodeEntry[]
}

export function NodesClient({ nodes }: NodesClientProps) {
  const t = useTranslations("nodes")
  const [countryFilter, setCountryFilter] = useState("all")
  const [carrierFilter, setCarrierFilter] = useState("all")
  const [statusFilter, setStatusFilter] = useState("all")
  const [search, setSearch] = useState("")

  const STATUS_OPTIONS = [
    { value: "all", label: t("filter.allStatus") },
    { value: "online", label: t("status.online") },
    { value: "degraded", label: t("status.degraded") },
    { value: "offline", label: t("status.offline") },
  ]

  const stats = useMemo(() => aggregateStats(nodes), [nodes])

  const countries = useMemo(
    () => Array.from(new Set(nodes.map((n) => n.country))).filter(Boolean).sort(),
    [nodes]
  )

  const carriers = useMemo(
    () => Array.from(new Set(nodes.map((n) => n.carrier))).filter(Boolean).sort(),
    [nodes]
  )

  const filtered = useMemo(
    () =>
      filterNodes(nodes, {
        country: countryFilter,
        carrier: carrierFilter,
        status: statusFilter,
        search,
      }),
    [nodes, countryFilter, carrierFilter, statusFilter, search]
  )

  return (
    <div className="space-y-6">
      {/* 统计卡片 */}
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
              <Server className="h-4 w-4" />
              {t("stats.total")}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-3xl font-bold tabular-nums">{stats.total}</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
              <Wifi className="h-4 w-4 text-success" />
              {t("stats.online")}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-3xl font-bold tabular-nums text-success">
              {stats.online}
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
              <Globe className="h-4 w-4" />
              {t("stats.countries")}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-3xl font-bold tabular-nums">{stats.countries}</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
              <WifiOff className="h-4 w-4" />
              {t("stats.carriers")}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-3xl font-bold tabular-nums">{stats.carriers}</p>
          </CardContent>
        </Card>
      </div>

      {/* 节点分布地图 */}
      <Card>
        <CardHeader className="pb-2">
          <div className="flex items-center justify-between">
            <CardTitle className="text-base">{t("map.title")}</CardTitle>
            <div className="flex items-center gap-4 text-xs text-muted-foreground">
              <span className="flex items-center gap-1.5">
                <span className="inline-block h-2 w-2 rounded-full bg-green-500" />
                {t("map.onlineNodes")}
              </span>
              <span className="flex items-center gap-1.5">
                <span className="inline-block h-2 w-2 rounded-full bg-slate-500" />
                {t("map.offlineNodes")}
              </span>
            </div>
          </div>
        </CardHeader>
        <CardContent className="p-0 pb-0">
          <NodesWorldMap nodes={nodes} />
        </CardContent>
      </Card>

      {/* 筛选栏 */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:gap-4">
        <div className="flex flex-1 flex-col gap-3 sm:flex-row sm:gap-3">
          <Select value={countryFilter} onValueChange={setCountryFilter}>
            <SelectTrigger className="w-full sm:w-44">
              <SelectValue placeholder={t("filter.selectCountry")} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem key="all" value="all">{t("filter.allCountries")}</SelectItem>
              {countries.map((code) => (
                <SelectItem key={code} value={code}>
                  {COUNTRY_NAMES[code] ?? code}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>

          <Select value={carrierFilter} onValueChange={setCarrierFilter}>
            <SelectTrigger className="w-full sm:w-44">
              <SelectValue placeholder={t("filter.selectCarrier")} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem key="all" value="all">{t("filter.allCarriers")}</SelectItem>
              {carriers.map((c) => (
                <SelectItem key={c} value={c}>
                  {c}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>

          <Select value={statusFilter} onValueChange={setStatusFilter}>
            <SelectTrigger className="w-full sm:w-36">
              <SelectValue placeholder={t("filter.selectStatus")} />
            </SelectTrigger>
            <SelectContent>
              {STATUS_OPTIONS.map((opt) => (
                <SelectItem key={opt.value} value={opt.value}>
                  {opt.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className="relative w-full sm:w-64">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            placeholder={t("filter.search")}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-9"
          />
        </div>
      </div>

      {/* 结果数量提示 */}
      <p className="text-sm text-muted-foreground">
        {t("table.showing", { count: filtered.length })}
        {filtered.length !== nodes.length && (
          <span>{t("table.showingOf", { total: nodes.length })}</span>
        )}
      </p>

      {/* 节点表格 */}
      <div className="overflow-x-auto rounded-lg border border-border">
        <Table className="w-full min-w-[640px]">
          <TableHeader>
            <TableRow>
              <TableHead className="whitespace-nowrap w-48">{t("table.nodeId")}</TableHead>
              <TableHead className="whitespace-nowrap w-28">{t("table.asn")}</TableHead>
              <TableHead className="whitespace-nowrap">{t("table.carrier")}</TableHead>
              <TableHead className="whitespace-nowrap">{t("table.region")}</TableHead>
              <TableHead className="whitespace-nowrap w-36">{t("table.exitIp")}</TableHead>
              <TableHead className="whitespace-nowrap w-24">{t("table.status")}</TableHead>
              <TableHead className="whitespace-nowrap w-28">{t("table.country")}</TableHead>
              <TableHead className="w-20" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {filtered.length === 0 ? (
              <TableRow>
                <TableCell colSpan={8} className="h-32 text-center text-muted-foreground">
                  {t("table.empty")}
                </TableCell>
              </TableRow>
            ) : (
              filtered.map((node) => {
                const { label, variant } = mapStatus(node.status)
                return (
                  <TableRow key={node.id}>
                    <TableCell className="font-mono text-xs">{node.id}</TableCell>
                    <TableCell className="font-mono text-xs text-muted-foreground">
                      {node.asn}
                    </TableCell>
                    <TableCell>{node.carrier}</TableCell>
                    <TableCell>{node.region}</TableCell>
                    <TableCell className="font-mono text-xs text-muted-foreground">
                      {formatIP(node.exitIp)}
                    </TableCell>
                    <TableCell>
                      <Badge variant={variant} className="whitespace-nowrap">{label}</Badge>
                    </TableCell>
                    <TableCell>
                      {COUNTRY_NAMES[node.country] ?? node.country}
                    </TableCell>
                    <TableCell>
                      <a
                        href={`/nodes/${node.id}`}
                        className="text-xs text-primary hover:underline whitespace-nowrap"
                      >
                        {t("table.viewDiag")}
                      </a>
                    </TableCell>
                  </TableRow>
                )
              })
            )}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}
