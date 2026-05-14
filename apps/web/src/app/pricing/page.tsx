import type { Metadata } from "next"
import type { Route } from "next"
import Link from "next/link"

export const metadata: Metadata = {
  title: "定价 | idcd",
  description: "idcd 定价方案，即将推出",
}

export default function PricingPage() {
  return (
    <div className="flex flex-1 flex-col items-center justify-center px-4 py-24 text-center">
      <h1 className="text-3xl font-bold tracking-tight">定价方案</h1>
      <p className="mt-4 max-w-md text-muted-foreground">
        我们正在制定定价方案，敬请期待。目前所有诊断工具免费使用。
      </p>
      <div className="mt-8 flex gap-4">
        <Link
          href={"/tools" as Route}
          className="inline-flex h-9 items-center justify-center rounded-md bg-primary px-4 text-sm font-medium text-primary-foreground hover:bg-primary/90 transition-colors"
        >
          立即使用工具
        </Link>
        <Link
          href="/"
          className="inline-flex h-9 items-center justify-center rounded-md border px-4 text-sm font-medium hover:bg-accent transition-colors"
        >
          返回首页
        </Link>
      </div>
    </div>
  )
}
