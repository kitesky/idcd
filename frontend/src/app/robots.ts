import { MetadataRoute } from "next"

export default function robots(): MetadataRoute.Robots {
  return {
    rules: [
      {
        userAgent: "*",
        allow: "/",
        disallow: ["/app/", "/api/", "/legacy/"]
      },
    ],
    sitemap: "https://idcd.com/sitemap.xml",
  }
}
