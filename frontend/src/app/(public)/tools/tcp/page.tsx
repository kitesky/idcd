import type { Metadata } from "next"
import TcpProbeClient from "./tcp-probe-client"

export const metadata: Metadata = {
  title: "TCP 端口测试 | idcd 工具",
  description: "从全球多个节点测试 TCP 端口的连通性和响应时间",
}

export default function TcpPage() {
  return <TcpProbeClient />
}
