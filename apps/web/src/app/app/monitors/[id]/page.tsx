import type { Metadata } from "next"
import { getT } from "@/i18n/getT"
import { MonitorDetailClient } from "./monitor-detail-client"
import type { Monitor, MonitorType } from "../types"
import { API_BASE, API_CREDENTIALS_POLICY } from "@/lib/api"

type Props = {
  params: Promise<{ id: string }>
}

interface RawApiMonitor {
  id: string
  name: string
  type: string
  target: string
  config: Record<string, unknown> | null
  interval_s: number
  node_count: number
  status: string
  uptime_percent: number
  last_check_at: string | null
}

function mapStatus(s: string): Monitor["status"] {
  switch (s) {
    case "active": return "UP"
    case "down":   return "DOWN"
    case "paused": return "PAUSED"
    case "degraded": return "degraded"
    default: return "UP"
  }
}

function mapApiMonitor(raw: RawApiMonitor): Monitor {
  const cfg = raw.config && typeof raw.config === "object" ? raw.config : {}
  return {
    id: raw.id,
    name: raw.name,
    type: raw.type as MonitorType,
    target: raw.target,
    status: mapStatus(raw.status),
    uptimePercent: raw.uptime_percent ?? 0,
    lastCheckedAt: raw.last_check_at ?? new Date().toISOString(),
    intervalSeconds: raw.interval_s ?? 300,
    concurrentNodes: raw.node_count ?? 1,
    timeoutMs: cfg.timeout_ms as number | undefined,
    assertStatusCode: cfg.assert_status_code as number | undefined,
    keywordMatch: (cfg.keyword ?? cfg.keyword_match) as string | undefined,
    packetLossThreshold: cfg.packet_loss_threshold as number | undefined,
    port: cfg.port as number | undefined,
    expectedIp: cfg.expected_ip as string | undefined,
    sslExpiryDays: (cfg.expiry_warning_days ?? cfg.ssl_expiry_days) as number | undefined,
  }
}

async function fetchMonitor(id: string): Promise<Monitor | null> {
  try {
    const res = await fetch(`${API_BASE}/v1/monitors/${id}`, {
      next: { revalidate: 0 },
      credentials: API_CREDENTIALS_POLICY,
    })
    if (!res.ok) return null
    const body = await res.json()
    const raw = body?.data
    if (!raw) return null
    return mapApiMonitor(raw as RawApiMonitor)
  } catch {
    return null
  }
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { id } = await params
  const monitor = await fetchMonitor(id)
  const t = await getT("monitors")
  if (!monitor) return { title: `${t("title")} - idcd` }
  const title = `${monitor.name} - ${t("title")} - idcd`
  const desc = monitor.name
  return {
    title,
    description: desc,
    openGraph: { title, description: desc, type: "website" },
    twitter: { card: "summary", title, description: desc },
  }
}

export default async function MonitorDetailPage({ params }: Props) {
  const { id } = await params
  const monitor = await fetchMonitor(id)

  return <MonitorDetailClient monitor={monitor} monitorId={id} />
}
