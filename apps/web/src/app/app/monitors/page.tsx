import type { Metadata } from "next"
import { MonitorsClient } from "./monitors-client"

export const metadata: Metadata = {
  title: "监控控制台 - idcd",
  description: "管理和查看您的所有监控项目，实时掌握目标可用性状态",
}

export default function MonitorsPage() {
  return (
    <>
      <div>
        <h1 className="text-2xl font-bold tracking-tight">监控控制台</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          管理和查看您的所有监控项目，实时掌握目标可用性状态
        </p>
      </div>
      <MonitorsClient />
    </>
  )
}