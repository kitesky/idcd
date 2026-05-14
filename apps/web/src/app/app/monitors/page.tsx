import type { Metadata } from "next"
import { MonitorsClient } from "./monitors-client"
import { MOCK_MONITORS } from "./mock-data"

export const metadata: Metadata = {
  title: "监控控制台 - idcd",
  description: "管理和查看您的所有监控项目，实时掌握目标可用性状态",
}

export default function MonitorsPage() {
  return (
    <div className="min-h-screen bg-background">
      <div className="container mx-auto px-4 py-8">
        <div className="mb-8">
          <h1 className="text-3xl font-bold tracking-tight">监控控制台</h1>
          <p className="mt-2 text-muted-foreground">
            管理和查看您的所有监控项目，实时掌握目标可用性状态
          </p>
        </div>

        <MonitorsClient initialMonitors={MOCK_MONITORS} />
      </div>
    </div>
  )
}
