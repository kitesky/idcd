"use client"

import { useEffect, useMemo, useState } from "react"
import dynamic from "next/dynamic"
import { Loader2Icon, RadioTowerIcon } from "lucide-react"
import { Card, CardContent, CardHeader, CardTitle, Badge } from "@/components/ui"
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { getNodes, type Node, type ProbeTaskResult, type TracerouteHop } from "@/lib/api"
import type { TraceOrigin } from "./TracerouteMap"

// TracerouteMap pulls in d3-geo + topojson lazily. ssr:false dodges the async
// fetch("/world-110m.json") inside the component during hydration.
const TracerouteMap = dynamic(
  () => import("./TracerouteMap").then(m => ({ default: m.TracerouteMap })),
  { ssr: false, loading: () => <div className="w-full h-80 bg-muted/30 animate-pulse rounded-lg" /> }
)

// Module-level nodes cache — shared across panels so we don't stampede /v1/nodes.
let nodesCache: Node[] | null = null
let nodesPromise: Promise<Node[]> | null = null
function fetchNodesOnce(): Promise<Node[]> {
  if (nodesCache) return Promise.resolve(nodesCache)
  if (!nodesPromise) {
    nodesPromise = getNodes()
      .then((n) => { nodesCache = n; return n })
      .catch((err) => { nodesPromise = null; throw err })
  }
  return nodesPromise
}

// ── Types ─────────────────────────────────────────────────────────────────────

/** Polling state shape produced by useProbePolling — kept here so callers can
 *  lift the hook to the page (so submit button can react to isPolling). */
type PollingState = {
  taskResult: ProbeTaskResult | null
  isPolling: boolean
  error: string
}

interface TracerouteResultPanelProps {
  polling: PollingState
  /** When provided, used to look up the source node's lat/lng for the map
   *  path origin. Falls back to a single-task lookup if absent. */
  sourceNodeId?: string
  /** "traceroute" or "mtr" — drives which fields to normalize from result.data. */
  variant?: "traceroute" | "mtr"
  /** Right-side header slot — used to host ShareResultButton from the parent. */
  headerSlot?: React.ReactNode
  /** External submit error to surface in the empty / error state. */
  error?: string
}

// ── Normalization (traceroute hops / mtr hops → shared TracerouteHop shape) ──

interface MTRHopRaw {
  hop: number
  ip?: string
  hostname?: string
  avg_rtt_ms?: number
  loss_pct?: number
  timeout?: boolean
  country?: string
  city?: string
  lat?: number
  lng?: number
}

function extractHops(taskResult: ProbeTaskResult | null, variant: "traceroute" | "mtr"): {
  hops: TracerouteHop[]
  mtrHops: MTRHopRaw[]
  reached: boolean
} {
  if (!taskResult?.result) return { hops: [], mtrHops: [], reached: false }
  const raw = taskResult.result as Record<string, unknown>
  const data = (raw["data"] ?? raw) as Record<string, unknown>
  const reached = Boolean(data["target_reached"])

  if (variant === "mtr") {
    const mtrHops = (data["hops"] ?? []) as MTRHopRaw[]
    // Project MTR hops onto TracerouteHop shape — avg_rtt_ms becomes rtt_ms
    // so the map's color thresholds (50/200/500ms) apply consistently.
    const hops: TracerouteHop[] = mtrHops.map(h => ({
      hop: h.hop,
      ip: h.ip ?? "",
      hostname: h.hostname,
      rtt_ms: h.avg_rtt_ms ?? 0,
      timeout: h.timeout ?? false,
      country: h.country,
      city: h.city,
      lat: h.lat,
      lng: h.lng,
    }))
    return { hops, mtrHops, reached }
  }

  const hops = (data["hops"] ?? []) as TracerouteHop[]
  return { hops, mtrHops: [], reached }
}

// ── HopRow helpers ───────────────────────────────────────────────────────────

function rttColor(rtt?: number, timeout?: boolean) {
  if (timeout || rtt === undefined) return "text-muted-foreground"
  if (rtt < 50) return "text-green-600"
  if (rtt < 200) return "text-lime-600"
  if (rtt < 500) return "text-yellow-600"
  return "text-destructive"
}

function fmtLoc(country?: string, city?: string) {
  return [city, country].filter(Boolean).join(", ") || "—"
}

// ── Tables ───────────────────────────────────────────────────────────────────

function TracerouteHopTable({ hops }: { hops: TracerouteHop[] }) {
  return (
    <div className="overflow-x-auto rounded-md border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className="w-12 text-center">跳</TableHead>
            <TableHead>IP 地址</TableHead>
            <TableHead>主机名</TableHead>
            <TableHead>位置</TableHead>
            <TableHead className="text-right">RTT</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {hops.map((h) => (
            <TableRow key={h.hop}>
              <TableCell className="text-center font-mono text-sm">{h.hop}</TableCell>
              <TableCell className="font-mono text-sm">
                {h.timeout || !h.ip ? <span className="text-muted-foreground">*</span> : h.ip}
              </TableCell>
              <TableCell className="text-sm text-muted-foreground max-w-[220px] truncate">
                {h.hostname ?? "—"}
              </TableCell>
              <TableCell className="text-sm text-muted-foreground">
                {fmtLoc(h.country, h.city)}
              </TableCell>
              <TableCell className={`text-right font-mono text-sm ${rttColor(h.rtt_ms, h.timeout)}`}>
                {h.timeout ? "超时" : `${h.rtt_ms.toFixed(1)} ms`}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

function MTRHopTable({ hops }: { hops: MTRHopRaw[] }) {
  return (
    <div className="overflow-x-auto rounded-md border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className="w-12 text-center">跳</TableHead>
            <TableHead>IP 地址</TableHead>
            <TableHead>主机名</TableHead>
            <TableHead>位置</TableHead>
            <TableHead className="text-right">丢包</TableHead>
            <TableHead className="text-right">平均</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {hops.map((h) => (
            <TableRow key={h.hop}>
              <TableCell className="text-center font-mono text-sm">{h.hop}</TableCell>
              <TableCell className="font-mono text-sm">
                {h.timeout || !h.ip ? <span className="text-muted-foreground">*</span> : h.ip}
              </TableCell>
              <TableCell className="text-sm text-muted-foreground max-w-[200px] truncate">
                {h.hostname ?? "—"}
              </TableCell>
              <TableCell className="text-sm text-muted-foreground">
                {fmtLoc(h.country, h.city)}
              </TableCell>
              <TableCell className="text-right font-mono text-sm">
                {h.timeout ? "100%" : `${(h.loss_pct ?? 0).toFixed(1)}%`}
              </TableCell>
              <TableCell className={`text-right font-mono text-sm ${rttColor(h.avg_rtt_ms, h.timeout)}`}>
                {h.timeout ? "—" : `${(h.avg_rtt_ms ?? 0).toFixed(1)} ms`}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

// ── Panel ────────────────────────────────────────────────────────────────────

export function TracerouteResultPanel({
  polling,
  sourceNodeId,
  variant = "traceroute",
  headerSlot,
  error: externalError,
}: TracerouteResultPanelProps) {
  const { taskResult, isPolling, error: pollError } = polling
  const error = externalError || pollError

  const [nodeById, setNodeById] = useState<Record<string, Node>>({})
  useEffect(() => {
    let cancelled = false
    fetchNodesOnce()
      .then((nodes) => {
        if (cancelled) return
        const map: Record<string, Node> = {}
        for (const n of nodes) map[n.id] = n
        setNodeById(map)
      })
      .catch(() => { /* swallow — origin marker just won't render */ })
    return () => { cancelled = true }
  }, [])

  const { hops, mtrHops, reached } = useMemo(
    () => extractHops(taskResult, variant),
    [taskResult, variant]
  )

  // Derive origin from the source node (if its enrollment carries lat/lng).
  // The result also carries node_id, so fall back to that when sourceNodeId
  // wasn't passed in by the caller.
  const origin: TraceOrigin | undefined = useMemo(() => {
    const nid = sourceNodeId ?? (taskResult?.result?.node_id as string | undefined)
    if (!nid) return undefined
    const n = nodeById[nid]
    if (!n || n.lat === undefined || n.lng === undefined) return undefined
    return {
      name: [n.city, n.country_code].filter(Boolean).join(" / ") || n.name,
      lat: n.lat,
      lng: n.lng,
    }
  }, [sourceNodeId, taskResult, nodeById])

  // ── Empty / loading / error states ─────────────────────────────────────────
  if (isPolling && hops.length === 0) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>路径追踪结果</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-3">
            {[1, 2, 3].map((i) => (
              <div key={i} className="h-12 bg-muted/50 animate-pulse rounded-md" />
            ))}
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <Loader2Icon className="size-3.5 animate-spin" aria-hidden="true" />
              <span>正在等待节点返回每一跳...</span>
            </div>
          </div>
        </CardContent>
      </Card>
    )
  }

  if (error) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>路径追踪结果</CardTitle>
        </CardHeader>
        <CardContent>
          <Badge variant="destructive">错误：{error}</Badge>
        </CardContent>
      </Card>
    )
  }

  if (!taskResult) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>路径追踪结果</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-muted-foreground text-sm">
            提交任务后，路径地图和跳点列表将显示在此处
          </p>
        </CardContent>
      </Card>
    )
  }

  // hops resolved but empty (e.g. unresolvable target) — fall through to the
  // hop table, which will render an empty body and a clear message.
  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle className="flex items-center gap-2">
            路径追踪结果
            <Badge variant={reached ? "default" : "secondary"}>
              {reached ? "已到达目标" : "未到达目标"}
            </Badge>
            <span className="text-sm font-normal text-muted-foreground">
              共 {hops.length} 跳
            </span>
          </CardTitle>
          {headerSlot}
        </div>
      </CardHeader>
      <CardContent>
        {isPolling && (
          <div className="mb-3">
            <Badge className="gap-1.5">
              <RadioTowerIcon className="size-3 animate-pulse" aria-hidden="true" />
              正在获取结果
            </Badge>
          </div>
        )}

        <Tabs defaultValue="map" className="w-full">
          <TabsList className="mb-4">
            <TabsTrigger value="map">路径地图</TabsTrigger>
            <TabsTrigger value="list">跳点列表</TabsTrigger>
          </TabsList>
          <TabsContent value="map">
            {hops.length === 0 ? (
              <p className="text-sm text-muted-foreground">本次任务无可绘制的跳点</p>
            ) : (
              <TracerouteMap hops={hops} origin={origin} />
            )}
          </TabsContent>
          <TabsContent value="list">
            {hops.length === 0 ? (
              <p className="text-sm text-muted-foreground">无跳点数据</p>
            ) : variant === "mtr" ? (
              <MTRHopTable hops={mtrHops} />
            ) : (
              <TracerouteHopTable hops={hops} />
            )}
          </TabsContent>
        </Tabs>
      </CardContent>
    </Card>
  )
}
