import type { Metadata } from "next"
import { UsageClient } from "./usage-client"

export const metadata: Metadata = {
  title: "用量统计 - idcd 控制台",
  description: "查看您的监控项和 API 调用用量统计",
}

export default function UsagePage() {
  return (
    <div className="min-h-screen bg-background">
      <div className="container mx-auto px-4 py-8">
        <div className="mb-8">
          <h1 className="text-3xl font-bold tracking-tight">用量统计</h1>
          <p className="mt-2 text-muted-foreground">
            查看您的资源使用情况和 API 调用趋势
          </p>
        </div>
        <UsageClient />
      </div>
    </div>
  )
}
