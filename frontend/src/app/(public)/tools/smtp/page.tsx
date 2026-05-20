import type { Metadata } from "next"
import SmtpProbeClient from "./smtp-probe-client"

export const metadata: Metadata = {
  title: "SMTP 邮件服务器检测 | idcd 工具",
  description: "测试邮件服务器的 SMTP 连接性，检测 banner 和 EHLO 握手",
}

export default function SmtpPage() {
  return <SmtpProbeClient />
}
