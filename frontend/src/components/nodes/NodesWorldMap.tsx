"use client"

import { useEffect, useMemo, useState } from "react"
import { useTheme } from "next-themes"
import type { NodeEntry } from "@/lib/nodes-utils"
import type { Topology, Objects } from "topojson-specification"
import type { GeoProjection, GeoPath } from "d3-geo"
import type { FeatureCollection } from "geojson"

// 国家代码 → [lng, lat]
const COUNTRY_COORDS: Record<string, [number, number]> = {
  CN: [104.2, 35.8], HK: [114.2, 22.3], TW: [121.5, 25.0], MO: [113.5, 22.2],
  JP: [138.3, 36.6], KR: [127.8, 36.5], SG: [103.8, 1.4],
  TH: [100.5, 13.8], MY: [109.7, 4.2], ID: [113.9, -0.8],
  VN: [108.3, 14.1], PH: [122.9, 12.9], IN: [78.9, 20.6],
  US: [-98.0, 39.0], CA: [-96.8, 56.1], MX: [-102.5, 23.6],
  BR: [-51.9, -14.2], AR: [-63.6, -38.4],
  GB: [-3.4, 55.4], DE: [10.5, 51.2], FR: [2.2, 46.2],
  NL: [5.3, 52.1], SE: [18.6, 60.1], NO: [8.5, 60.5],
  PL: [19.1, 51.9], ES: [-3.7, 40.4], IT: [12.6, 41.9],
  CH: [8.2, 46.8], RU: [105.3, 61.5],
  AU: [134.5, -25.3], NZ: [172.5, -41.5],
  ZA: [22.9, -30.6], AE: [53.8, 23.4], TR: [35.2, 39.1],
}

// 名称模糊匹配兜底（country_code 为空时）
const NAME_COORDS: Array<{ keys: string[]; coords: [number, number] }> = [
  { keys: ["Singapore", "新加坡", "SG"],        coords: [103.8, 1.4] },
  { keys: ["Hong Kong", "HongKong", "香港", "HK"], coords: [114.2, 22.3] },
  { keys: ["Tokyo", "Japan", "日本", "东京"],    coords: [139.7, 35.7] },
  { keys: ["Seoul", "Korea", "韩国", "首尔"],    coords: [126.9, 37.6] },
  { keys: ["Beijing", "Shanghai", "Guangzhou", "China", "中国", "北京", "上海", "广州"], coords: [104.2, 35.8] },
  { keys: ["Los Angeles", "LA", "California"],   coords: [-118.2, 34.1] },
  { keys: ["New York", "NYC"],                   coords: [-74.0, 40.7] },
  { keys: ["London", "UK"],                      coords: [-0.1, 51.5] },
  { keys: ["Frankfurt", "Germany"],              coords: [8.7, 50.1] },
  { keys: ["Amsterdam", "Netherlands"],          coords: [4.9, 52.4] },
  { keys: ["Sydney", "Australia"],               coords: [151.2, -33.9] },
]

function lookupByName(name: string): [number, number] | undefined {
  const lower = name.toLowerCase()
  for (const e of NAME_COORDS) {
    if (e.keys.some(k => lower.includes(k.toLowerCase()))) return e.coords
  }
  return undefined
}

interface Marker {
  x: number; y: number
  online: boolean
  label: string
}

export function NodesWorldMap({ nodes }: { nodes: NodeEntry[] }) {
  const [geoPaths, setGeoPaths] = useState<string[]>([])
  const [markers, setMarkers] = useState<Marker[]>([])
  const [tooltip, setTooltip] = useState<{ x: number; y: number; label: string } | null>(null)
  const W = 800, H = 400
  const nodeIdKey = useMemo(() => nodes.map(n => n.id).join(","), [nodes])

  const { resolvedTheme } = useTheme()
  const isDark = resolvedTheme !== "light"
  const c = isDark ? {
    ocean: "#0f172a", land: "#1e293b", border: "#334155",
    tipBg: "rgba(15,23,42,0.92)", tipBorder: "#334155", tipText: "#f1f5f9",
  } : {
    ocean: "#dde6f0", land: "#c8d6e8", border: "#8fa8c4",
    tipBg: "rgba(248,250,252,0.96)", tipBorder: "#8fa8c4", tipText: "#1e293b",
  }

  // 按坐标聚合节点（同一国家只打一个点，但保留所有节点名用于 tooltip）
  useEffect(() => {
    Promise.all([
      fetch("/world-110m.json").then(r => r.json()),
      import("d3-geo"),
      import("topojson-client"),
    ]).then(([topoJson, d3, topo]) => {
      const topology = topoJson as Topology<Objects>
      const countries = topo.feature(topology, topology.objects["countries"]!)
      const projection: GeoProjection = d3.geoNaturalEarth1()
        .fitExtent([[12, 12], [W - 12, H - 12]], countries)
      const pathGen: GeoPath = d3.geoPath(projection)
      setGeoPaths((countries as FeatureCollection).features.map(f => pathGen(f) ?? ""))

      // 聚合：key = "lng,lat"，value = { online, labels }
      const grouped = new Map<string, { online: boolean; names: string[] }>()

      for (const n of nodes) {
        const coords: [number, number] | undefined =
          (n.country && COUNTRY_COORDS[n.country])
            ? COUNTRY_COORDS[n.country]
            : lookupByName(n.name || n.region || "")

        if (!coords) continue
        const key = coords.join(",")
        const existing = grouped.get(key)
        if (existing) {
          existing.online = existing.online || n.status === "online"
          if (n.name) existing.names.push(n.name)
        } else {
          grouped.set(key, { online: n.status === "online", names: n.name ? [n.name] : [] })
        }
      }

      const mk: Marker[] = []
      for (const [key, val] of grouped) {
        const [lngStr, latStr] = key.split(",")
        const [x, y] = projection([parseFloat(lngStr ?? "0"), parseFloat(latStr ?? "0")]) ?? [0, 0]
        mk.push({ x, y, online: val.online, label: val.names.join(" · ") || key })
      }
      setMarkers(mk)
    }).catch(console.error)
  // eslint-disable-next-line react-hooks/exhaustive-deps -- nodeIdKey 派生自 nodes，作为稳定 key 避免每次渲染重新 fetch
  }, [nodeIdKey])

  return (
    <div className="w-full overflow-hidden rounded-b-md relative" style={{ touchAction: "pan-y" }}>
      <svg
        viewBox={`0 0 ${W} ${H}`}
        style={{ width: "100%", display: "block", touchAction: "pan-y", background: c.ocean }}
        onMouseLeave={() => setTooltip(null)}
      >
        <rect width={W} height={H} fill={c.ocean} />

        {geoPaths.map((d, i) => (
          <path key={i} d={d} fill={c.land} stroke={c.border} strokeWidth={0.4} />
        ))}

        {markers.map((m, i) => (
          <g
            key={i}
            style={{ cursor: "pointer" }}
            onMouseEnter={(e) => {
              const svg = (e.currentTarget as SVGGElement).closest("svg")!
              const rect = svg.getBoundingClientRect()
              const scaleX = W / rect.width
              const scaleY = H / rect.height
              setTooltip({
                x: (e.clientX - rect.left) * scaleX,
                y: (e.clientY - rect.top) * scaleY - 14,
                label: m.label,
              })
            }}
            onMouseLeave={() => setTooltip(null)}
          >
            <circle cx={m.x} cy={m.y} r={10}
              fill={m.online ? "rgba(34,197,94,0.12)" : "rgba(148,163,184,0.08)"} />
            <circle cx={m.x} cy={m.y} r={4.5}
              fill={m.online ? "#22c55e" : "#64748b"}
              stroke={m.online ? "#86efac" : "#94a3b8"}
              strokeWidth={1.2} />
          </g>
        ))}

        {/* SVG tooltip */}
        {tooltip && (
          <g pointerEvents="none">
            <rect
              x={tooltip.x - 4} y={tooltip.y - 16}
              width={tooltip.label.length * 7 + 16} height={22}
              rx={4} fill={c.tipBg} stroke={c.tipBorder} strokeWidth={0.8}
            />
            <text x={tooltip.x + 4} y={tooltip.y - 1}
              fill={c.tipText} fontSize={11} fontFamily="monospace">
              {tooltip.label}
            </text>
          </g>
        )}
      </svg>
    </div>
  )
}
