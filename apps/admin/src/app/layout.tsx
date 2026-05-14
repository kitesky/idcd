import type { Metadata } from "next"
import { Geist, Geist_Mono } from "next/font/google"
import "./globals.css"

const geistSans = Geist({
  subsets: ["latin"],
  variable: "--font-sans",
  display: "swap",
})

const geistMono = Geist_Mono({
  subsets: ["latin"],
  variable: "--font-mono",
  display: "swap",
})

export const metadata: Metadata = {
  title: "idcd Admin",
  description: "idcd 管理台 — 内部使用，仅限 VPN 访问",
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html lang="zh-CN" className="dark" suppressHydrationWarning>
      <body
        className={`${geistSans.variable} ${geistMono.variable} antialiased min-h-screen bg-background text-foreground`}
      >
        <div className="flex min-h-screen flex-col">
          <header className="border-b border-border bg-card px-6 py-3">
            <div className="flex items-center gap-4">
              <span className="text-lg font-semibold text-primary">
                idcd Admin
              </span>
              <nav className="flex gap-4 text-sm">
                <a href="/metrics" className="text-muted-foreground hover:text-foreground transition-colors">系统概览</a>
                <a href="/users" className="text-muted-foreground hover:text-foreground transition-colors">用户管理</a>
                <a href="/nodes" className="text-muted-foreground hover:text-foreground transition-colors">节点健康</a>
                <a href="/refund-failed" className="text-muted-foreground hover:text-foreground transition-colors">退款失败</a>
              </nav>
            </div>
          </header>
          <main className="flex-1 container mx-auto px-6 py-6">{children}</main>
        </div>
      </body>
    </html>
  )
}
