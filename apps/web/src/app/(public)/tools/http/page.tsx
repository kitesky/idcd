import type { Metadata } from "next"
import HttpProbeClient from "./http-probe-client"

export const metadata: Metadata = {
  title: 'HTTP/HTTPS 拨测 - 多地 HTTP 请求测试 | idcd',
  description: '多地 HTTP/HTTPS 拨测工具，测试网站在不同地区的可访问性和响应时间。',
}

export default function HttpProbePage() {
  return <HttpProbeClient />
}
