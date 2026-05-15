import type { Metadata } from "next"
import SslInfoClient from "./ssl-info-client"

export const metadata: Metadata = {
  title: "SSL 证书检测 | idcd 工具",
  description: "检查域名的 SSL 证书有效性、颁发机构、到期日期和 SAN 域名列表",
}

export default function SslPage() {
  return <SslInfoClient />
}
