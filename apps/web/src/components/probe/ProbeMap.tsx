"use client"

import { useEffect, useState } from "react"

// ── Types ─────────────────────────────────────────────────────────────────────

export interface MapNode {
  name: string
  lat: number
  lng: number
  latency_ms?: number
}

interface ProvinceData {
  id: number
  name: string
  center: [number, number] | null
  centroid: [number, number] | null
  d: string
}

interface ChinaMapData {
  w: number
  h: number
  provinces: ProvinceData[]
}

// ── Latency colors ────────────────────────────────────────────────────────────

function nodeColor(ms?: number) {
  if (ms === undefined) return "#6b7280"
  if (ms < 300)  return "#22c55e"
  if (ms < 1000) return "#f59e0b"
  return "#ef4444"
}

function provinceColor(ms?: number): string {
  if (ms === undefined) return "#cbd5e1"  // 无数据，slate-300
  if (ms < 300)  return "#4ade80"         // 绿-500（鲜绿）
  if (ms < 1000) return "#fbbf24"         // amber-400（鲜黄）
  return "#f87171"                         // red-400（鲜红）
}

// ── Strip province name suffix ────────────────────────────────────────────────

function stripSuffix(name: string) {
  return name
    .replace(/维吾尔|壮族|回族|藏族|满族/g, "")
    .replace(/自治区|特别行政区|省|市$/g, "")
}

// ── Legend ────────────────────────────────────────────────────────────────────

function Legend() {
  return (
    <div className="flex items-center gap-4 text-xs text-slate-500 px-3 py-2 border-t bg-slate-50">
      {[
        { color: "#22c55e", label: "< 300ms" },
        { color: "#f59e0b", label: "300–1000ms" },
        { color: "#ef4444", label: "> 1000ms" },
        { color: "#dde6ee", label: "无节点" },
      ].map(({ color, label }) => (
        <span key={label} className="flex items-center gap-1.5">
          <span className="h-2.5 w-2.5 rounded-full flex-shrink-0 border border-slate-300" style={{ background: color }} />
          {label}
        </span>
      ))}
    </div>
  )
}

// ── ChinaMap ──────────────────────────────────────────────────────────────────

export function ChinaMap({ nodes }: { nodes: MapNode[] }) {
  const [mapData, setMapData] = useState<ChinaMapData | null>(null)

  useEffect(() => {
    fetch("/china-map-data.json")
      .then(r => r.json())
      .then(setMapData)
      .catch(console.error)
  }, [])

  if (!mapData) return <div className="w-full h-48 bg-slate-100 animate-pulse rounded" />

  // Build name → latency lookup
  const lookup = new Map<string, number>()
  for (const n of nodes) {
    for (const p of mapData.provinces) {
      const short = stripSuffix(p.name)
      if (n.name.includes(short) && short.length > 1 && n.latency_ms !== undefined) {
        const prev = lookup.get(short)
        if (prev === undefined || n.latency_ms < prev) lookup.set(short, n.latency_ms)
      }
    }
  }

  // Build markers from matched provinces
  const markers: Array<{ x: number; y: number; node: MapNode; provName: string }> = []
  for (const n of nodes) {
    const prov = mapData.provinces.find(p => {
      const short = stripSuffix(p.name)
      return n.name.includes(short) && short.length > 1
    })
    if (!prov) continue
    const [x, y] = prov.center ?? prov.centroid ?? [0, 0]
    if (x && y) markers.push({ x, y, node: n, provName: prov.name })
  }

  const { w, h } = mapData

  return (
    <div className="w-full rounded border border-slate-200 overflow-hidden">
      <svg
        viewBox={`0 0 ${w} ${h}`}
        style={{ width: "100%", display: "block", background: "#f0f9ff" }}
        overflow="hidden"
      >
        {mapData.provinces.map(p => {
          const short = stripSuffix(p.name)
          const ms = lookup.get(short)
          return (
            <path
              key={p.id}
              d={p.d ?? ""}
              fill={provinceColor(ms)}
              stroke="#94a3b8"
              strokeWidth={0.8}
            />
          )
        })}

        {markers.map((m, i) => (
          <g key={i}>
            <circle cx={m.x} cy={m.y} r={8} fill={nodeColor(m.node.latency_ms)} stroke="#fff" strokeWidth={2} />
            {m.node.latency_ms !== undefined && (
              <text
                x={m.x}
                y={m.y - 13}
                textAnchor="middle"
                fontSize={11}
                fontWeight="bold"
                fill="#1e293b"
                fontFamily="sans-serif"
              >
                {m.node.latency_ms}ms
              </text>
            )}
          </g>
        ))}
      </svg>
      <Legend />
    </div>
  )
}

// ── WorldMap (simple country dots) ────────────────────────────────────────────

const COUNTRY_COORDS: Record<string, [number, number]> = {
  US: [-98, 38],  GB: [-3, 54],   DE: [10, 51],  FR: [2, 46],
  JP: [138, 36],  KR: [128, 36],  SG: [104, 1],  AU: [133, -25],
  IN: [78, 20],   BR: [-51, -10], CA: [-96, 60],  RU: [100, 60],
  HK: [114, 22],  TW: [121, 24],  NL: [5, 52],   SE: [18, 60],
}

export function WorldMap({ nodes }: { nodes: MapNode[] }) {
  const [geoPaths, setGeoPaths] = useState<string[]>([])
  const [markers, setMarkers] = useState<Array<{ x: number; y: number; node: MapNode }>>([])
  const W = 640, H = 380

  useEffect(() => {
    Promise.all([
      fetch("/world-110m.json").then(r => r.json()),
      import("d3-geo").then(m => m),
      import("topojson-client").then(m => m),
    ]).then(([topoJson, d3, topo]) => {
      const countries = (topo as any).feature(topoJson, topoJson.objects.countries)
      const projection = (d3 as any).geoNaturalEarth1().fitExtent([[10, 10], [W - 10, H - 10]], countries)
      const pathGen = (d3 as any).geoPath(projection)
      setGeoPaths(countries.features.map((f: any) => pathGen(f) ?? ""))

      const mk = nodes.flatMap(n => {
        const coords: [number, number] | undefined =
          (n.lat !== 0 && n.lng !== 0) ? [n.lng, n.lat] : COUNTRY_COORDS[n.name]
        if (!coords) return []
        const [x, y] = projection(coords) ?? [0, 0]
        return [{ x, y, node: n }]
      })
      setMarkers(mk)
    }).catch(console.error)
  }, [nodes.map(n => n.name).join(",")])

  return (
    <div className="w-full rounded border border-slate-200 overflow-hidden">
      <svg viewBox={`0 0 ${W} ${H}`} style={{ width: "100%", display: "block", background: "#dbeafe" }}>
        {geoPaths.map((d, i) => (
          <path key={i} d={d} fill="#bfdbfe" stroke="#ffffff" strokeWidth={0.5} />
        ))}
        {markers.map((m, i) => (
          <circle key={i} cx={m.x} cy={m.y} r={7} fill={nodeColor(m.node.latency_ms)} stroke="#fff" strokeWidth={2} />
        ))}
      </svg>
      <Legend />
    </div>
  )
}

// ── ProbeMap ──────────────────────────────────────────────────────────────────

export function ProbeMap({ nodes, isChinaOnly }: { nodes: MapNode[]; isChinaOnly: boolean }) {
  return isChinaOnly ? <ChinaMap nodes={nodes} /> : <WorldMap nodes={nodes} />
}
