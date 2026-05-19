"use client"

import { useState, useEffect } from "react"
import Link from "next/link"
import { ChevronDown, Search, Settings2, ArrowRight, MapPin, RotateCcw } from "lucide-react"
import { useTranslations } from "next-intl"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Popover, PopoverTrigger, PopoverContent } from "@/components/ui/popover"
import { Checkbox } from "@/components/ui/checkbox"
import { cn } from "@/lib/utils"
import { getNodes, probeHttp, probePing, probeDns, probeTcp, probeMtr, probeTraceroute } from "@/lib/api"
import type { Node as ProbeNode } from "@/lib/api"
import { useProbePolling } from "@/hooks/useProbePolling"
import ProbeResults from "@/components/probe/ProbeResults"

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

interface NodePanelBodyProps {
  selectedIds: string[]
  onConfirm: (ids: string[]) => void
  onClose: () => void
}

// NodePanelBody — content rendered inside the shadcn Popover. Stays as a
// separate component so the popover trigger doesn't pay the cost of
// `getNodes` + reduce groupings until it's actually opened.
function NodePanelBody({ selectedIds, onConfirm, onClose }: NodePanelBodyProps) {
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
    <div className="w-[480px] max-w-[calc(100vw-2rem)]">
      {/* 中国 / 全球 tabs — shadcn Tabs */}
      <Tabs value={tab} onValueChange={(v) => setTab(v as "cn" | "global")}>
        <TabsList className="grid w-full grid-cols-2 rounded-none border-b bg-transparent h-auto p-0">
          <TabsTrigger
            value="cn"
            className="rounded-none border-b-2 border-transparent data-[state=active]:border-primary data-[state=active]:bg-muted/30 data-[state=active]:shadow-none py-2.5"
          >
            {t("nodePanelCn")}
          </TabsTrigger>
          <TabsTrigger
            value="global"
            className="rounded-none border-b-2 border-transparent data-[state=active]:border-primary data-[state=active]:bg-muted/30 data-[state=active]:shadow-none py-2.5"
          >
            {t("nodePanelGlobal")}
          </TabsTrigger>
        </TabsList>
      </Tabs>

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
              const someSelected = groupNodes.some(n => pending.includes(n.id))
              return (
                <div key={group}>
                  <label className="flex items-center gap-1.5 mb-2 cursor-pointer select-none">
                    <Checkbox
                      checked={allSelected ? true : someSelected ? "indeterminate" : false}
                      onCheckedChange={() => toggleGroup(groupNodes)}
                    />
                    <span className="text-xs font-medium text-muted-foreground">{group}</span>
                  </label>
                  <div className="flex flex-wrap gap-2 pl-6">
                    {groupNodes.map(node => {
                      const selected = pending.includes(node.id)
                      return (
                        <label
                          key={node.id}
                          className="flex items-center gap-1.5 cursor-pointer select-none"
                        >
                          <Checkbox
                            checked={selected}
                            onCheckedChange={() => toggleNode(node.id)}
                          />
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
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={reset}
            className="h-7 px-2 text-xs text-muted-foreground hover:text-foreground"
          >
            <RotateCcw className="h-3 w-3" aria-hidden="true" />
            {t("nodePanelReset")}
          </Button>
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

  const label = selectedIds.length === 0
    ? t("nodeSelectorPlaceholder")
    : t("nodeSelectorSelected", { count: selectedIds.length })

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          variant="ghost"
          className="flex h-11 w-full items-center justify-between gap-2 px-4 text-sm font-normal text-foreground/80 hover:text-foreground rounded-none"
        >
          <span className="flex items-center gap-2 min-w-0">
            <MapPin className="h-3.5 w-3.5 text-muted-foreground flex-shrink-0" aria-hidden="true" />
            <span className={cn("truncate", selectedIds.length === 0 && "text-muted-foreground")}>
              {label}
            </span>
          </span>
          <ChevronDown
            className={cn("h-4 w-4 flex-shrink-0 text-muted-foreground transition-transform duration-150", open && "rotate-180")}
            aria-hidden="true"
          />
        </Button>
      </PopoverTrigger>
      <PopoverContent align="start" className="p-0 w-auto" sideOffset={4}>
        <NodePanelBody
          selectedIds={selectedIds}
          onConfirm={onConfirm}
          onClose={() => setOpen(false)}
        />
      </PopoverContent>
    </Popover>
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
  // taskId + polling replaces the old "probeResult: ProbeResult" model: the
  // backend returns only `{ task_id, status:"queued" }` synchronously, so
  // pretending the immediate response had results was the root cause of the
  // hero panel showing nothing after submit. Mirrors the (public)/tools/*
  // pages so we share the same result rendering path.
  const [taskId, setTaskId] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [submitError, setSubmitError] = useState("")
  const polling = useProbePolling(taskId)

  const catConfig = CATEGORY_CONFIGS.find(c => c.key === activeCat) ?? CATEGORY_CONFIGS[1]!

  const resetResult = () => {
    setTaskId(null)
    setSubmitError("")
  }

  const handleCatChange = (key: string) => {
    setActiveCat(key)
    resetResult()
    const c = CATEGORY_CONFIGS.find(c => c.key === key)
    if (c?.tools.length) setActiveTool(c.tools[0]!.key)
  }

  const handleGo = async () => {
    if (!query.trim()) return
    setSubmitError("")
    setTaskId(null)
    setSubmitting(true)

    try {
      const fn = PROBE_FN[activeTool] ?? probeHttp
      const res = await fn({
        target: query.trim(),
        node_ids: selectedNodeIds.length ? selectedNodeIds : undefined,
      })
      setTaskId(res.task_id)
    } catch (err) {
      setSubmitError(err instanceof Error ? err.message : t("hero.probeError", { message: "" }))
    } finally {
      setSubmitting(false)
    }
  }

  const probeLoading = submitting || polling.isPolling
  const hasResult = polling.taskResult !== null
  const probeError = submitError || polling.error

  return (
    <>
    <section className="relative bg-muted/30 border-b">
      {/* 顶部大类 tabs — shadcn Tabs (受控) */}
      <div className="border-b">
        <div className="mx-auto max-w-screen-xl px-4 md:px-6 py-4">
          <Tabs value={activeCat} onValueChange={handleCatChange}>
            <TabsList>
              {CATEGORY_CONFIGS.map(c => (
                <TabsTrigger key={c.key} value={c.key}>
                  {t(c.labelKey as never)}
                </TabsTrigger>
              ))}
            </TabsList>
          </Tabs>
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

        {/* 工具子 tabs — shadcn Tabs (受控，下划线风格通过 className 覆盖) */}
        {catConfig.tools.length > 0 && (
          <Tabs value={activeTool} onValueChange={setActiveTool} className="mb-4">
            <TabsList className="bg-transparent h-auto p-0 gap-0 justify-start overflow-x-auto scrollbar-hide w-full">
              {catConfig.tools.map(toolTab => (
                <TabsTrigger
                  key={toolTab.key}
                  value={toolTab.key}
                  className={cn(
                    "px-4 md:px-5 py-2 text-sm font-medium rounded-none border-b-2 border-transparent",
                    "data-[state=active]:bg-transparent data-[state=active]:shadow-none",
                    "data-[state=active]:text-primary data-[state=active]:border-primary",
                    "text-muted-foreground hover:text-foreground"
                  )}
                >
                  {toolTab.label}
                </TabsTrigger>
              ))}
            </TabsList>
          </Tabs>
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
          </div>
          <Link href="/tools/diagnose" className="text-xs text-primary underline underline-offset-4">
            {t("hero.compareCheck")}
          </Link>
        </div>
      </div>
    </section>

    {/* ── 拨测结果面板 ─────────────────────────────────────────── */}
    {(probeLoading || hasResult || probeError) && (
      <div className="border-b bg-background">
        <div className="mx-auto max-w-screen-xl px-4 md:px-6 py-6">
          <ProbeResults
            taskId={taskId}
            polling={polling}
            loading={submitting}
            error={submitError}
          />
        </div>
      </div>
    )}
    </>
  )
}
