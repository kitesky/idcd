import type { Metadata } from "next"

export const metadata: Metadata = {
  title: "简单透明的定价",
  description: "从 Free 到 Business，找到适合你的方案。监控、告警、状态页一体化，无隐藏收费，随时取消。",
}

export default function PricingLayout({ children }: { children: React.ReactNode }) {
  return children
}
