import type { Metadata } from "next"
import { StatusPagesClient } from "./status-pages-client"

export const metadata: Metadata = {
  title: "状态页管理 - idcd 控制台",
  description: "管理您的公开服务状态页",
}

export default function StatusPagesPage() {
  return (
    <div className="min-h-screen bg-background">
      <div className="container mx-auto px-4 py-8">
        <div className="mb-8">
          <h1 className="text-3xl font-bold tracking-tight">状态页管理</h1>
          <p className="mt-2 text-muted-foreground">
            创建和管理对外公开的服务状态页
          </p>
        </div>
        <StatusPagesClient />
      </div>
    </div>
  )
}
