import type { Metadata } from "next"
import BgpInfoClient from "./bgp-info-client"

export const metadata: Metadata = {
  title: "BGP 路由查询 | idcd 工具",
  description: "查询 IP 地址的 BGP 路由信息，包括所属前缀和 AS 路径",
}

export default function BgpPage() {
  return <BgpInfoClient />
}
