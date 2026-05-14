import { Metadata } from "next"
import { AlertsClient } from "./alerts-client"

export const metadata: Metadata = {
  title: "告警管理 - idcd",
  description: "管理告警事件、通知通道和告警策略",
}

export default function AlertsPage() {
  return (
    <div className="min-h-screen bg-background">
      <div className="container mx-auto px-4 py-8">
        <div className="mb-8">
          <h1 className="text-3xl font-bold tracking-tight">告警管理</h1>
          <p className="mt-2 text-muted-foreground">
            查看告警事件历史，管理通知通道与告警策略
          </p>
        </div>

        <AlertsClient />
      </div>
    </div>
  )
}
