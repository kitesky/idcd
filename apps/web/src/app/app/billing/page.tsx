import { Suspense } from "react"
import type { Metadata } from "next"
import { BillingClient } from "./billing-client"

export const metadata: Metadata = {
  title: "计费 - idcd 控制台",
  description: "管理您的订阅方案和发票记录",
}

export default function BillingPage() {
  return (
    <>
      <div>
        <h1 className="text-2xl font-bold tracking-tight">计费</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          管理您的订阅方案、查看用量和下载发票
        </p>
      </div>
      <Suspense>
        <BillingClient />
      </Suspense>
    </>
  )
}