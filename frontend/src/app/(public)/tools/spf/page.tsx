import type { Metadata } from "next"
import SpfInfoClient from "./spf-info-client"

export const metadata: Metadata = {
  title: "SPF 记录查询 | idcd 工具",
  description: "查询域名的 SPF（发件人策略框架）记录，验证邮件发送授权",
}

export default function SpfPage() {
  return <SpfInfoClient />
}
