import type { Metadata } from "next"
import { notFound, redirect } from "next/navigation"
import { getReport, isSingleReport } from "@/lib/diagnose-store"
import ComboReportView from "@/components/report/combo-report-view"

type Props = {
  params: Promise<{ id: string }>
}

// Legacy share path. Combo reports render in place to keep existing links live; single-type reports redirect to canonical /r/[id].
export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { id } = await params
  const report = await getReport(id)
  const domain = report && !isSingleReport(report) ? report.domain : "未知域名"
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
  const report = await getReport(id)
  if (!report) notFound()

  if (isSingleReport(report)) {
    redirect(`/r/${id}`)
  }

  return <ComboReportView report={report} />
}
