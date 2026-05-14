import type { Metadata } from "next"
import { RefundClient, type RefundFailedPayment } from "./refund-client"

export const metadata: Metadata = {
  title: "退款失败看板 — idcd Admin",
}

const API_BASE = process.env.API_BASE_URL ?? "http://localhost:8080"

async function fetchRefundFailed(): Promise<RefundFailedPayment[]> {
  try {
    const res = await fetch(`${API_BASE}/v1/admin/refund-failed`, {
      cache: "no-store",
    })
    if (!res.ok) return []
    const json = await res.json()
    return json?.data?.payments ?? []
  } catch {
    return []
  }
}

export default async function RefundFailedPage() {
  const payments = await fetchRefundFailed()

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-2xl font-bold tracking-tight">退款失败看板</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          D5 — refund_failed 状态支付一览，支持手动触发重试
        </p>
      </div>
      <RefundClient initialPayments={payments} />
    </div>
  )
}
