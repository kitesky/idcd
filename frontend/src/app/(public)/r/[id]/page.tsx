import type { Metadata } from "next"
import { notFound } from "next/navigation"
import { getReport, isSingleReport } from "@/lib/diagnose-store"
import ComboReportView from "@/components/report/combo-report-view"
import SingleReportView from "@/components/report/single-report-view"

type Props = {
  params: Promise<{ id: string }>
}

const TOOL_LABEL_FOR_TITLE: Record<string, string> = {
  ping: "Ping",
  http: "HTTP",
  dns: "DNS",
  traceroute: "Traceroute",
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { id } = await params
  const report = await getReport(id)
  if (!report) {
    return {
      title: "报告未找到 - idcd",
      description: "该拨测报告已过期或不存在",
    }
  }

  if (isSingleReport(report)) {
    const toolName = TOOL_LABEL_FOR_TITLE[report.tool] ?? report.tool
    const date = new Date(report.createdAt).toLocaleDateString("zh-CN")
    return {
      title: `${report.target} ${toolName} 拨测结果 - idcd`,
      description: `${report.target} 的 ${toolName} 多节点拨测快照 · ${date}`,
      openGraph: {
        title: `${report.target} ${toolName} 多节点拨测`,
        description: `idcd 拨测快照 · ${date} · ${toolName}`,
        type: "article",
      },
    }
  }

  const domain = report.domain ?? "未知域名"
  const date = new Date(report.createdAt).toLocaleDateString("zh-CN")
  return {
    title: `${domain} 诊断报告 - idcd`,
    description: `${domain} 的完整网络诊断报告，包含 DNS / HTTP / Ping / Traceroute / SSL / ICP 备案 / WHOIS 七项检测结果`,
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

  return isSingleReport(report) ? (
    <SingleReportView report={report} />
  ) : (
    <ComboReportView report={report} />
  )
}
