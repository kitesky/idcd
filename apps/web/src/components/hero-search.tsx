"use client"

import { useState, useEffect, useRef } from "react"
import { ChevronDown, Search, Settings2, ArrowRight, MapPin, RotateCcw, Check } from "lucide-react"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import { getNodes, probeHttp, probePing, probeDns, probeTcp, probeMtr, probeTraceroute } from "@/lib/api"
import type { Node as ProbeNode, ProbeResult } from "@/lib/api"
import { ProbeResultPanel } from "@/components/probe/ProbeResultPanel"

// ── 大类 tab 配置 ────────────────────────────────────────────────────────────

interface ToolTab {
  key: string
  label: string
  path: string
}

interface CategoryTab {
  key: string
  label: string
  title: string
  subtitle: string
  desc: string
  tools: ToolTab[]
  inputPlaceholder: string
  showNodeSelector: boolean
}

const CATEGORIES: CategoryTab[] = [
  {
    key: "diagnose",
    label: "一键诊断",
    title: "一键综合诊断",
    subtitle: "HTTP • Ping • DNS • SSL • 路由追踪",
    desc: "一次输入，同时发起多项检测，快速定位域名/IP 的网络连通性和配置问题。",
    tools: [],
    inputPlaceholder: "请输入域名或 IP，如 github.com",
    showNodeSelector: false,
  },
  {
    key: "probe",
    label: "网络拨测",
    title: "网络拨测工具",
    subtitle: "多节点实时拨测 • 网络连通性诊断",
    desc: "全球 100+ 网络拨测节点，模拟用户访问域名/IP，帮助发现网络、站点可用性问题。",
    tools: [
      { key: "http",       label: "HTTP",       path: "/tools/http" },
      { key: "ping",       label: "Ping",       path: "/tools/ping" },
      { key: "dns",        label: "DNS",        path: "/tools/dns" },
      { key: "tcp",        label: "TCP",        path: "/tools/tcp" },
      { key: "mtr",        label: "MTR",        path: "/tools/mtr" },
      { key: "traceroute", label: "Traceroute", path: "/tools/traceroute" },
    ],
    inputPlaceholder: "请输入域名或 IP",
    showNodeSelector: true,
  },
  {
    key: "domain",
    label: "域名查询",
    title: "域名信息查询",
    subtitle: "WHOIS • ICP • SSL • DNS 全方位查询",
    desc: "查询域名注册信息、ICP 备案、SSL 证书状态及 DNS 解析记录。",
    tools: [
      { key: "whois", label: "WHOIS",  path: "/tools/whois" },
      { key: "icp",   label: "ICP 备案", path: "/tools/icp" },
      { key: "ssl",   label: "SSL 证书", path: "/tools/ssl" },
      { key: "dns",   label: "DNS 解析", path: "/tools/dns" },
    ],
    inputPlaceholder: "请输入域名，如 example.com",
    showNodeSelector: false,
  },
]

// ── NodePanel ─────────────────────────────────────────────────────────────────
// 按 country_code 分 中国 / 全球 两个 tab，多选节点，确认后回传 id 列表

interface NodePanelProps {
  selectedIds: string[]
  onConfirm: (ids: string[]) => void
  onClose: () => void
}

function NodePanel({ selectedIds, onConfirm, onClose }: NodePanelProps) {
  const [tab, setTab] = useState<"cn" | "global">("cn")
  const [nodes, setNodes] = useState<ProbeNode[]>([])
  const [loading, setLoading] = useState(true)
  const [pending, setPending] = useState<string[]>(selectedIds)

  useEffect(() => {
    getNodes()
      .then(setNodes)
      .catch(() => setNodes([]))
      .finally(() => setLoading(false))
  }, [])

  const cnNodes     = nodes.filter(n => n.country_code === "CN" && n.is_active)
  const globalNodes = nodes.filter(n => n.country_code !== "CN" && n.is_active)
  const displayed   = tab === "cn" ? cnNodes : globalNodes

  // 按 region / country_code 分组
  const grouped = displayed.reduce<Record<string, ProbeNode[]>>((acc, n) => {
    const key = n.region ?? n.city ?? n.country_code
    ;(acc[key] ??= []).push(n)
    return acc
  }, {})

  const toggleNode = (id: string) => {
    setPending(prev =>
      prev.includes(id) ? prev.filter(x => x !== id) : [...prev, id]
    )
  }

  const toggleGroup = (groupNodes: ProbeNode[]) => {
    const ids = groupNodes.map(n => n.id)
    const allSelected = ids.every(id => pending.includes(id))
    if (allSelected) {
      setPending(prev => prev.filter(id => !ids.includes(id)))
    } else {
      setPending(prev => [...new Set([...prev, ...ids])])
    }
  }

  const reset = () => setPending([])

  const pendingInView = pending.filter(id =>
    displayed.some(n => n.id === id)
  ).length

  return (
    <div className="absolute top-full left-0 z-50 mt-1 w-[480px] max-w-[calc(100vw-2rem)] bg-popover border rounded-lg shadow-xl overflow-hidden">
      {/* 中国 / 全球 tabs */}
      <div className="flex border-b">
        {(["cn", "global"] as const).map(t => (
          <button
            key={t}
            type="button"
            onClick={() => setTab(t)}
            className={cn(
              "flex-1 py-2.5 text-sm font-medium transition-colors",
              tab === t
                ? "text-primary border-b-2 border-primary bg-muted/30"
                : "text-muted-foreground hover:text-foreground"
            )}
          >
            {t === "cn" ? "中国" : "全球"}
          </button>
        ))}
      </div>

      {/* 节点列表 */}
      <div className="max-h-72 overflow-y-auto p-4">
        {loading ? (
          <p className="text-sm text-muted-foreground text-center py-6">加载节点中...</p>
        ) : displayed.length === 0 ? (
          <p className="text-sm text-muted-foreground text-center py-6">暂无可用节点</p>
        ) : (
          <div className="space-y-4">
            {Object.entries(grouped).map(([group, groupNodes]) => {
              const allSelected = groupNodes.every(n => pending.includes(n.id))
              return (
                <div key={group}>
                  {/* 分组标题 + 全选 */}
                  <div className="flex items-center gap-2 mb-2">
                    <label className="flex items-center gap-1.5 cursor-pointer select-none">
                      <div
                        onClick={() => toggleGroup(groupNodes)}
                        className={cn(
                          "h-4 w-4 rounded border flex items-center justify-center cursor-pointer transition-colors",
                          allSelected
                            ? "bg-primary border-primary"
                            : "border-border hover:border-primary"
                        )}
                      >
                        {allSelected && <Check className="h-2.5 w-2.5 text-primary-foreground" />}
                      </div>
                      <span className="text-xs font-medium text-muted-foreground">{group}</span>
                    </label>
                  </div>
                  {/* 节点列表 */}
                  <div className="flex flex-wrap gap-2 pl-6">
                    {groupNodes.map(node => {
                      const selected = pending.includes(node.id)
                      return (
                        <label
                          key={node.id}
                          className="flex items-center gap-1.5 cursor-pointer select-none"
                        >
                          <div
                            onClick={() => toggleNode(node.id)}
                            className={cn(
                              "h-4 w-4 rounded border flex items-center justify-center cursor-pointer transition-colors flex-shrink-0",
                              selected
                                ? "bg-primary border-primary"
                                : "border-border hover:border-primary"
                            )}
                          >
                            {selected && <Check className="h-2.5 w-2.5 text-primary-foreground" />}
                          </div>
                          <span className="text-sm text-foreground/80">{node.name}</span>
                        </label>
                      )
                    })}
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </div>

      {/* 底部操作栏 */}
      <div className="flex items-center justify-between border-t px-4 py-3 bg-muted/20">
        <span className="text-xs text-muted-foreground">
          已选 <span className="text-foreground font-medium">{pending.length}</span> 个节点发起探测
        </span>
        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={reset}
            className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors"
          >
            <RotateCcw className="h-3 w-3" />
            重置
          </button>
          <Button size="sm" className="h-7 px-4 text-xs" onClick={() => { onConfirm(pending); onClose() }}>
            确认
          </Button>
        </div>
      </div>
    </div>
  )
}

// ── NodeSelectorTrigger ──────────────────────────────────────────────────────

function NodeSelectorTrigger({ selectedIds, onConfirm }: { selectedIds: string[]; onConfirm: (ids: string[]) => void }) {
  const [open, setOpen] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    const handler = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as globalThis.Node)) {
        setOpen(false)
      }
    }
    document.addEventListener("mousedown", handler)
    return () => document.removeEventListener("mousedown", handler)
  }, [open])

  const label = selectedIds.length === 0
    ? "请选择拨测节点"
    : `已选 ${selectedIds.length} 个节点`

  return (
    <div ref={containerRef} className="relative w-full">
      <button
        type="button"
        onClick={() => setOpen(v => !v)}
        className="flex h-11 w-full items-center justify-between gap-2 px-4 text-sm text-foreground/80 hover:text-foreground transition-colors"
      >
        <div className="flex items-center gap-2 min-w-0">
          <MapPin className="h-3.5 w-3.5 text-muted-foreground flex-shrink-0" />
          <span className={cn("truncate", selectedIds.length === 0 && "text-muted-foreground/70")}>
            {label}
          </span>
        </div>
        <ChevronDown className={cn("h-4 w-4 flex-shrink-0 text-muted-foreground transition-transform duration-150", open && "rotate-180")} />
      </button>

      {open && (
        <NodePanel
          selectedIds={selectedIds}
          onConfirm={onConfirm}
          onClose={() => setOpen(false)}
        />
      )}
    </div>
  )
}

// ── Probe dispatch ────────────────────────────────────────────────────────────

const PROBE_FN: Record<string, typeof probeHttp> = {
  http: probeHttp,
  ping: probePing,
  dns: probeDns,
  tcp: probeTcp,
  mtr: probeMtr,
  traceroute: probeTraceroute,
}

// ── HeroSearch ────────────────────────────────────────────────────────────────


export function HeroSearch() {
  const [activeCat, setActiveCat] = useState("probe")
  const [activeTool, setActiveTool] = useState("http")
  const [selectedNodeIds, setSelectedNodeIds] = useState<string[]>([])
  const [query, setQuery] = useState("")
  const [probeResult, setProbeResult] = useState<ProbeResult | null>(null)
  const [probeLoading, setProbeLoading] = useState(false)
  const [probeError, setProbeError] = useState("")

  const cat  = CATEGORIES.find(c => c.key === activeCat) ?? CATEGORIES[1]!
  const tool = cat.tools.find(t => t.key === activeTool) ?? cat.tools[0]

  const handleCatChange = (key: string) => {
    setActiveCat(key)
    setProbeResult(null)
    const c = CATEGORIES.find(c => c.key === key)
    if (c?.tools.length) setActiveTool(c.tools[0]!.key)
  }

  const handleGo = async () => {
    if (!query.trim()) return
    setProbeResult(null)
    setProbeError("")
    setProbeLoading(true)

    try {
      const fn = PROBE_FN[activeTool] ?? probeHttp
      const res = await fn({
        target: query.trim(),
        node_ids: selectedNodeIds.length ? selectedNodeIds : undefined,
      })
      setProbeResult(res)
    } catch (err) {
      setProbeError(err instanceof Error ? err.message : "拨测失败")
    } finally {
      setProbeLoading(false)
    }
  }

  return (
    <>
    <section className="relative bg-muted/30 border-b">
      {/* 顶部大类 tabs — 胶囊分段控件 */}
      <div className="border-b">
        <div className="mx-auto max-w-screen-xl px-4 md:px-6 py-4">
          <div className="inline-flex items-center gap-0.5 rounded-lg bg-muted/60 border border-border/50 p-0.5">
            {CATEGORIES.map(c => (
              <button
                key={c.key}
                onClick={() => handleCatChange(c.key)}
                className={cn(
                  "px-3.5 py-1.5 text-xs font-medium rounded-md transition-all duration-150 whitespace-nowrap",
                  activeCat === c.key
                    ? "bg-background text-foreground shadow-sm"
                    : "text-muted-foreground hover:text-foreground"
                )}
              >
                {c.label}
              </button>
            ))}
          </div>
        </div>
      </div>

      {/* 主体内容 */}
      <div className="mx-auto max-w-screen-xl px-4 md:px-6 py-6 md:py-10">
        {/* 标题区 */}
        <div className="text-center mb-6 md:mb-8">
          <h2 className="text-xl md:text-2xl font-bold text-foreground">{cat.title}</h2>
          <p className="mt-2 text-sm text-primary font-medium">{cat.subtitle}</p>
          <p className="mt-1.5 text-sm text-muted-foreground max-w-xl mx-auto hidden md:block">{cat.desc}</p>
        </div>

        {/* 工具子 tabs */}
        {cat.tools.length > 0 && (
          <div className="flex items-center gap-0 mb-4 overflow-x-auto scrollbar-hide">
            {cat.tools.map(t => (
              <button
                key={t.key}
                onClick={() => setActiveTool(t.key)}
                className={cn(
                  "px-4 md:px-5 py-2 text-sm font-medium transition-colors border-b-2 whitespace-nowrap flex-shrink-0",
                  activeTool === t.key
                    ? "text-primary border-primary"
                    : "text-muted-foreground border-transparent hover:text-foreground"
                )}
              >
                {t.label}
              </button>
            ))}
          </div>
        )}

        {/* 输入行 — 桌面横排，移动端纵排 */}
        <div className="flex flex-col md:flex-row gap-0 rounded-md border border-border overflow-visible bg-background shadow-sm">
          {/* 节点选择器 */}
          {cat.showNodeSelector && (
            <div className="w-full md:w-56 md:flex-shrink-0 border-b md:border-b-0 md:border-r">
              <NodeSelectorTrigger
                selectedIds={selectedNodeIds}
                onConfirm={setSelectedNodeIds}
              />
            </div>
          )}

          {/* 输入框 + CTA */}
          <div className="flex flex-1 items-center gap-2 px-4 min-h-[44px]">
            <Search className="h-4 w-4 text-muted-foreground flex-shrink-0" />
            <input
              type="text"
              value={query}
              onChange={e => setQuery(e.target.value)}
              onKeyDown={e => e.key === "Enter" && handleGo()}
              placeholder={cat.inputPlaceholder}
              className="flex-1 h-11 bg-transparent text-sm outline-none placeholder:text-muted-foreground/60"
            />
          </div>

          {/* CTA */}
          <Button
            onClick={handleGo}
            disabled={!query.trim() || probeLoading}
            className="h-11 px-6 text-sm font-medium w-full md:w-auto rounded-none rounded-b-md md:rounded-r-md"
          >
            {probeLoading ? "检测中..." : "立即检测"}
            {!probeLoading && <ArrowRight className="h-4 w-4 ml-1" />}
          </Button>
        </div>

        {/* 底部辅助行 */}
        <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-2 mt-3 px-0.5">
          <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
            <Settings2 className="h-3.5 w-3.5 flex-shrink-0" />
            <span>高级配置</span>
            <span className="mx-2 opacity-30">|</span>
            <span>
              <a href="/app/monitors/new" className="text-primary hover:underline">
                创建定时监测任务
              </a>
              ，持续监测不同地区用户到站点的访问连通性
            </span>
          </div>
          <a href="/tools/diagnose" className="text-xs text-primary hover:underline">
            对比检测
          </a>
        </div>
      </div>
    </section>

    {/* ── 拨测结果面板 ─────────────────────────────────────────── */}
    {(probeLoading || probeResult || probeError) && (
      <div className="border-b bg-background">
        {probeError ? (
          <div className="mx-auto max-w-screen-xl px-6 py-6 text-sm text-destructive">
            拨测失败：{probeError}
          </div>
        ) : probeResult ? (
          <ProbeResultPanel
            result={probeResult}
            target={query}
            probeType={activeTool}
            isLoading={probeLoading}
          />
        ) : (
          <div className="mx-auto max-w-screen-xl px-6 py-8 space-y-3">
            {[1, 2, 3].map(i => (
              <div key={i} className="h-12 bg-muted/50 animate-pulse rounded-md" />
            ))}
            <p className="text-xs text-muted-foreground">正在发起拨测，请稍候...</p>
          </div>
        )}
      </div>
    )}
    </>
  )
}
