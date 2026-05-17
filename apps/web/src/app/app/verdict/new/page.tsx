import type { Metadata } from "next"
import { NewVerdictOrderClient } from "./new-verdict-order-client"

export const metadata: Metadata = {
  title: "创建证据报告 — idcd",
  description: "为指定目标和时间窗生成可签名、可验证的 idcd 证据报告 PDF。",
}

export default function NewVerdictOrderPage() {
  return (
    <>
      <div>
        <h1 className="text-2xl font-bold tracking-tight">创建证据报告</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          选择模板、目标和时间窗，下单后将跳转支付。报告生成完成后可在控制台下载，
          并通过公开验签页验证。
        </p>
      </div>
      <NewVerdictOrderClient />
    </>
  )
}
