"use client"

import { ComposableMap, Geographies, Geography, Marker, ZoomableGroup } from "react-simple-maps"
import { cn } from "@/lib/utils"

// ── Types ─────────────────────────────────────────────────────────────────────

export interface MapNode {
  name: string
  lat: number
  lng: number
  latency_ms?: number
}

// ── Latency color ─────────────────────────────────────────────────────────────

function nodeColor(ms?: number): string {
  if (ms === undefined) return "#6b7280"
  if (ms < 300)  return "#22c55e"   // green
  if (ms < 1000) return "#eab308"   // yellow
  return "#ef4444"                   // red
}

// ── Legend ────────────────────────────────────────────────────────────────────

function MapLegend() {
  return (
    <div className="flex items-center gap-4 text-xs text-muted-foreground mt-3">
      <span className="flex items-center gap-1.5">
        <span className="h-2.5 w-2.5 rounded-full bg-green-500" /> 0–300ms
      </span>
      <span className="flex items-center gap-1.5">
        <span className="h-2.5 w-2.5 rounded-full bg-yellow-500" /> 300–1000ms
      </span>
      <span className="flex items-center gap-1.5">
        <span className="h-2.5 w-2.5 rounded-full bg-red-500" /> {">"}1000ms
      </span>
    </div>
  )
}

// ── China province → center coordinates ──────────────────────────────────────

const CN_PROVINCE_COORDS: Record<string, [number, number]> = {
  "北京":   [116.41, 39.91], "天津":   [117.20, 39.13], "上海":   [121.47, 31.23],
  "重庆":   [106.55, 29.56], "河北":   [114.47, 38.04], "山西":   [112.57, 37.87],
  "辽宁":   [123.43, 41.80], "吉林":   [125.32, 43.90], "黑龙江": [126.66, 45.75],
  "江苏":   [118.76, 32.05], "浙江":   [120.15, 30.27], "安徽":   [117.28, 31.86],
  "福建":   [119.30, 26.08], "江西":   [115.89, 28.68], "山东":   [117.02, 36.67],
  "河南":   [113.65, 34.76], "湖北":   [114.30, 30.60], "湖南":   [112.98, 28.19],
  "广东":   [113.28, 23.13], "广西":   [108.34, 22.82], "海南":   [110.35, 20.02],
  "四川":   [104.07, 30.65], "贵州":   [106.71, 26.57], "云南":   [102.71, 25.04],
  "陕西":   [108.93, 34.27], "甘肃":   [103.82, 36.05], "青海":   [101.78, 36.62],
  "内蒙古": [111.76, 40.82], "西藏":   [91.12,  29.65], "新疆":   [87.62,  43.82],
  "宁夏":   [106.27, 38.47],
}

function findCnCoords(nodeName: string): [number, number] | null {
  for (const [prov, coords] of Object.entries(CN_PROVINCE_COORDS)) {
    if (nodeName.includes(prov)) return coords
  }
  return null
}

// ── Country code → approximate center ────────────────────────────────────────

const COUNTRY_COORDS: Record<string, [number, number]> = {
  US: [-98, 38],  GB: [-3, 54],   DE: [10, 51],  FR: [2, 46],
  JP: [138, 36],  KR: [128, 36],  SG: [104, 1],  AU: [133, -25],
  IN: [78, 20],   BR: [-51, -10], CA: [-96, 60],  RU: [100, 60],
  HK: [114, 22],  TW: [121, 24],  NL: [5, 52],   SE: [18, 60],
  CH: [8, 47],    IT: [12, 42],   ES: [-3, 40],   PL: [20, 52],
  UA: [31, 49],   ZA: [25, -29],  NG: [8, 10],    EG: [30, 27],
  MX: [-102, 23], AR: [-64, -34], CL: [-71, -30], ID: [120, -5],
  MY: [110, 4],   TH: [101, 15],  VN: [108, 14],  PH: [122, 12],
}

// ── ChinaMap ──────────────────────────────────────────────────────────────────

export function ChinaMap({ nodes }: { nodes: MapNode[] }) {
  const markers = nodes.flatMap(n => {
    const coords = findCnCoords(n.name)
    return coords ? [{ ...n, lat: coords[1], lng: coords[0] }] : []
  })

  return (
    <div className="w-full">
      <ComposableMap
        projection="geoMercator"
        projectionConfig={{ center: [105, 36], scale: 500 }}
        width={460}
        height={340}
        style={{ width: "100%", height: "auto" }}
      >
        <Geographies geography="/china-provinces.json">
          {({ geographies }: { geographies: any[] }) =>
            geographies.map((geo: any) => (
              <Geography
                key={geo.rsmKey}
                geography={geo}
                fill="#3f3f46"
                stroke="#71717a"
                strokeWidth={0.6}
                style={{
                  default: { outline: "none" },
                  hover:   { fill: "#52525b", outline: "none" },
                  pressed: { outline: "none" },
                }}
              />
            ))
          }
        </Geographies>

        {markers.map((m, i) => (
          <Marker key={i} coordinates={[m.lng, m.lat]}>
            <circle
              r={5}
              fill={nodeColor(m.latency_ms)}
              stroke="hsl(var(--background))"
              strokeWidth={1.5}
              opacity={0.9}
            />
            {m.latency_ms !== undefined && (
              <text
                textAnchor="middle"
                y={-9}
                style={{ fontSize: 9, fill: "hsl(var(--foreground))", fontFamily: "inherit" }}
              >
                {m.latency_ms}ms
              </text>
            )}
          </Marker>
        ))}
      </ComposableMap>
      <MapLegend />
    </div>
  )
}

// ── WorldMap ──────────────────────────────────────────────────────────────────

export function WorldMap({ nodes }: { nodes: MapNode[] }) {
  const markers = nodes.flatMap(n => {
    // Try to find coordinates from node_name or pass lat/lng directly
    const coords: [number, number] | null =
      n.lat !== 0 && n.lng !== 0 ? [n.lng, n.lat] : null
    return coords ? [{ ...n, _coords: coords }] : []
  })

  return (
    <div className="w-full">
      <ComposableMap
        projection="geoNaturalEarth1"
        projectionConfig={{ scale: 140 }}
        width={460}
        height={260}
        style={{ width: "100%", height: "auto" }}
      >
        <Geographies geography="/world-110m.json">
          {({ geographies }: { geographies: any[] }) =>
            geographies.map((geo: any) => (
              <Geography
                key={geo.rsmKey}
                geography={geo}
                fill="#3f3f46"
                stroke="#71717a"
                strokeWidth={0.5}
                style={{
                  default: { outline: "none" },
                  hover:   { fill: "hsl(var(--muted-foreground) / 0.3)", outline: "none" },
                  pressed: { outline: "none" },
                }}
              />
            ))
          }
        </Geographies>

        {markers.map((m, i) => (
          <Marker key={i} coordinates={m._coords}>
            <circle
              r={5}
              fill={nodeColor(m.latency_ms)}
              stroke="hsl(var(--background))"
              strokeWidth={1.5}
              opacity={0.9}
            />
          </Marker>
        ))}
      </ComposableMap>
      <MapLegend />
    </div>
  )
}

// ── ProbeMap (auto-selects China vs World) ────────────────────────────────────

interface ProbeMapProps {
  nodes: MapNode[]
  isChinaOnly: boolean
}

export function ProbeMap({ nodes, isChinaOnly }: ProbeMapProps) {
  if (isChinaOnly) {
    return <ChinaMap nodes={nodes} />
  }
  return <WorldMap nodes={nodes} />
}
