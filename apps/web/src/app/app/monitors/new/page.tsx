import type { Metadata } from "next"
import { NewMonitorClient } from "./new-monitor-client"

export const metadata: Metadata = {
  title: "新建监控 - idcd",
  description: "创建新的监控项目",
}

export default function NewMonitorPage() {
  return (
    <div className="min-h-screen bg-background">
      <div className="container mx-auto px-4 py-8">
        <div className="mb-8">
          <h1 className="text-3xl font-bold tracking-tight">新建监控</h1>
          <p className="mt-2 text-muted-foreground">
            按步骤配置您的监控项目
          </p>
        </div>

        <NewMonitorClient />
      </div>
    </div>
  )
}
