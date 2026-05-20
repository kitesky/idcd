import type { Metadata } from "next"
import DkimInfoClient from "./dkim-info-client"

export const metadata: Metadata = {
  title: "DKIM 记录查询 | idcd 工具",
  description: "查询域名的 DKIM（域名密钥标识邮件）签名记录",
}

export default function DkimPage() {
  return <DkimInfoClient />
}
