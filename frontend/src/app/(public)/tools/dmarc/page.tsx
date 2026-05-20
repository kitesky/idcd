import type { Metadata } from "next"
import DmarcInfoClient from "./dmarc-info-client"

export const metadata: Metadata = {
  title: "DMARC 记录查询 | idcd 工具",
  description: "查询域名的 DMARC 策略记录，检查邮件认证配置",
}

export default function DmarcPage() {
  return <DmarcInfoClient />
}
