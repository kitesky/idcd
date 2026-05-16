import type { Metadata } from "next"
import { OrdersClient } from "./orders-client"

export const metadata: Metadata = {
  title: "证书订单 - idcd",
  description: "查看所有证书申请订单及其状态",
}

export default function OrdersPage() {
  return (
    <>
      <div>
        <h1 className="text-2xl font-bold tracking-tight">证书订单</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          每次申请都会生成一个订单，订单完成后会出现在「已签证书」列表。
        </p>
      </div>
      <OrdersClient />
    </>
  )
}
