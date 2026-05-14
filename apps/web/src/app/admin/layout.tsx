import Link from "next/link"

const NAV = [
  { href: "/admin/metrics",          label: "系统概览" },
  { href: "/admin/users",            label: "用户管理" },
  { href: "/admin/nodes",            label: "节点健康" },
  { href: "/admin/refund-failed",    label: "退款失败" },
  { href: "/admin/beta-invitations", label: "Beta 邀请码" },
]

export default function AdminLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex min-h-screen flex-col dark">
      <header className="border-b bg-card px-6 py-3">
        <div className="flex items-center gap-6">
          <Link href={"/admin" as any} className="text-base font-semibold text-primary">
            idcd Admin
          </Link>
          <nav className="flex gap-4 text-sm">
            {NAV.map(item => (
              <Link
                key={item.href}
                href={item.href as any}
                className="text-muted-foreground transition-colors hover:text-foreground"
              >
                {item.label}
              </Link>
            ))}
          </nav>
        </div>
      </header>
      <main className="flex-1 container mx-auto px-6 py-6">{children}</main>
    </div>
  )
}
