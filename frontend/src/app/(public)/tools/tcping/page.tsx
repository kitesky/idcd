import type { Metadata } from "next"
import TcpingProbeClient from "./tcping-probe-client"

export const metadata: Metadata = {
  title: 'TCP 端口连通性测试 - TCPing 工具 | idcd',
  description: '多地 TCPing 测试工具，检测 TCP 端口在全球不同地区的连通性和延迟。',
}

export default function TcpingProbePage() {
  return <TcpingProbeClient />
}
