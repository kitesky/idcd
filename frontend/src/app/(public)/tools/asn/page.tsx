import type { Metadata } from "next"
import AsnInfoClient from "./asn-info-client"

export const metadata: Metadata = {
  title: "ASN 查询 | idcd 工具",
  description: "查询 IP 地址或 AS 号对应的自治系统、ISP 和国家/地区信息",
}

export default function AsnPage() {
  return <AsnInfoClient />
}
