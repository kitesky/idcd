import type { Metadata } from "next"
import { MonitorDetailClient } from "./monitor-detail-client"

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080"

type Props = {
  params: Promise<{ id: string }>
}

async function fetchMonitor(id: string) {
  try {
    const res = await fetch(`${API_BASE}/v1/monitors/${id}`, {
      next: { revalidate: 0 },
      credentials: "include",
    })
    if (!res.ok) return null
    const body = await res.json()
    return body?.data ?? null
  } catch {
    return null
  }
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { id } = await params
  const monitor = await fetchMonitor(id)
  if (!monitor) return { title: "监控详情 - idcd" }
  return {
    title: `${monitor.name} - 监控详情 - idcd`,
    description: `查看 ${monitor.name} 的实时状态和历史趋势`,
  }
}

export default async function MonitorDetailPage({ params }: Props) {
  const { id } = await params
  const monitor = await fetchMonitor(id)

  return <MonitorDetailClient monitor={monitor} monitorId={id} />
}
