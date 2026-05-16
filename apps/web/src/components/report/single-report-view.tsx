import Link from "next/link"
import { ArrowLeft } from "lucide-react"
import { Card, CardContent, CardHeader, CardTitle, Badge } from "@/components/ui"
import ProbeResults from "@/components/probe/ProbeResults"
import ShareLinkButton from "./share-link-button"
import type { SingleProbeReport } from "@/lib/diagnose-store"
import type { ProbeResult } from "@/lib/api"

const TOOL_LABEL: Record<SingleProbeReport["tool"], string> = {
  ping: "Ping 拨测",
  http: "HTTP / HTTPS 拨测",
  dns: "DNS 解析",
  traceroute: "路由追踪",
}

const TOOL_BACK_HREF: Record<SingleProbeReport["tool"], string> = {
  ping: "/tools/ping",
  http: "/tools/http",
  dns: "/tools/dns",
  traceroute: "/tools/traceroute",
}

/**
 * Re-shape a saved single-tool snapshot back into a ProbeResult so the
 * existing <ProbeResults> table (which already knows how to render a result
 * row) can display it without polling.
 */
function toLegacyProbeResult(report: SingleProbeReport): ProbeResult {
  const r = report.result
  if (!r) {
    return { task_id: report.taskId, status: report.status, results: [] }
  }
  const knownKeys = new Set(["node_id", "success", "duration_ms", "error"])
  const details: Record<string, unknown> = Object.fromEntries(
    Object.entries(r).filter(([k]) => !knownKeys.has(k)),
  )
  return {
    task_id: report.taskId,
    status: report.status,
    results: [
      {
        node_id: r.node_id ?? "unknown",
        node_name: "",
        success: r.success ?? false,
        latency_ms: r.duration_ms,
        error: r.error,
        details: Object.keys(details).length > 0 ? details : undefined,
      },
    ],
  }
}

export default function SingleReportView({ report }: { report: SingleProbeReport }) {
  const probeResult = toLegacyProbeResult(report)
  const toolLabel = TOOL_LABEL[report.tool]
  const backHref = TOOL_BACK_HREF[report.tool]

  return (
    <div className="container mx-auto px-4 py-8 max-w-4xl">
      <div className="space-y-6">
        <div>
          <Link
            href={backHref}
            className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground mb-4 transition-colors"
          >
            <ArrowLeft className="h-4 w-4" />
            返回 {toolLabel}
          </Link>
          <div className="flex items-center gap-3 flex-wrap">
            <h1 className="text-3xl font-bold break-all">{report.target}</h1>
            <Badge variant="outline">{toolLabel}</Badge>
          </div>
          <p className="text-muted-foreground mt-1">
            拨测快照 · {new Date(report.createdAt).toLocaleString("zh-CN")}
          </p>
        </div>

        <ProbeResults result={probeResult} />

        {report.params && Object.keys(report.params).length > 0 && (
          <Card>
            <CardHeader>
              <CardTitle>拨测参数</CardTitle>
            </CardHeader>
            <CardContent>
              <pre className="text-xs bg-muted rounded-md p-3 overflow-x-auto whitespace-pre-wrap break-all">
                {JSON.stringify(report.params, null, 2)}
              </pre>
            </CardContent>
          </Card>
        )}

        <Card>
          <CardHeader>
            <CardTitle>分享报告</CardTitle>
          </CardHeader>
          <CardContent>
            <ShareLinkButton />
            <p className="text-xs text-muted-foreground mt-3">
              报告 7 天内有效。需要永久存证？请前往{" "}
              <Link href={backHref} className="text-primary underline underline-offset-4">
                工具页
              </Link>{" "}
              重新生成。
            </p>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
