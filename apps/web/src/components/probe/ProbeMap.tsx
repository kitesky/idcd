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

// 精确到城市/州级坐标表
// key 支持: 国家代码、城市名、州名、常见节点名（部分匹配）
const LOCATION_COORDS: Array<{ keys: string[]; coords: [number, number] }> = [
  // ── 美国各州/城市 ────────────────────────────────────────────────────────
  { keys: ["New York", "NYC", "纽约"],                    coords: [-74.0, 40.7] },
  { keys: ["Oregon", "Portland", "俄勒冈"],               coords: [-122.7, 45.5] },
  { keys: ["California", "Los Angeles", "LA", "加州"],    coords: [-118.2, 34.1] },
  { keys: ["San Francisco", "SF", "旧金山"],              coords: [-122.4, 37.8] },
  { keys: ["Seattle", "Washington State", "西雅图"],      coords: [-122.3, 47.6] },
  { keys: ["Texas", "Dallas", "Houston", "德克萨斯"],     coords: [-96.8, 32.8] },
  { keys: ["Chicago", "Illinois", "芝加哥"],              coords: [-87.6, 41.9] },
  { keys: ["Virginia", "Ashburn", "弗吉尼亚"],            coords: [-77.5, 39.0] },
  { keys: ["Miami", "Florida", "迈阿密"],                 coords: [-80.2, 25.8] },
  { keys: ["US", "United States", "美国"],                coords: [-98.0, 39.0] },
  // ── 欧洲 ────────────────────────────────────────────────────────────────
  { keys: ["London", "UK", "United Kingdom", "英国", "伦敦"],      coords: [-0.1, 51.5] },
  { keys: ["Frankfurt", "Germany", "德国", "法兰克福"],            coords: [8.7, 50.1] },
  { keys: ["Amsterdam", "Netherlands", "荷兰"],                    coords: [4.9, 52.4] },
  { keys: ["Paris", "France", "法国", "巴黎"],                     coords: [2.3, 48.9] },
  { keys: ["Stockholm", "Sweden", "瑞典"],                         coords: [18.1, 59.3] },
  { keys: ["Warsaw", "Poland", "波兰"],                            coords: [21.0, 52.2] },
  { keys: ["Madrid", "Spain", "西班牙"],                           coords: [-3.7, 40.4] },
  { keys: ["Milan", "Italy", "意大利"],                            coords: [9.2, 45.5] },
  // ── 亚太 ────────────────────────────────────────────────────────────────
  { keys: ["Tokyo", "Japan", "日本", "东京"],                      coords: [139.7, 35.7] },
  { keys: ["Seoul", "Korea", "韩国", "首尔"],                      coords: [126.9, 37.6] },
  { keys: ["Singapore", "新加坡"],                                  coords: [103.8, 1.4] },
  { keys: ["Hong Kong", "HK", "香港"],                             coords: [114.2, 22.3] },
  { keys: ["Taipei", "Taiwan", "台湾"],                            coords: [121.5, 25.0] },
  { keys: ["Sydney", "Australia", "澳大利亚", "悉尼"],             coords: [151.2, -33.9] },
  { keys: ["Melbourne", "墨尔本"],                                  coords: [144.9, -37.8] },
  { keys: ["Mumbai", "India", "印度"],                             coords: [72.9, 19.1] },
  { keys: ["Bangkok", "Thailand", "泰国"],                         coords: [100.5, 13.8] },
  // ── 其他 ────────────────────────────────────────────────────────────────
  { keys: ["São Paulo", "Brazil", "巴西"],                         coords: [-46.6, -23.5] },
  { keys: ["Toronto", "Canada", "加拿大"],                         coords: [-79.4, 43.7] },
  { keys: ["Moscow", "Russia", "俄罗斯"],                          coords: [37.6, 55.8] },
  { keys: ["Dubai", "UAE", "阿联酋"],                              coords: [55.3, 25.2] },
  { keys: ["Johannesburg", "South Africa", "南非"],                coords: [28.0, -26.2] },
]

function lookupCoords(nodeName: string): [number, number] | undefined {
  const lower = nodeName.toLowerCase()
  for (const entry of LOCATION_COORDS) {
    if (entry.keys.some(k => lower.includes(k.toLowerCase()))) {
      return entry.coords
    }
  }
  return undefined
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
        // 优先用节点自带的经纬度，其次模糊匹配城市/州名
        const coords: [number, number] | undefined =
          (n.lat !== 0 && n.lng !== 0) ? [n.lng, n.lat] : lookupCoords(n.name)
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
