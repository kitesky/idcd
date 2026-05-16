import type { Metadata, Viewport } from "next"
import { headers } from "next/headers"
import { Geist, Geist_Mono } from "next/font/google"
import { NextIntlClientProvider } from "next-intl"
import { ThemeProvider } from "@/components/providers"
import { TooltipProvider } from "@/components/ui/tooltip"
import { Toaster } from "@/components/ui/sonner"
import { CookieBanner } from "@/components/cookie-banner"
import { getLocale } from "@/i18n/locale"
import { loadMessages } from "@/i18n/request"
import { bcp47Of } from "@/i18n/registry"
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
  metadataBase: new URL("https://idcd.com"),
  title: {
    default: "idcd — 网络诊断工具",
    template: "%s | idcd",
  },
  description: "专业的网络诊断和监控平台，提供多地拨测、实时监控、Evidence证据服务",
  keywords: ["网络诊断", "拨测", "监控", "ping", "http", "traceroute", "dns"],
  authors: [{ name: "idcd.com" }],
  manifest: "/manifest.webmanifest",
  openGraph: {
    title: "idcd — 全球网络诊断",
    description: "多节点拨测、实时监控、一键诊断，专业网络可观测平台",
    url: "https://idcd.com",
    siteName: "idcd",
    type: "website",
    locale: "zh_CN",
  },
  twitter: {
    card: "summary_large_image",
    title: "idcd — 全球网络诊断",
    description: "多节点拨测、实时监控、一键诊断",
    site: "@idcd_com",
  },
}

export default async function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  const [headersList, locale] = await Promise.all([
    headers(),
    getLocale(),
  ])
  const messages = await loadMessages(locale)
  const nonce = headersList.get("x-nonce") ?? undefined
  const htmlLang = bcp47Of(locale)

  return (
    <html lang={htmlLang} suppressHydrationWarning>
      <body className={`${geistSans.variable} ${geistMono.variable} antialiased min-h-screen flex flex-col`}>
        <NextIntlClientProvider locale={locale} messages={messages} timeZone="Asia/Shanghai" now={new Date()}>
          <ThemeProvider nonce={nonce}>
            <TooltipProvider delayDuration={200}>
              {children}
              <CookieBanner />
              <Toaster />
            </TooltipProvider>
          </ThemeProvider>
        </NextIntlClientProvider>
      </body>
    </html>
  )
}
