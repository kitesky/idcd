import type { Metadata } from "next"
import Link from "next/link"
import { ALL_TOOLS } from "./tools-config"

export const metadata: Metadata = {
  title: "网络诊断工具箱 | idcd",
  description: "idcd 提供 50+ 网络诊断和开发工具，覆盖 HTTP/Ping/DNS/SSL/路由追踪等常用场景。",
}

const probeTools = ALL_TOOLS.filter((t) => t.category === "probe")
const utilityTools = ALL_TOOLS.filter((t) => t.category === "utility")

export default function ToolsPage() {
  return (
    <div className="mx-auto max-w-7xl px-4 py-12 sm:px-6 lg:px-8">
      <div className="mb-10">
        <h1 className="text-3xl font-bold tracking-tight">网络诊断工具箱</h1>
        <p className="mt-3 text-muted-foreground">
          {ALL_TOOLS.length}+ 工具，从全球多个节点检测网络可用性和性能
        </p>
      </div>

      <section className="mb-12">
        <h2 className="mb-6 text-xl font-semibold">拨测工具</h2>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {probeTools.map((tool) => (
            <Link
              key={tool.slug}
              href={`/tools/${tool.slug}` as any}
              className="group rounded-lg border bg-card p-5 hover:border-primary hover:shadow-sm transition-all"
            >
              <h3 className="font-medium group-hover:text-primary transition-colors">
                {tool.name}
              </h3>
              <p className="mt-1.5 text-sm text-muted-foreground line-clamp-2">
                {tool.description}
              </p>
            </Link>
          ))}
        </div>
      </section>

      <section>
        <h2 className="mb-6 text-xl font-semibold">辅助工具</h2>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {utilityTools.map((tool) => (
            <Link
              key={tool.slug}
              href={`/tools/${tool.slug}` as any}
              className="group rounded-lg border bg-card p-5 hover:border-primary hover:shadow-sm transition-all"
            >
              <h3 className="font-medium group-hover:text-primary transition-colors">
                {tool.name}
              </h3>
              <p className="mt-1.5 text-sm text-muted-foreground line-clamp-2">
                {tool.description}
              </p>
            </Link>
          ))}
        </div>
      </section>
    </div>
  )
}
