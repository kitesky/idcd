import type { Metadata } from "next"
import TracerouteProbeClient from "./traceroute-probe-client"

export const metadata: Metadata = {
  title: '路由追踪 - 多地 Traceroute 工具 | idcd',
  description: '多地路由追踪工具，显示数据包从不同节点到目标的完整路径和每一跳的延迟。',
}

export default function TracerouteProbePage() {
  return <TracerouteProbeClient />
}
