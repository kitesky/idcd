import Link from "next/link"

export default function NotFound() {
  return (
    <main className="min-h-screen flex items-center justify-center bg-background">
      <div className="text-center space-y-4">
        <h1 className="text-4xl font-bold text-foreground">404</h1>
        <h2 className="text-xl text-muted-foreground">页面未找到</h2>
        <p className="text-muted-foreground">
          抱歉，您访问的页面不存在。
        </p>
        <Link
          href="/"
          className="inline-block mt-4 px-4 py-2 bg-primary text-primary-foreground rounded-md hover:bg-primary/90 transition-colors"
        >
          返回首页
        </Link>
      </div>
    </main>
  )
}