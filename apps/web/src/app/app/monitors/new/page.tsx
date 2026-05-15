import type { Metadata } from "next"
import { NewMonitorClient } from "./new-monitor-client"

export const metadata: Metadata = {
  title: "新建监控 - idcd",
  description: "创建新的监控项目",
}

export default function NewMonitorPage() {
  return (
    <>
      <div>
        <h1 className="text-2xl font-bold tracking-tight">新建监控</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          按步骤配置您的监控项目
        </p>
      </div>
      <NewMonitorClient />
    </>
  )
}