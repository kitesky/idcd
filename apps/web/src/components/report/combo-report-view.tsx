import Link from "next/link"
import { ArrowLeft, CheckCircle2, XCircle } from "lucide-react"
import { Card, CardContent, CardHeader, CardTitle, Badge } from "@/components/ui"
import ShareLinkButton from "./share-link-button"
import type { DiagnoseReport, CheckResult } from "@/lib/diagnose-store"

/**
 * Renders the 7-in-1 comprehensive diagnose report. Used by both
 * `/report/[id]` (legacy) and `/r/[id]` (new canonical) so the two routes
 * stay in lock-step.
 */
export default function ComboReportView({ report }: { report: DiagnoseReport }) {
  return (
    <div className="container mx-auto px-4 py-8 max-w-4xl">
      <div className="space-y-6">
        <div>
          <Link
            href="/tools/diagnose"
            className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground mb-4 transition-colors"
          >
            <ArrowLeft className="h-4 w-4" />
            返回诊断工具
          </Link>
          <h1 className="text-3xl font-bold break-all">{report.domain}</h1>
          <p className="text-muted-foreground mt-1">
            诊断报告 · {new Date(report.createdAt).toLocaleString("zh-CN")}
          </p>
        </div>

        <div className="grid grid-cols-3 gap-4">
          <Card>
            <CardContent className="pt-6">
              <div className="text-2xl font-bold text-green-500">{report.doneCount}</div>
              <p className="text-xs text-muted-foreground mt-1">检测通过</p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6">
              <div className="text-2xl font-bold text-red-500">{report.errorCount}</div>
              <p className="text-xs text-muted-foreground mt-1">检测失败</p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6">
              <div className="text-2xl font-bold">{report.checks.length}</div>
              <p className="text-xs text-muted-foreground mt-1">检测总项</p>
            </CardContent>
          </Card>
        </div>

        <Card>
          <CardHeader>
            <CardTitle>检测详情</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            {report.checks.map((check) => (
              <CheckCard key={check.key} check={check} />
            ))}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>分享报告</CardTitle>
          </CardHeader>
          <CardContent>
            <ShareLinkButton />
          </CardContent>
        </Card>
      </div>
    </div>
  )
}

function CheckCard({ check }: { check: CheckResult }) {
  const isSuccess = check.status === "done"

  return (
    <Card>
      <CardContent className="pt-4 flex items-start gap-3">
        <div className="mt-0.5 shrink-0">
          {isSuccess ? (
            <CheckCircle2 className="h-5 w-5 text-green-500" />
          ) : (
            <XCircle className="h-5 w-5 text-red-500" />
          )}
        </div>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="font-medium">{check.label}</span>
            <Badge
              variant={isSuccess ? "success" : "destructive"}
              className="ml-auto text-xs"
            >
              {isSuccess ? "通过" : "失败"}
            </Badge>
          </div>
          {check.summary && (
            <p className="text-sm text-muted-foreground mt-1">{check.summary}</p>
          )}
          {check.detail && Object.keys(check.detail).length > 0 && (
            <details className="mt-2">
              <summary className="text-xs text-muted-foreground cursor-pointer hover:text-foreground select-none">
                查看详细数据
              </summary>
              <pre className="text-xs bg-muted rounded-md p-2 mt-1 overflow-x-auto whitespace-pre-wrap break-all">
                {JSON.stringify(check.detail, null, 2)}
              </pre>
            </details>
          )}
          {check.error && (
            <p className="text-sm text-red-500 mt-1">{check.error}</p>
          )}
        </div>
      </CardContent>
    </Card>
  )
}
