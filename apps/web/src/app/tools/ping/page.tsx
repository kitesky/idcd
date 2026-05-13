import type { Metadata } from "next"
import PingProbeClient from "./ping-probe-client"

export const metadata: Metadata = {
  title: '多地 Ping 测试 - 全球 Ping 延迟检测 | idcd',
  description: '多地 Ping 测试工具，检测目标服务器在全球不同地区的网络延迟和丢包率。',
}

export default function PingProbePage() {
  return <PingProbeClient />
}
