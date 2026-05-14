import type { Metadata } from "next"
import DnsProbeClient from "./dns-probe-client"

export const metadata: Metadata = {
  title: 'DNS 解析查询 - 多地 DNS 查询工具 | idcd',
  description: '多地 DNS 解析查询工具，检测域名在全球不同地区的 DNS 解析结果和响应时间。',
}

export default function DnsProbePage() {
  return <DnsProbeClient />
}
