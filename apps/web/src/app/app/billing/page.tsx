import type { Metadata } from "next"
import { BillingClient } from "./billing-client"

export const metadata: Metadata = {
  title: "计费 - idcd 控制台",
  description: "管理您的订阅方案和发票记录",
}

export default function BillingPage() {
  return (
    <div className="min-h-screen bg-background">
      <div className="container mx-auto px-4 py-8">
        <div className="mb-8">
          <h1 className="text-3xl font-bold tracking-tight">计费</h1>
          <p className="mt-2 text-muted-foreground">
            管理您的订阅方案、查看用量和下载发票
          </p>
        </div>
        <BillingClient />
      </div>
    </div>
  )
}
