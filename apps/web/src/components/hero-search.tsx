"use client"

import { useState, useEffect, useRef } from "react"
import Link from "next/link"
import { ChevronDown, Search, Settings2, ArrowRight, MapPin, RotateCcw, Check } from "lucide-react"
import { useTranslations } from "next-intl"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
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

interface CategoryConfig {
  key: string
  labelKey: string
  titleKey: string
  subtitleKey: string
  descKey: string
  tools: ToolTab[]
  placeholderKey: string
  showNodeSelector: boolean
}

const CATEGORY_CONFIGS: CategoryConfig[] = [
  {
    key: "diagnose",
    labelKey: "categories.diagnose.label",
    titleKey: "categories.diagnose.title",
    subtitleKey: "categories.diagnose.subtitle",
    descKey: "categories.diagnose.desc",
    tools: [],
    placeholderKey: "categories.diagnose.placeholder",
    showNodeSelector: false,
  },
  {
    key: "probe",
    labelKey: "categories.probe.label",
    titleKey: "categories.probe.title",
    subtitleKey: "categories.probe.subtitle",
    descKey: "categories.probe.desc",
    tools: [
      { key: "http",       label: "HTTP",       path: "/tools/http" },
      { key: "ping",       label: "Ping",       path: "/tools/ping" },
      { key: "dns",        label: "DNS",        path: "/tools/dns" },
      { key: "tcp",        label: "TCP",        path: "/tools/tcp" },
      { key: "mtr",        label: "MTR",        path: "/tools/mtr" },
      { key: "traceroute", label: "Traceroute", path: "/tools/traceroute" },
    ],
    placeholderKey: "categories.probe.placeholder",
    showNodeSelector: true,
  },
  {
    key: "domain",
    labelKey: "categories.domain.label",
    titleKey: "categories.domain.title",
    subtitleKey: "categories.domain.subtitle",
    descKey: "categories.domain.desc",
    tools: [
      { key: "whois", label: "WHOIS",  path: "/tools/whois" },
      { key: "icp",   label: "ICP 备案", path: "/tools/icp" },
      { key: "ssl",   label: "SSL 证书", path: "/tools/ssl" },
      { key: "dns",   label: "DNS 解析", path: "/tools/dns" },
    ],
    placeholderKey: "categories.domain.placeholder",
    showNodeSelector: false,
  },
]

// ── NodePanel ─────────────────────────────────────────────────────────────────

interface NodePanelProps {
  selectedIds: string[]
  onConfirm: (ids: string[]) => void
  onClose: () => void
}

function NodePanel({ selectedIds, onConfirm, onClose }: NodePanelProps) {
  const t = useTranslations("home")
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

  return (
    <div className="absolute top-full left-0 z-50 mt-1 w-[480px] max-w-[calc(100vw-2rem)] bg-popover border rounded-lg shadow-xl overflow-hidden">
      {/* 中国 / 全球 tabs */}
      <div className="flex border-b">
        {(["cn", "global"] as const).map(tabKey => (
          <button
            key={tabKey}
            type="button"
            onClick={() => setTab(tabKey)}
            className={cn(
              "flex-1 py-2.5 text-sm font-medium transition-colors",
              tab === tabKey
                ? "text-primary border-b-2 border-primary bg-muted/30"
                : "text-muted-foreground hover:text-foreground"
            )}
          >
            {tabKey === "cn" ? t("nodePanelCn") : t("nodePanelGlobal")}
          </button>
        ))}
      </div>

      {/* 节点列表 */}
      <div className="max-h-72 overflow-y-auto p-4">
        {loading ? (
          <p className="text-sm text-muted-foreground text-center py-6">{t("nodePanelLoading")}</p>
        ) : displayed.length === 0 ? (
          <p className="text-sm text-muted-foreground text-center py-6">{t("nodePanelEmpty")}</p>
        ) : (
          <div className="space-y-4">
            {Object.entries(grouped).map(([group, groupNodes]) => {
              const allSelected = groupNodes.every(n => pending.includes(n.id))
              return (
                <div key={group}>
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
          {t("nodePanelSelected", { count: pending.length })}
        </span>
        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={reset}
            className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors"
          >
            <RotateCcw className="h-3 w-3" />
            {t("nodePanelReset")}
          </button>
          <Button size="sm" className="h-7 px-4 text-xs" onClick={() => { onConfirm(pending); onClose() }}>
            {t("nodePanelConfirm")}
          </Button>
        </div>
      </div>
    </div>
  )
}

// ── NodeSelectorTrigger ──────────────────────────────────────────────────────

function NodeSelectorTrigger({ selectedIds, onConfirm }: { selectedIds: string[]; onConfirm: (ids: string[]) => void }) {
  const t = useTranslations("home")
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
    ? t("nodeSelectorPlaceholder")
    : t("nodeSelectorSelected", { count: selectedIds.length })

  return (
    <div ref={containerRef} className="relative w-full">
      <button
        type="button"
        onClick={() => setOpen(v => !v)}
        className="flex h-11 w-full items-center justify-between gap-2 px-4 text-sm text-foreground/80 hover:text-foreground transition-colors"
      >
        <div className="flex items-center gap-2 min-w-0">
          <MapPin className="h-3.5 w-3.5 text-muted-foreground flex-shrink-0" />
          <span className={cn("truncate", selectedIds.length === 0 && "text-muted-foreground")}>
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
  const t = useTranslations("home")
  const [activeCat, setActiveCat] = useState("probe")
  const [activeTool, setActiveTool] = useState("http")
  const [selectedNodeIds, setSelectedNodeIds] = useState<string[]>([])
  const [query, setQuery] = useState("")
  const [probeResult, setProbeResult] = useState<ProbeResult | null>(null)
  const [probeLoading, setProbeLoading] = useState(false)
  const [probeError, setProbeError] = useState("")

  const catConfig = CATEGORY_CONFIGS.find(c => c.key === activeCat) ?? CATEGORY_CONFIGS[1]!

  const handleCatChange = (key: string) => {
    setActiveCat(key)
    setProbeResult(null)
    const c = CATEGORY_CONFIGS.find(c => c.key === key)
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
      setProbeError(err instanceof Error ? err.message : t("hero.probeError", { message: "" }))
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
            {CATEGORY_CONFIGS.map(c => (
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
                {t(c.labelKey as never)}
              </button>
            ))}
          </div>
        </div>
      </div>

      {/* 主体内容 */}
      <div className="mx-auto max-w-screen-xl px-4 md:px-6 py-6 md:py-10">
        {/* 标题区 */}
        <div className="text-center mb-6 md:mb-8">
          <h2 className="text-xl md:text-2xl font-bold text-foreground">{t(catConfig.titleKey as never)}</h2>
          <p className="mt-2 text-sm text-primary font-medium">{t(catConfig.subtitleKey as never)}</p>
          <p className="mt-1.5 text-sm text-muted-foreground max-w-xl mx-auto hidden md:block">{t(catConfig.descKey as never)}</p>
        </div>

        {/* 工具子 tabs */}
        {catConfig.tools.length > 0 && (
          <div className="flex items-center gap-0 mb-4 overflow-x-auto scrollbar-hide">
            {catConfig.tools.map(toolTab => (
              <button
                key={toolTab.key}
                onClick={() => setActiveTool(toolTab.key)}
                className={cn(
                  "px-4 md:px-5 py-2 text-sm font-medium transition-colors border-b-2 whitespace-nowrap flex-shrink-0",
                  activeTool === toolTab.key
                    ? "text-primary border-primary"
                    : "text-muted-foreground border-transparent hover:text-foreground"
                )}
              >
                {toolTab.label}
              </button>
            ))}
          </div>
        )}

        {/* 输入行 — 桌面横排，移动端纵排 */}
        <div className="flex flex-col md:flex-row gap-0 rounded-md border border-border overflow-visible bg-background shadow-sm">
          {/* 节点选择器 */}
          {catConfig.showNodeSelector && (
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
            <Input
              type="text"
              value={query}
              onChange={e => setQuery(e.target.value)}
              onKeyDown={e => e.key === "Enter" && handleGo()}
              placeholder={t(catConfig.placeholderKey as never)}
              className="flex-1 h-11 border-0 bg-transparent text-sm shadow-none focus-visible:ring-0 placeholder:text-muted-foreground"
            />
          </div>

          {/* CTA */}
          <Button
            onClick={handleGo}
            disabled={!query.trim() || probeLoading}
            className="h-11 px-6 text-sm font-medium w-full md:w-auto rounded-none rounded-b-md md:rounded-r-md"
          >
            {probeLoading ? t("hero.detecting") : t("hero.cta")}
            {!probeLoading && <ArrowRight className="h-4 w-4 ml-1" />}
          </Button>
        </div>

        {/* 底部辅助行 */}
        <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-2 mt-3 px-0.5">
          <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
            <Settings2 className="h-3.5 w-3.5 flex-shrink-0" />
            <span>{t("hero.advancedConfig")}</span>
            <span className="mx-2 opacity-30">|</span>
            <span>
              <Link href="/app/monitors/new" className="text-primary underline underline-offset-4">
                {t("hero.createMonitor")}
              </Link>
              {t("hero.createMonitorDesc")}
            </span>
          </div>
          <Link href="/tools/diagnose" className="text-xs text-primary underline underline-offset-4">
            {t("hero.compareCheck")}
          </Link>
        </div>
      </div>
    </section>

    {/* ── 拨测结果面板 ─────────────────────────────────────────── */}
    {(probeLoading || probeResult || probeError) && (
      <div className="border-b bg-background">
        {probeError ? (
          <div className="mx-auto max-w-screen-xl px-6 py-6 text-sm text-destructive">
            {t("hero.probeError", { message: probeError })}
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
            <p className="text-xs text-muted-foreground">{t("hero.probePending")}</p>
          </div>
        )}
      </div>
    )}
    </>
  )
}
