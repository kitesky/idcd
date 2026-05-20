import type { Metadata } from "next"
import RdnsInfoClient from "./rdns-info-client"

export const metadata: Metadata = {
  title: "反向 DNS 查询 | idcd 工具",
  description: "查询 IP 地址对应的反向 DNS（PTR 记录）主机名",
}

export default function RdnsPage() {
  return <RdnsInfoClient />
}
