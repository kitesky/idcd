import { MetadataRoute } from "next"
import { ALL_TOOLS } from "@/app/(public)/tools/tools-config"

export default function sitemap(): MetadataRoute.Sitemap {
  const baseUrl = "https://idcd.com"
  const now = new Date()

  const mainPages = ["/", "/nodes", "/about", "/terms", "/privacy", "/aup"]
  const staticToolSlugs = [
    "diagnose",
    "http",
    "ping",
    "tcping",
    "dns",
    "traceroute",
    "json-formatter",
    "base64",
    "timestamp",
    "hash",
    "jwt-decoder",
    "regex-tester",
    "cron-parser",
    "qrcode",
    "cidr-calculator",
    "ipv6-converter",
  ]

  return [
    // 主要落地页
    ...mainPages.map((url) => ({
      url: baseUrl + url,
      lastModified: now,
      changeFrequency: "weekly" as const,
      priority: url === "/" ? 1.0 : 0.8,
    })),
    // 静态工具页
    ...staticToolSlugs.map((slug) => ({
      url: `${baseUrl}/tools/${slug}`,
      lastModified: now,
      changeFrequency: "monthly" as const,
      priority: 0.7,
    })),
    // 动态工具页（ALL_TOOLS 中的 50 个）
    ...ALL_TOOLS.map((tool) => ({
      url: `${baseUrl}/tools/${tool.slug}`,
      lastModified: now,
      changeFrequency: "monthly" as const,
      priority: 0.7,
    })),
  ]
}
