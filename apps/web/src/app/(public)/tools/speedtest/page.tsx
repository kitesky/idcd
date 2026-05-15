import type { Metadata } from "next"
import SpeedtestProbeClient from "./speedtest-probe-client"

export const metadata: Metadata = {
  title: "网速测试 | idcd 工具",
  description: "通过分布式节点测量您的网站或服务器下载/上传带宽（HTTP 大包测速）",
}

export default function SpeedtestPage() {
  return <SpeedtestProbeClient />
}
