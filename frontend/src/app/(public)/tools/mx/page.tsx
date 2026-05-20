import type { Metadata } from "next"
import MxInfoClient from "./mx-info-client"

export const metadata: Metadata = {
  title: "MX 记录查询 | idcd 工具",
  description: "查询域名的 MX 邮件交换记录，显示邮件服务器优先级和主机名",
}

export default function MxPage() {
  return <MxInfoClient />
}
