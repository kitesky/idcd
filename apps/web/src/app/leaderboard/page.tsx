import { Metadata } from "next"
import { LeaderboardClient } from "./leaderboard-client"
import { NODE_COUNT, getCurrentMonthLabel } from "./leaderboard-data"

export const metadata: Metadata = {
  title: "全球 CDN & 网络性能排行榜 - idcd",
  description:
    "基于真实探测节点的 CDN 测速排行榜，涵盖 Cloudflare、Akamai、腾讯云、阿里云等主流 CDN 全球延迟、中国大陆延迟及可用性数据，每月更新。",
  keywords: [
    "CDN 测速",
    "全球延迟",
    "网络性能排行",
    "CDN 排行榜",
    "Cloudflare 延迟",
    "阿里云 CDN",
    "腾讯云 CDN",
    "P50 延迟",
    "网络诊断",
    "TTFB",
  ],
  openGraph: {
    title: "全球 CDN & 网络性能排行榜 - idcd",
    description:
      "基于真实探测节点的月度 CDN 性能排行榜，覆盖中国大陆与海外主流 CDN 提供商。",
    url: "https://idcd.com/leaderboard",
  },
}

export default function LeaderboardPage() {
  const monthLabel = getCurrentMonthLabel()

  return (
    <main className="min-h-screen bg-background">
      <div className="container mx-auto px-4 py-8 max-w-7xl">
        {/* Header */}
        <div className="mb-8">
          <h1 className="text-3xl font-bold tracking-tight">
            全球 CDN &amp; 网络性能排行榜
          </h1>
          <p className="mt-2 text-muted-foreground">
            基于{" "}
            <span className="font-medium text-foreground">{NODE_COUNT}</span>{" "}
            个真实探测节点，月度更新
          </p>
          <p className="mt-1 text-sm text-muted-foreground">
            数据周期：{monthLabel} &nbsp;·&nbsp; 更新频率：每 5 分钟探测一次，月底汇总
          </p>
        </div>

        {/* Tabs + Content */}
        <LeaderboardClient />
      </div>
    </main>
  )
}
