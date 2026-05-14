import type { Metadata } from "next"
import { notFound } from "next/navigation"
import Link from "next/link"
import { Card, CardContent, CardHeader, CardTitle, Badge } from "@/components/ui"
import { CheckCircle2, XCircle, ArrowLeft } from "lucide-react"
import { getReport } from "@/lib/diagnose-store"
import type { CheckResult } from "@/lib/diagnose-store"
import ShareButton from "./share-button"

type Props = {
  params: Promise<{ id: string }>
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { id } = await params
  const report = getReport(id)
  const domain = report?.domain ?? "未知域名"
  const date = report
    ? new Date(report.createdAt).toLocaleDateString("zh-CN")
    : ""

  return {
    title: `${domain} 诊断报告 - idcd`,
    description: `${domain} 的完整网络诊断报告，包含 DNS/HTTP/Ping/Traceroute/SSL/ICP 备案/WHOIS 七项检测结果`,
    openGraph: {
      title: `${domain} 一键诊断报告`,
      description: `idcd 网络诊断 · ${date} · DNS / HTTP / Ping / Traceroute / SSL / ICP / WHOIS`,
      type: "article",
    },
  }
}

export default async function ReportPage({ params }: Props) {
  const { id } = await params
  const report = getReport(id)

  if (!report) {
    notFound()
  }

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
            <ShareButton />
          </CardContent>
        </Card>
      </div>
    </div>
  )
}

function CheckCard({ check }: { check: CheckResult }) {
  const isSuccess = check.status === "done"

  return (
    <div className="flex items-start gap-3 p-3 rounded-lg border bg-card">
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
    </div>
  )
}
