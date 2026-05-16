import { MetadataRoute } from "next"
import { ALL_TOOLS } from "@/app/(public)/tools/tools-config"

export default function sitemap(): MetadataRoute.Sitemap {
  const baseUrl = "https://idcd.com"
  const now = new Date()

  const mainPages: { url: string; priority: number }[] = [
    { url: "/", priority: 1.0 },
    { url: "/nodes", priority: 0.8 },
    { url: "/about", priority: 0.8 },
    { url: "/pricing", priority: 0.9 },
    { url: "/agent", priority: 0.9 },
    { url: "/leaderboard", priority: 0.8 },
    { url: "/transparency", priority: 0.7 },
    { url: "/en", priority: 0.7 },
    { url: "/terms", priority: 0.8 },
    { url: "/privacy", priority: 0.8 },
    { url: "/aup", priority: 0.8 },
  ]

  return [
    // 主要落地页
    ...mainPages.map(({ url, priority }) => ({
      url: baseUrl + url,
      lastModified: now,
      changeFrequency: "weekly" as const,
      priority,
    })),
    // 工具页（ALL_TOOLS，无重复）
    ...ALL_TOOLS.map((tool) => ({
      url: `${baseUrl}/tools/${tool.slug}`,
      lastModified: now,
      changeFrequency: "monthly" as const,
      priority: 0.7,
    })),
  ]
}
