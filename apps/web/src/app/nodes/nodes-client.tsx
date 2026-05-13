"use client"

import { useState, useMemo } from "react"
import dynamic from "next/dynamic"
import { Node } from "@/lib/api"
import { countryFlag, countryCoords, getCountryName } from "@/lib/country"
import { Search } from "lucide-react"

const ReactECharts = dynamic(() => import("echarts-for-react"), { ssr: false })

interface NodesClientProps {
  initialNodes: Node[]
}

const TIER_LABELS: Record<string, string> = {
  tier1_cn: "国内一级节点",
  tier1_overseas: "海外一级节点",
  community: "社区节点",
}

const TIER_COLORS: Record<string, string> = {
  tier1_cn: "hsl(var(--primary))",
  tier1_overseas: "hsl(var(--blue))",
  community: "hsl(var(--success))",
}

export function NodesClient({ initialNodes }: NodesClientProps) {
  const [selectedCountry, setSelectedCountry] = useState<string>("all")
  const [selectedTier, setSelectedTier] = useState<string>("all")
  const [searchQuery, setSearchQuery] = useState<string>("")
  const [currentPage, setCurrentPage] = useState(1)
  const itemsPerPage = 20

  // 提取唯一的国家列表
  const countries = useMemo(() => {
    const uniqueCountries = Array.from(
      new Set(initialNodes.map((n) => n.country_code))
    ).sort()
    return uniqueCountries
  }, [initialNodes])

  // 过滤节点
  const filteredNodes = useMemo(() => {
    return initialNodes.filter((node) => {
      const matchesCountry =
        selectedCountry === "all" || node.country_code === selectedCountry
      const matchesTier = selectedTier === "all" || node.tier === selectedTier
      const matchesSearch =
        searchQuery === "" ||
        node.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
        node.isp?.toLowerCase().includes(searchQuery.toLowerCase()) ||
        node.city.toLowerCase().includes(searchQuery.toLowerCase())

      return matchesCountry && matchesTier && matchesSearch
    })
  }, [initialNodes, selectedCountry, selectedTier, searchQuery])

  // 分页
  const totalPages = Math.ceil(filteredNodes.length / itemsPerPage)
  const paginatedNodes = useMemo(() => {
    const startIndex = (currentPage - 1) * itemsPerPage
    return filteredNodes.slice(startIndex, startIndex + itemsPerPage)
  }, [filteredNodes, currentPage])

  // 重置分页当过滤条件变化
  useMemo(() => {
    setCurrentPage(1)
  }, [selectedCountry, selectedTier, searchQuery])

  // 地图数据
  const mapOption = useMemo(() => {
    const scatterData = filteredNodes
      .map((node) => {
        const coords = countryCoords[node.country_code]
        if (!coords) return null
        return {
          name: node.name,
          value: [...coords, 1],
          node,
        }
      })
      .filter(Boolean)

    return {
      tooltip: {
        trigger: "item",
        formatter: (params: any) => {
          if (!params.data?.node) return ""
          const node = params.data.node as Node
          return `
            <div style="text-align: left;">
              <strong>${countryFlag(node.country_code)} ${node.name}</strong><br/>
              <span style="color: #999;">位置：</span>${node.city}, ${node.region}<br/>
              <span style="color: #999;">ISP：</span>${node.isp}<br/>
              <span style="color: #999;">ASN：</span>${node.asn}<br/>
              <span style="color: #999;">类型：</span>${TIER_LABELS[node.tier]}<br/>
              <span style="color: #999;">状态：</span>${node.status === "active" ? "在线" : "离线"}
            </div>
          `
        },
      },
      geo: {
        map: "world",
        roam: true,
        itemStyle: {
          areaColor: "hsl(var(--muted))",
          borderColor: "hsl(var(--border))",
        },
        emphasis: {
          itemStyle: {
            areaColor: "hsl(var(--accent))",
          },
        },
      },
      series: [
        {
          name: "节点",
          type: "scatter",
          coordinateSystem: "geo",
          data: scatterData,
          symbolSize: 10,
          itemStyle: {
            color: (params: any) => {
              const node = params.data?.node as Node
              return TIER_COLORS[node?.tier] || "hsl(var(--foreground))"
            },
          },
          emphasis: {
            itemStyle: {
              borderColor: "hsl(var(--ring))",
              borderWidth: 2,
            },
          },
        },
      ],
    }
  }, [filteredNodes])

  return (
    <>
      {/* 筛选栏 */}
      <div className="mb-6 flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
        <div className="flex flex-col gap-4 md:flex-row md:items-center md:gap-4">
          {/* 国家筛选 */}
          <select
            value={selectedCountry}
            onChange={(e) => setSelectedCountry(e.target.value)}
            className="rounded-md border border-input bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
          >
            <option value="all">所有国家/地区</option>
            {countries.map((code) => (
              <option key={code} value={code}>
                {countryFlag(code)} {getCountryName(code)}
              </option>
            ))}
          </select>

          {/* Tier 筛选 */}
          <select
            value={selectedTier}
            onChange={(e) => setSelectedTier(e.target.value)}
            className="rounded-md border border-input bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
          >
            <option value="all">所有节点类型</option>
            {Object.entries(TIER_LABELS).map(([key, label]) => (
              <option key={key} value={key}>
                {label}
              </option>
            ))}
          </select>
        </div>

        {/* 搜索框 */}
        <div className="relative">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <input
            type="text"
            placeholder="搜索节点名称、城市或 ISP..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="w-full rounded-md border border-input bg-background py-2 pl-10 pr-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring md:w-80"
          />
        </div>
      </div>

      {/* 地图可视化 */}
      <div className="mb-8 rounded-lg border bg-card p-4">
        <h2 className="mb-4 text-xl font-semibold">节点分布地图</h2>
        <div className="h-[500px]">
          <ReactECharts option={mapOption} style={{ height: "100%", width: "100%" }} />
        </div>
      </div>

      {/* 统计信息 */}
      <div className="mb-6 rounded-lg border bg-card p-4">
        <div className="flex items-center justify-between">
          <div>
            <p className="text-sm text-muted-foreground">
              共 <span className="font-semibold text-foreground">{filteredNodes.length}</span> 个节点
              {(selectedCountry !== "all" || selectedTier !== "all" || searchQuery) && (
                <span className="ml-2">
                  （已过滤，总计 {initialNodes.length} 个）
                </span>
              )}
            </p>
          </div>
          <div className="flex items-center gap-4 text-sm">
            {Object.entries(TIER_LABELS).map(([key, label]) => (
              <div key={key} className="flex items-center gap-2">
                <span
                  className="h-3 w-3 rounded-full"
                  style={{ backgroundColor: TIER_COLORS[key] }}
                />
                <span className="text-muted-foreground">{label}</span>
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* 节点卡片列表 */}
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
        {paginatedNodes.map((node) => (
          <div
            key={node.id}
            className="rounded-lg border bg-card p-4 transition-colors hover:bg-accent"
          >
            <div className="mb-2 flex items-start justify-between">
              <div className="flex items-center gap-2">
                <span className="text-2xl">{countryFlag(node.country_code)}</span>
                <div>
                  <h3 className="font-semibold">{node.name}</h3>
                  <p className="text-xs text-muted-foreground">
                    {node.city}, {node.region}
                  </p>
                </div>
              </div>
              <div className="flex flex-col items-end gap-1">
                <span
                  className={`rounded-full px-2 py-0.5 text-xs font-medium ${
                    node.status === "active"
                      ? "bg-success/10 text-success"
                      : "bg-muted text-muted-foreground"
                  }`}
                >
                  {node.status === "active" ? "在线" : "离线"}
                </span>
              </div>
            </div>

            <div className="space-y-1 text-sm">
              <div className="flex justify-between">
                <span className="text-muted-foreground">ISP</span>
                <span className="font-mono">{node.isp}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-muted-foreground">ASN</span>
                <span className="font-mono">{node.asn}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-muted-foreground">类型</span>
                <span
                  className="rounded px-2 py-0.5 text-xs font-medium"
                  style={{
                    backgroundColor: `${TIER_COLORS[node.tier]}20`,
                    color: TIER_COLORS[node.tier],
                  }}
                >
                  {TIER_LABELS[node.tier]}
                </span>
              </div>
            </div>
          </div>
        ))}
      </div>

      {/* 分页 */}
      {totalPages > 1 && (
        <div className="mt-8 flex items-center justify-center gap-2">
          <button
            onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
            disabled={currentPage === 1}
            className="rounded-md border border-input bg-background px-4 py-2 text-sm font-medium transition-colors hover:bg-accent disabled:cursor-not-allowed disabled:opacity-50"
          >
            上一页
          </button>
          <span className="text-sm text-muted-foreground">
            第 {currentPage} 页，共 {totalPages} 页
          </span>
          <button
            onClick={() => setCurrentPage((p) => Math.min(totalPages, p + 1))}
            disabled={currentPage === totalPages}
            className="rounded-md border border-input bg-background px-4 py-2 text-sm font-medium transition-colors hover:bg-accent disabled:cursor-not-allowed disabled:opacity-50"
          >
            下一页
          </button>
        </div>
      )}
    </>
  )
}
