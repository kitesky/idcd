import type { Metadata } from "next"
import "./globals.css"

export const metadata: Metadata = {
  title: "服务状态 - idcd",
  description: "实时查看服务可用性状态",
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html lang="zh-CN" className="dark">
      <body className="antialiased min-h-screen bg-background text-foreground">
        {children}
      </body>
    </html>
  )
}
