import type { Metadata } from "next"
import MTRProbeClient from "./mtr-probe-client"

export const metadata: Metadata = {
  title: 'MTR 路由测试 - 网络路径诊断 | idcd',
  description: 'MTR 网络路由测试工具，结合 Ping 和 Traceroute 功能，诊断网络路径问题，统计每跳延迟和丢包率。',
}

export default function MTRProbePage() {
  return <MTRProbeClient />
}
