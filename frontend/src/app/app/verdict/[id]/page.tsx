import type { Metadata } from "next"
import { VerdictOrderDetailClient } from "./verdict-order-detail-client"

export const metadata: Metadata = {
  title: "证据报告订单 — idcd",
  description: "查看证据报告订单状态、下载已签发的 PDF 并跳转公开验签。",
}

export default async function VerdictOrderDetailPage({
  params,
}: {
  params: Promise<{ id: string }>
}) {
  const { id } = await params
  return <VerdictOrderDetailClient orderId={id} />
}
