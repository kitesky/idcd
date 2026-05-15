import type { Metadata } from "next"
import WhoisInfoClient from "./whois-info-client"

export const metadata: Metadata = {
  title: "WHOIS 查询 | idcd 工具",
  description: "查询域名的注册信息、注册商、注册日期、到期日期和名称服务器",
}

export default function WhoisPage() {
  return <WhoisInfoClient />
}
