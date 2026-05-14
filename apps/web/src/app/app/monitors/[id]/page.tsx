import type { Metadata } from "next"
import { notFound } from "next/navigation"
import { MonitorDetailClient } from "./monitor-detail-client"
import { MOCK_MONITORS } from "../types"

type Props = {
  params: Promise<{ id: string }>
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { id } = await params
  const monitor = MOCK_MONITORS.find((m) => m.id === id)
  if (!monitor) {
    return { title: "监控详情 - idcd" }
  }
  return {
    title: `${monitor.name} - 监控详情 - idcd`,
    description: `查看 ${monitor.name} 的实时状态和历史趋势`,
  }
}

export default async function MonitorDetailPage({ params }: Props) {
  const { id } = await params
  const monitor = MOCK_MONITORS.find((m) => m.id === id)

  if (!monitor) {
    notFound()
  }

  return (
    <div className="min-h-screen bg-background">
      <div className="container mx-auto px-4 py-8">
        <MonitorDetailClient monitor={monitor} />
      </div>
    </div>
  )
}
