import Link from "next/link"
import { Separator } from "@/components/ui"

const productLinks = [
  { name: "工具", href: "/tools" },
  { name: "节点", href: "/nodes" },
  { name: "定价", href: "/pricing" },
]

const resourceLinks = [
  { name: "文档", href: "/docs" },
  { name: "API", href: "/docs/api" },
  { name: "博客", href: "/blog" },
]

const companyLinks = [
  { name: "关于", href: "/about" },
  { name: "条款", href: "/terms" },
  { name: "隐私", href: "/privacy" },
  { name: "AUP", href: "/aup" },
]

export function Footer() {
  return (
    <footer className="border-t bg-background">
      <div className="mx-auto max-w-7xl px-4 py-12 sm:px-6 lg:px-8">
        {/* Links */}
        <div className="grid grid-cols-1 gap-8 sm:grid-cols-3">
          <div>
            <h3 className="text-sm font-semibold text-foreground">产品</h3>
            <ul className="mt-4 space-y-3">
              {productLinks.map((link) => (
                <li key={link.name}>
                  <Link
                    href={link.href as any}
                    className="text-sm text-muted-foreground hover:text-foreground transition-colors"
                  >
                    {link.name}
                  </Link>
                </li>
              ))}
            </ul>
          </div>

          <div>
            <h3 className="text-sm font-semibold text-foreground">资源</h3>
            <ul className="mt-4 space-y-3">
              {resourceLinks.map((link) => (
                <li key={link.name}>
                  <Link
                    href={link.href as any}
                    className="text-sm text-muted-foreground hover:text-foreground transition-colors"
                  >
                    {link.name}
                  </Link>
                </li>
              ))}
            </ul>
          </div>

          <div>
            <h3 className="text-sm font-semibold text-foreground">公司</h3>
            <ul className="mt-4 space-y-3">
              {companyLinks.map((link) => (
                <li key={link.name}>
                  <Link
                    href={link.href as any}
                    className="text-sm text-muted-foreground hover:text-foreground transition-colors"
                  >
                    {link.name}
                  </Link>
                </li>
              ))}
            </ul>
          </div>
        </div>

        <Separator className="my-8" />

        {/* Copyright */}
        <div className="flex flex-col items-center space-y-2 sm:flex-row sm:justify-between sm:space-y-0">
          <p className="text-sm text-muted-foreground">
            © 2026 idcd.com. 保留所有权利。
          </p>
        </div>
      </div>
    </footer>
  )
}
