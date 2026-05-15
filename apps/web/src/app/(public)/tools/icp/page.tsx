import type { Metadata } from "next"
import IcpInfoClient from "./icp-info-client"

export const metadata: Metadata = {
  title: "ICP 备案查询 | idcd 工具",
  description: "查询域名的 ICP 备案号、主办单位、备案类型和备案时间",
}

export default function IcpPage() {
  return <IcpInfoClient />
}
