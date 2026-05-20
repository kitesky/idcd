import type { Metadata } from "next"
import { OrderDetailClient } from "./order-detail-client"

type Props = { params: Promise<{ id: string }> }

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { id } = await params
  return {
    title: `订单 ${id} - idcd`,
    description: "证书订单详情",
  }
}

export default async function OrderDetailPage({ params }: Props) {
  const { id } = await params
  return <OrderDetailClient orderId={id} />
}
