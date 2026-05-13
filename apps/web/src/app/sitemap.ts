import { MetadataRoute } from "next"

export default function sitemap(): MetadataRoute.Sitemap {
  const baseUrl = "https://idcd.com"

  const staticPages = ["/", "/nodes", "/tools/diagnose"]
  const toolPages = [
    "http", "ping", "tcping", "dns", "traceroute",
    "json-formatter", "base64", "timestamp", "hash", "jwt-decoder",
    "regex-tester", "cron-parser", "qrcode", "cidr-calculator", "ipv6-converter"
  ]
  const authPages = ["/auth/register", "/auth/login", "/auth/forgot-password"]

  return [
    ...staticPages.map(url => ({
      url: baseUrl + url,
      changeFrequency: "weekly" as const,
      priority: url === "/" ? 1.0 : 0.8
    })),
    ...toolPages.map(slug => ({
      url: `${baseUrl}/tools/${slug}`,
      changeFrequency: "monthly" as const,
      priority: 0.7
    })),
    ...authPages.map(url => ({
      url: baseUrl + url,
      changeFrequency: "yearly" as const,
      priority: 0.3
    })),
  ]
}
