import type { Metadata, Viewport } from "next"
import { headers } from "next/headers"
import { Geist, Geist_Mono } from "next/font/google"
import { ThemeProvider } from "@/components/providers"
import { TooltipProvider } from "@/components/ui/tooltip"
import { CookieBanner } from "@/components/cookie-banner"
import "./globals.css"

const geistSans = Geist({
  subsets: ["latin", "latin-ext"],
  variable: "--font-sans",
  display: "swap"
})

const geistMono = Geist_Mono({
  subsets: ["latin"],
  variable: "--font-mono",
  display: "swap"
})

export const viewport: Viewport = {
  themeColor: "#2563EB",
  width: "device-width",
  initialScale: 1,
}

export const metadata: Metadata = {
  title: "idcd — 网络诊断工具",
  description: "专业的网络诊断和监控平台，提供多地拨测、实时监控、Evidence证据服务",
  keywords: ["网络诊断", "拨测", "监控", "ping", "http", "traceroute", "dns"],
  authors: [{ name: "idcd.com" }],
  manifest: "/manifest.webmanifest",
  openGraph: {
    title: "idcd — 全球網絡診斷",
    description: "多節點拨測、一鍵診斷",
    url: "https://idcd.com",
    siteName: "idcd",
    type: "website",
  },
  twitter: {
    card: "summary_large_image",
    title: "idcd — 全球網絡診斷"
  },
}

export default async function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  const nonce = (await headers()).get("x-nonce") ?? undefined
  return (
    <html lang="zh-CN" suppressHydrationWarning>
      <body className={`${geistSans.variable} ${geistMono.variable} antialiased min-h-screen flex flex-col`}>
        <ThemeProvider nonce={nonce}>
          <TooltipProvider delayDuration={200}>
            {children}
            <CookieBanner />
          </TooltipProvider>
        </ThemeProvider>
      </body>
    </html>
  )
}
