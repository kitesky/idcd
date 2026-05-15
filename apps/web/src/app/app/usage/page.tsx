import type { Metadata } from "next"
import { UsageClient } from "./usage-client"

export const metadata: Metadata = {
  title: "用量统计 - idcd 控制台",
  description: "查看您的监控项和 API 调用用量统计",
}

export default function UsagePage() {
  return (
    <>
      <div>
        <h1 className="text-2xl font-bold tracking-tight">用量统计</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          查看您的资源使用情况和 API 调用趋势
        </p>
      </div>
      <UsageClient />
    </>
  )
}