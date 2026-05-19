"use client"

import { useEffect, useMemo, useRef, useState } from "react"
import { useTheme } from "next-themes"
import type { Topology, Objects } from "topojson-specification"
import type { GeoProjection, GeoPath } from "d3-geo"
import type { FeatureCollection } from "geojson"
import type { TracerouteHop } from "@/lib/api"

// ── Types ─────────────────────────────────────────────────────────────────────

export interface TraceOrigin {
  name: string
  lat: number
  lng: number
}

interface PlottedPoint {
  x: number
  y: number
  hop: number
  label: string
  ip: string
  rtt?: number
  city?: string
  country?: string
}

// ── Colors ────────────────────────────────────────────────────────────────────

function hopColor(rtt?: number, timeout?: boolean) {
  if (timeout || rtt === undefined) return "#94a3b8"
  if (rtt < 50) return "#22c55e"
  if (rtt < 200) return "#84cc16"
  if (rtt < 500) return "#f59e0b"
  return "#ef4444"
}

// ── Legend ────────────────────────────────────────────────────────────────────

const LEGEND_ITEMS = [
  { c: "#22c55e", l: "<50ms" },
  { c: "#84cc16", l: "<200ms" },
  { c: "#f59e0b", l: "<500ms" },
  { c: "#ef4444", l: "≥500ms" },
  { c: "#94a3b8", l: "超时" },
] as const

function Legend({ hopCount, reached }: { hopCount: number; reached: boolean }) {
  return (
    <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground px-3 py-2 border-t bg-muted/30">
      <span className="font-medium text-foreground">{hopCount} 跳</span>
      <span>{reached ? "已到达目标" : "未到达目标"}</span>
      <span className="ml-auto flex items-center gap-3">
        {LEGEND_ITEMS.map(({ c, l }) => (
          <span key={l} className="flex items-center gap-1">
            <span className="h-2 w-2 rounded-full" style={{ background: c }} />
            {l}
          </span>
        ))}
      </span>
    </div>
  )
}

// ── TracerouteMap ─────────────────────────────────────────────────────────────

interface TracerouteMapProps {
  hops: TracerouteHop[]
  origin?: TraceOrigin
  /** Hide the outer card chrome (rounded + border) so the map fits flush
   *  inside a tab panel that already provides its own container. */
  embedded?: boolean
  /** Authoritative "did we reach the target?" from the backend
   *  (target_reached on the task result). When omitted we fall back to
   *  inferring from the last hop's timeout flag — that can disagree with
   *  the backend when a TTL-exceeded came from the target but the last
   *  ICMP probe still timed out, so prefer passing this explicitly. */
  reached?: boolean
}

export function TracerouteMap({ hops, origin, embedded = false, reached: reachedProp }: TracerouteMapProps) {
  const { resolvedTheme } = useTheme()
  const isDark = resolvedTheme === "dark"
  // Aligned with NodesWorldMap's palette so both maps render consistently
  // under each theme. SVG fill/stroke attributes can't read CSS variables
  // reliably across browsers, so we resolve to hex per theme.
  const palette = isDark ? {
    ocean: "#0f172a", land: "#1e293b", border: "#334155",
    route: "#60a5fa", labelText: "#e2e8f0", labelHalo: "#0f172a",
  } : {
    ocean: "#f1f5f9", land: "#e2e8f0", border: "#cbd5e1",
    route: "#3b82f6", labelText: "#1e293b", labelHalo: "#ffffff",
  }
  const [geoPaths, setGeoPaths] = useState<string[]>([])
  const [projectFn, setProjectFn] = useState<GeoProjection | null>(null)
  const [hoverHop, setHoverHop] = useState<PlottedPoint | null>(null)
  const [loadFailed, setLoadFailed] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)
  const W = 720, H = 380

  // Load world topology once.
  useEffect(() => {
    let cancelled = false
    Promise.all([
      fetch("/world-110m.json").then(r => r.json()),
      import("d3-geo"),
      import("topojson-client"),
    ]).then(([topoJson, d3, topo]) => {
      if (cancelled) return
      const topology = topoJson as Topology<Objects>
      const countries = topo.feature(topology, topology.objects["countries"]!)
      const projection: GeoProjection = d3
        .geoNaturalEarth1()
        .fitExtent([[10, 10], [W - 10, H - 10]], countries)
      const pathGen: GeoPath = d3.geoPath(projection)
      setGeoPaths((countries as FeatureCollection).features.map(f => pathGen(f) ?? ""))
      setProjectFn(() => projection)
    }).catch(() => {
      // Topology fetch failure is a degraded experience, not an error worth
      // dumping a stack to the user's console. The hop list tab still works.
      if (!cancelled) setLoadFailed(true)
    })
    return () => { cancelled = true }
  }, [])

  // Plot every hop that has coords + the origin (if provided). Hops without
  // lat/lng (private, unresolved, or geo-DB miss) are dropped from the path
  // but still listed in the legend hop count, so the user sees the total.
  const points = useMemo<PlottedPoint[]>(() => {
    if (!projectFn) return []
    const pts: PlottedPoint[] = []
    if (origin && origin.lat !== 0 && origin.lng !== 0) {
      const [x, y] = projectFn([origin.lng, origin.lat]) ?? [0, 0]
      pts.push({ x, y, hop: 0, label: origin.name, ip: "", city: origin.name })
    }
    for (const h of hops) {
      if (h.timeout) continue
      if (!h.lat || !h.lng) continue
      const [x, y] = projectFn([h.lng, h.lat]) ?? [0, 0]
      pts.push({
        x, y,
        hop: h.hop,
        label: h.hostname || h.ip,
        ip: h.ip,
        rtt: h.rtt_ms,
        city: h.city,
        country: h.country,
      })
    }
    return pts
  }, [projectFn, hops, origin])

  // Quadratic-bezier path between consecutive plotted hops — slight curve
  // (~15% of segment length, perpendicular) gives a "great-circle" feel
  // without paying for d3.geoInterpolate per segment.
  const pathD = useMemo(() => {
    if (points.length < 2) return ""
    const parts: string[] = [`M ${points[0]!.x} ${points[0]!.y}`]
    for (let i = 1; i < points.length; i++) {
      const a = points[i - 1]!
      const b = points[i]!
      const mx = (a.x + b.x) / 2
      const my = (a.y + b.y) / 2
      const dx = b.x - a.x
      const dy = b.y - a.y
      // Perpendicular offset, magnitude scales with distance.
      const len = Math.hypot(dx, dy)
      const offset = Math.min(len * 0.15, 40)
      const nx = -dy / (len || 1)
      const ny = dx / (len || 1)
      parts.push(`Q ${mx + nx * offset} ${my + ny * offset}, ${b.x} ${b.y}`)
    }
    return parts.join(" ")
  }, [points])

  // Prefer the authoritative target_reached from the backend; fall back to
  // inferring from the last hop's timeout when the caller didn't pass it in.
  const reached = useMemo(() => {
    if (reachedProp !== undefined) return reachedProp
    if (hops.length === 0) return false
    return !hops[hops.length - 1]!.timeout
  }, [hops, reachedProp])

  const wrapperClass = embedded
    ? "w-full overflow-hidden relative"
    : "w-full rounded-lg border border-border overflow-hidden relative"

  return (
    <div ref={containerRef} className={wrapperClass}>
      <svg
        viewBox={`0 0 ${W} ${H}`}
        style={{ width: "100%", display: "block", background: palette.ocean }}
        onMouseLeave={() => setHoverHop(null)}
      >
        {/* Country fills */}
        {geoPaths.map((d, i) => (
          <path key={i} d={d} fill={palette.land} stroke={palette.border} strokeWidth={0.5} />
        ))}

        {/* Route path */}
        {pathD && (
          <path
            d={pathD}
            fill="none"
            stroke={palette.route}
            strokeWidth={2}
            strokeDasharray="4 3"
            strokeLinecap="round"
            opacity={0.7}
          />
        )}

        {/* Hop markers (rendered last so they sit on top of the path) */}
        {points.map((p, i) => {
          const isOrigin = p.hop === 0
          const isTarget = i === points.length - 1 && !isOrigin
          const r = isOrigin || isTarget ? 8 : 6
          const fill = isOrigin ? "#0ea5e9" : hopColor(p.rtt, false)
          return (
            <g
              key={`${i}-${p.hop}`}
              onMouseEnter={() => setHoverHop(p)}
              style={{ cursor: "pointer" }}
            >
              <circle cx={p.x} cy={p.y} r={r + 2} fill={palette.labelHalo} />
              <circle cx={p.x} cy={p.y} r={r} fill={fill} stroke={palette.labelHalo} strokeWidth={1.5} />
              {(isOrigin || isTarget || p.hop % 3 === 0) && (
                <text
                  x={p.x}
                  y={p.y - r - 4}
                  textAnchor="middle"
                  fontSize={10}
                  fontWeight="600"
                  fill={palette.labelText}
                >
                  {isOrigin ? "起点" : isTarget ? "目标" : `#${p.hop}`}
                </text>
              )}
            </g>
          )
        })}
      </svg>

      {loadFailed && (
        <div className="absolute inset-0 flex items-center justify-center bg-background/80 backdrop-blur-sm">
          <p className="text-sm text-muted-foreground">地图加载失败，请切换到&ldquo;跳点列表&rdquo;</p>
        </div>
      )}

      {/* Hover tooltip — absolute-positioned overlay so it can extend past
          the svg viewBox without cropping. */}
      {hoverHop && (
        <div className="pointer-events-none absolute top-2 right-2 max-w-xs rounded-md border bg-background/95 backdrop-blur px-3 py-2 text-xs shadow-sm">
          <div className="font-semibold mb-1">
            {hoverHop.hop === 0 ? "起点节点" : `Hop #${hoverHop.hop}`}
          </div>
          {hoverHop.ip && (
            <div className="text-muted-foreground">
              <span className="text-foreground/70">IP: </span>
              {hoverHop.ip}
            </div>
          )}
          {hoverHop.label && hoverHop.label !== hoverHop.ip && (
            <div className="text-muted-foreground truncate">
              <span className="text-foreground/70">主机名: </span>
              {hoverHop.label}
            </div>
          )}
          {(hoverHop.city || hoverHop.country) && (
            <div className="text-muted-foreground">
              <span className="text-foreground/70">位置: </span>
              {[hoverHop.city, hoverHop.country].filter(Boolean).join(", ")}
            </div>
          )}
          {hoverHop.rtt !== undefined && (
            <div className="text-muted-foreground">
              <span className="text-foreground/70">延迟: </span>
              <span style={{ color: hopColor(hoverHop.rtt, false) }}>
                {hoverHop.rtt.toFixed(1)} ms
              </span>
            </div>
          )}
        </div>
      )}

      <Legend hopCount={hops.length} reached={reached} />
    </div>
  )
}
