import type { Metadata } from "next"
import { notFound, redirect } from "next/navigation"
import { getReport, isSingleReport } from "@/lib/diagnose-store"
import ComboReportView from "@/components/report/combo-report-view"

type Props = {
  params: Promise<{ id: string }>
}

/**
 * Legacy share path. Kept alive for backward compat — share URLs from
 * before May 2026 land here. New share URLs all use /r/[id], which handles
 * both combo and single-tool reports.
 *
 * For combo reports we render in place (no redirect — keeps any external
 * links pointing here intact). For single-tool reports (which technically
 * shouldn't reach this route via current code paths but might in the
 * future) we redirect to the canonical /r/[id].
 */

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
