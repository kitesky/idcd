import type { Metadata } from "next"
import { Geist, Geist_Mono } from "next/font/google"
import { ThemeProvider } from "@/components/providers"
import { Nav } from "@/components/nav"
import { Footer } from "@/components/footer"
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

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html
      lang="zh-CN"
      className="dark"
      suppressHydrationWarning
    >
      <body className={`${geistSans.variable} ${geistMono.variable} antialiased min-h-screen flex flex-col`}>
        <ThemeProvider>
          <Nav />
          {children}
          <Footer />
        </ThemeProvider>
      </body>
    </html>
  )
}