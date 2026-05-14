import type { Metadata } from "next"
import DiagnoseClient from "./diagnose-client"

export const metadata: Metadata = {
  title: '一键网络诊断 - idcd',
  description: '输入域名，一键并发 DNS/HTTPS/Ping/Traceroute/SSL/WHOIS 六项诊断，实时查看网络状况',
}

export default function DiagnosePage() {
  return <DiagnoseClient />
}
