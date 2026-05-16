"use client"

import { useState, useMemo, useEffect } from "react"
import { Copy, Check, ChevronLeft, ChevronRight } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs"
import { cn } from "@/lib/utils"
import type { ProbeResult } from "@/lib/api"
import dynamic from "next/dynamic"
import type { MapNode } from "@/components/probe/ProbeMap"

const ProbeMap = dynamic(
  () => import("@/components/probe/ProbeMap").then(m => ({ default: m.ProbeMap })),
  { ssr: false, loading: () => <div className="w-full h-52 bg-muted/30 animate-pulse rounded" /> }
)

// ── Types ────────────────────────────────────────────────────────────────────

export interface ResultRow {
  node_id: string
  node_name: string
  success: boolean
  latency_ms?: number
  error?: string
  // HTTP timing breakdown (from details)
  status_code?: number
  redirect_ms?: number
  dns_ms?: number
  connect_ms?: number
  ssl_ms?: number
  ttfb_ms?: number
  download_ms?: number
  body_size?: number
  resolved_ip?: string
}

function parseRow(item: NonNullable<ProbeResult["results"]>[number]): ResultRow {
  const d = item.details ?? {}
  return {
    node_id:     item.node_id,
    node_name:   item.node_name,
    success:     item.success,
    latency_ms:  item.latency_ms,
    error:       item.error,
    status_code: d.status_code as number | undefined,
    redirect_ms: d.redirect_ms as number | undefined,
    dns_ms:      d.dns_ms as number | undefined,
    connect_ms:  d.connect_ms as number | undefined,
    ssl_ms:      d.ssl_ms as number | undefined,
    ttfb_ms:     d.ttfb_ms as number | undefined,
    download_ms: d.download_ms as number | undefined,
    body_size:   d.body_size as number | undefined,
    resolved_ip: (d.resolved_ip ?? d.ip) as string | undefined,
  }
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function fmtMs(v?: number) {
  return v !== undefined ? `${v.toFixed(0)}ms` : "-"
}

function fmtBytes(b?: number) {
  if (b === undefined) return "-"
  return b >= 1024 ? `${(b / 1024).toFixed(2)} KB` : `${b} B`
}

function latencyColor(ms?: number) {
  if (ms === undefined) return "text-muted-foreground"
  if (ms < 300) return "text-green-500"
  if (ms < 1000) return "text-yellow-500"
  return "text-destructive"
}

// ── CopyButton ────────────────────────────────────────────────────────────────

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false)
  const copy = () => {
    navigator.clipboard.writeText(text).catch(() => {})
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }
  return (
    <Button variant="ghost" size="sm" onClick={copy} className="h-auto p-0 text-sm text-primary hover:text-primary/80 hover:bg-transparent gap-1">
      {copied ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
      {copied ? "已复制" : "复制"}
    </Button>
  )
}


// ── StackedBar ────────────────────────────────────────────────────────────────

const HTTP_PHASES = [
  { key: "redirect_ms", label: "重定向", color: "#3b82f6" },
  { key: "dns_ms",      label: "解析",   color: "#6366f1" },
  { key: "connect_ms",  label: "建连",   color: "#f59e0b" },
  { key: "ssl_ms",      label: "SSL",    color: "#10b981" },
  { key: "ttfb_ms",     label: "首包",   color: "#22d3ee" },
  { key: "download_ms", label: "下载",   color: "#a3e635" },
] as const

function StackedBarChart({ rows }: { rows: ResultRow[] }) {
  const maxTotal = Math.max(...rows.map(r =>
    HTTP_PHASES.reduce((s, p) => s + (r[p.key] ?? 0), 0)
  ), 1)

  const BAR_HEIGHT = 180

  return (
    <div className="mt-6 rounded-lg border bg-background p-5">
      <div className="flex items-center justify-between mb-4">
        <p className="text-sm text-muted-foreground">
          检测目标: <span className="text-foreground font-medium">{rows[0]?.node_name ?? "–"}</span>
        </p>
        <div className="flex flex-wrap gap-3">
          {HTTP_PHASES.map(p => (
            <span key={p.key} className="flex items-center gap-1.5 text-xs text-muted-foreground">
              <span className="h-2.5 w-2.5 rounded-sm flex-shrink-0" style={{ background: p.color }} />
              {p.label}时间
            </span>
          ))}
        </div>
      </div>

      <div className="overflow-x-auto">
        <div className="flex items-end gap-6 min-w-0" style={{ minHeight: BAR_HEIGHT + 40 }}>
          {/* Y 轴刻度 */}
          <div className="flex flex-col justify-between text-right text-xs text-muted-foreground flex-shrink-0" style={{ height: BAR_HEIGHT }}>
            {[1, 0.75, 0.5, 0.25, 0].map(f => (
              <span key={f}>{Math.round(maxTotal * f)}</span>
            ))}
          </div>

          {rows.map(row => {
            const phases = HTTP_PHASES.map(p => ({ ...p, val: row[p.key] ?? 0 }))
            const total = phases.reduce((s, p) => s + p.val, 0)
            return (
              <div key={row.node_id} className="flex flex-col items-center gap-1 flex-1 min-w-[48px]">
                <div
                  className="flex flex-col-reverse w-full rounded-sm overflow-hidden"
                  style={{ height: `${(total / maxTotal) * BAR_HEIGHT}px` }}
                >
                  {phases.map(p => p.val > 0 && (
                    <div
                      key={p.key}
                      title={`${p.label}: ${p.val}ms`}
                      style={{
                        background: p.color,
                        height: `${(p.val / total) * 100}%`,
                      }}
                    />
                  ))}
                </div>
                <span className="text-[11px] text-muted-foreground text-center leading-tight mt-1">
                  {row.node_name}
                </span>
              </div>
            )
          })}
        </div>
      </div>
    </div>
  )
}

// ── SummaryTab ────────────────────────────────────────────────────────────────

function SummaryTab({ rows, isHttp, isChinaOnly }: { rows: ResultRow[]; isHttp: boolean; isChinaOnly: boolean }) {
  const sorted = [...rows].sort((a, b) => (a.latency_ms ?? Infinity) - (b.latency_ms ?? Infinity))
  const avg = rows.filter(r => r.latency_ms !== undefined).reduce((s, r, _, a) => s + r.latency_ms! / a.length, 0)

  // Build MapNode list for the map
  const mapNodes: MapNode[] = sorted.map(r => ({
    name: r.node_name,
    lat: 0,
    lng: 0,
    latency_ms: r.latency_ms,
  }))

  return (
    <div className="space-y-4">
      {/* 检测结果卡片 — 左地图 + 右排名 */}
      <div className="rounded-lg border bg-background overflow-hidden">
        <div className="flex items-center justify-between px-5 py-3 border-b">
          <span className="text-sm font-medium">检测结果</span>
          <div className="flex items-center gap-4 text-xs text-muted-foreground">
            <span>平均时间: <span className="text-foreground font-medium">{avg > 0 ? `${avg.toFixed(2)} ms` : "–"}</span></span>
            <span>检测节点数: <span className="text-foreground font-medium">{rows.length}</span></span>
          </div>
        </div>

        <div className="flex min-h-[320px]">
          {/* 左侧地图 */}
          <div className="w-[420px] flex-shrink-0 border-r p-4 flex flex-col gap-2">
            <ProbeMap nodes={mapNodes} isChinaOnly={isChinaOnly} />
          </div>

          {/* 右侧排名表 */}
          <div className="flex-1 overflow-auto">
            <table className="w-full text-sm">
              <thead className="sticky top-0">
                <tr className="border-b bg-muted/30">
                  <th className="text-left px-4 py-2.5 font-medium text-muted-foreground w-8">#</th>
                  <th className="text-left px-4 py-2.5 font-medium text-muted-foreground">节点</th>
                  <th className="text-left px-4 py-2.5 font-medium text-muted-foreground">平均时间</th>
                  <th className="text-left px-4 py-2.5 font-medium text-muted-foreground">结果</th>
                </tr>
              </thead>
              <tbody>
                {sorted.map((row, i) => (
                  <tr key={row.node_id} className="border-b hover:bg-muted/20 transition-colors">
                    <td className="px-4 py-3">
                      <span className={cn(
                        "inline-flex h-5 w-5 items-center justify-center rounded text-xs font-bold text-white",
                        i === 0 ? "bg-green-500" : i === 1 ? "bg-green-400" : i === 2 ? "bg-yellow-500" : "bg-muted text-muted-foreground"
                      )}>
                        {i + 1}
                      </span>
                    </td>
                    <td className="px-4 py-3 font-medium">{row.node_name}</td>
                    <td className={cn("px-4 py-3 font-medium tabular-nums", latencyColor(row.latency_ms))}>
                      {fmtMs(row.latency_ms)}
                    </td>
                    <td className="px-4 py-3">
                      <span className={cn(
                        "inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium",
                        row.success
                          ? "bg-green-500/10 text-green-600"
                          : "bg-destructive/10 text-destructive"
                      )}>
                        {row.success ? "成功" : (row.error ?? "失败")}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </div>

      {/* HTTP stacked bar chart */}
      {isHttp && rows.some(r => r.ttfb_ms !== undefined) && (
        <StackedBarChart rows={sorted} />
      )}
    </div>
  )
}

// ── DetailTab ─────────────────────────────────────────────────────────────────

function DetailTab({ rows, isHttp }: { rows: ResultRow[]; isHttp: boolean }) {
  const [page, setPage] = useState(1)
  const PAGE_SIZE = 10
  const pageCount = Math.ceil(rows.length / PAGE_SIZE)
  const paged = rows.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE)

  return (
    <div>
      <div className="flex items-center justify-end mb-3">
        <Button variant="outline" size="sm" className="h-7 text-xs px-3">导出报表</Button>
      </div>

      <div className="rounded-lg border bg-background overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-xs whitespace-nowrap">
            <thead>
              <tr className="border-b bg-muted/30">
                <th className="text-left px-4 py-3 font-medium text-muted-foreground">探测点</th>
                {isHttp && <th className="text-left px-4 py-3 font-medium text-muted-foreground">解析 IP</th>}
                <th className="text-left px-4 py-3 font-medium text-muted-foreground">状态</th>
                <th className="text-left px-4 py-3 font-medium text-muted-foreground">总响应</th>
                {isHttp && <>
                  <th className="text-left px-4 py-3 font-medium text-muted-foreground">重定向</th>
                  <th className="text-left px-4 py-3 font-medium text-muted-foreground">解析</th>
                  <th className="text-left px-4 py-3 font-medium text-muted-foreground">建连</th>
                  <th className="text-left px-4 py-3 font-medium text-muted-foreground">SSL</th>
                  <th className="text-left px-4 py-3 font-medium text-muted-foreground">首包</th>
                  <th className="text-left px-4 py-3 font-medium text-muted-foreground">下载</th>
                  <th className="text-left px-4 py-3 font-medium text-muted-foreground">大小</th>
                </>}
                <th className="text-left px-4 py-3 font-medium text-muted-foreground">操作</th>
              </tr>
            </thead>
            <tbody>
              {paged.map(row => (
                <tr key={row.node_id} className="border-b hover:bg-muted/20 transition-colors">
                  <td className="px-4 py-3 font-medium">{row.node_name}</td>
                  {isHttp && (
                    <td className="px-4 py-3 text-muted-foreground">
                      {row.resolved_ip ?? "-"}
                    </td>
                  )}
                  <td className="px-4 py-3">
                    {row.success ? (
                      <span className="text-green-600">{row.status_code ?? "200"}</span>
                    ) : (
                      <span className="text-destructive">失败</span>
                    )}
                  </td>
                  <td className={cn("px-4 py-3 font-medium", latencyColor(row.latency_ms))}>
                    {fmtMs(row.latency_ms)}
                  </td>
                  {isHttp && <>
                    <td className="px-4 py-3 text-muted-foreground">{fmtMs(row.redirect_ms)}</td>
                    <td className="px-4 py-3 text-muted-foreground">{fmtMs(row.dns_ms)}</td>
                    <td className="px-4 py-3 text-muted-foreground">{fmtMs(row.connect_ms)}</td>
                    <td className="px-4 py-3 text-muted-foreground">{fmtMs(row.ssl_ms)}</td>
                    <td className="px-4 py-3 text-muted-foreground">{fmtMs(row.ttfb_ms)}</td>
                    <td className="px-4 py-3 text-muted-foreground">{fmtMs(row.download_ms)}</td>
                    <td className="px-4 py-3 text-muted-foreground">{fmtBytes(row.body_size)}</td>
                  </>}
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2 text-primary text-xs">
                      <a href={`/tools/ping?q=${row.node_id}`} className="hover:underline">Ping</a>
                      <span className="opacity-30">|</span>
                      <a href={`/tools/dns?q=${row.node_id}`} className="hover:underline">DNS</a>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        {/* Pagination */}
        {pageCount > 1 && (
          <div className="flex items-center justify-center gap-3 py-3 border-t text-sm text-muted-foreground">
            <Button
              variant="ghost"
              size="icon"
              onClick={() => setPage(p => Math.max(1, p - 1))}
              disabled={page === 1}
              className="h-7 w-7"
            >
              <ChevronLeft className="h-4 w-4" />
            </Button>
            <span>{page}/{pageCount}</span>
            <Button
              variant="ghost"
              size="icon"
              onClick={() => setPage(p => Math.min(pageCount, p + 1))}
              disabled={page === pageCount}
              className="h-7 w-7"
            >
              <ChevronRight className="h-4 w-4" />
            </Button>
          </div>
        )}
      </div>
    </div>
  )
}

// ── ProbeResultPanel ──────────────────────────────────────────────────────────

interface ProbeResultPanelProps {
  result: ProbeResult
  target: string
  probeType?: string
  isLoading?: boolean
}

export function ProbeResultPanel({ result, target, probeType = "http", isLoading }: ProbeResultPanelProps) {
  // Client-only values to avoid hydration mismatch
  const [timestamp, setTimestamp] = useState("")
  const [shareUrl, setShareUrl] = useState("")
  useEffect(() => {
    setTimestamp(new Date().toLocaleString("zh-CN", { year: "numeric", month: "long", day: "numeric", hour: "2-digit", minute: "2-digit", second: "2-digit" }))
    setShareUrl(window.location.href)
  }, [])

  const rows = useMemo(
    () => (result.results ?? []).map(parseRow),
    [result.results]
  )

  const isHttp = probeType === "http"
  const isChinaOnly = rows.every(r => /[一-龥]/.test(r.node_name))
  const resolvedIps = [...new Set(rows.flatMap(r => r.resolved_ip ? [r.resolved_ip] : []))]

  return (
    <div className="mx-auto max-w-screen-xl px-6 py-6">
      {/* 检测结果 header */}
      <div className="mb-5">
        <h3 className="text-base font-semibold text-foreground mb-3">检测结果</h3>
        <div className="flex flex-wrap items-center gap-x-8 gap-y-1 text-sm text-muted-foreground">
          <span>
            <span className="mr-1.5">检测目标</span>
            <span className="text-foreground font-medium">{target}</span>
          </span>
          <span>
            <span className="mr-1.5">时间</span>
            <span className="text-foreground">{timestamp}</span>
          </span>
          {resolvedIps.length > 0 && (
            <span>
              <span className="mr-1.5">解析结果 IP</span>
              <span className="text-foreground">{resolvedIps.length} 个</span>
            </span>
          )}
          <div className="flex items-center gap-2 ml-auto">
            <span className="text-muted-foreground/60 text-xs truncate max-w-xs">{shareUrl}</span>
            <CopyButton text={shareUrl} />
          </div>
        </div>
      </div>

      {/* Loading skeleton */}
      {isLoading && rows.length === 0 && (
        <div className="space-y-3">
          {[1, 2, 3].map(i => (
            <div key={i} className="h-12 bg-muted/50 animate-pulse rounded-md" />
          ))}
          <p className="text-xs text-muted-foreground">等待节点返回结果...</p>
        </div>
      )}

      {/* Content */}
      {!isLoading && rows.length === 0 ? (
        <p className="text-sm text-muted-foreground">暂无结果</p>
      ) : rows.length > 0 ? (
        <Tabs defaultValue="summary" className="w-full">
          <TabsList className="mb-5">
            <TabsTrigger value="summary">结果概况</TabsTrigger>
            <TabsTrigger value="detail">详情结果</TabsTrigger>
          </TabsList>
          <TabsContent value="summary">
            <SummaryTab rows={rows} isHttp={isHttp} isChinaOnly={isChinaOnly} />
          </TabsContent>
          <TabsContent value="detail">
            <DetailTab rows={rows} isHttp={isHttp} />
          </TabsContent>
        </Tabs>
      ) : null}
    </div>
  )
}
