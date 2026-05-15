import type { Metadata } from "next"
import IpInfoClient from "./ip-info-client"

export const metadata: Metadata = {
  title: "IP 地址查询 | idcd 工具",
  description: "查询 IP 地址的地理位置、ASN、ISP 信息，支持 IPv4 和 IPv6",
}

export default function IpPage() {
  return <IpInfoClient />
}
