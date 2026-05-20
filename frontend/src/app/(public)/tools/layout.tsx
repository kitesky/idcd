import type { Metadata } from "next"
import { ToolPageContainer } from "@/components/tools/ToolPageContainer"

export const metadata: Metadata = {
  title: "专业网络工具站",
  description: "50+ 网络诊断工具：HTTP 拨测、Ping、DNS、SSL、WHOIS、IP 归属、ASN、BGP 一站式查询，格式转换与文本处理工具。",
}

export default function ToolsLayout({ children }: { children: React.ReactNode }) {
  return <ToolPageContainer>{children}</ToolPageContainer>
}
