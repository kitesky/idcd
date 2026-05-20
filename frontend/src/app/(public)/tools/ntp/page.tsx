import type { Metadata } from "next"
import NtpProbeClient from "./ntp-probe-client"

export const metadata: Metadata = {
  title: "NTP 服务器检测 | idcd 工具",
  description: "查询 NTP 时间服务器，返回服务器时间与本地时钟的偏移量",
}

export default function NtpPage() {
  return <NtpProbeClient />
}
