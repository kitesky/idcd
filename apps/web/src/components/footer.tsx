import { Separator } from "@idcd/ui"

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
                  <a
                    href={link.href}
                    className="text-sm text-muted-foreground hover:text-foreground transition-colors"
                  >
                    {link.name}
                  </a>
                </li>
              ))}
            </ul>
          </div>

          <div>
            <h3 className="text-sm font-semibold text-foreground">资源</h3>
            <ul className="mt-4 space-y-3">
              {resourceLinks.map((link) => (
                <li key={link.name}>
                  <a
                    href={link.href}
                    className="text-sm text-muted-foreground hover:text-foreground transition-colors"
                  >
                    {link.name}
                  </a>
                </li>
              ))}
            </ul>
          </div>

          <div>
            <h3 className="text-sm font-semibold text-foreground">公司</h3>
            <ul className="mt-4 space-y-3">
              {companyLinks.map((link) => (
                <li key={link.name}>
                  <a
                    href={link.href}
                    className="text-sm text-muted-foreground hover:text-foreground transition-colors"
                  >
                    {link.name}
                  </a>
                </li>
              ))}
            </ul>
          </div>
        </div>

        <Separator className="my-8" />

        {/* Copyright */}
        <div className="flex flex-col items-center space-y-2 sm:flex-row sm:justify-between sm:space-y-0">
          <div className="flex flex-col items-center space-y-1 sm:flex-row sm:space-y-0 sm:space-x-4">
            <p className="text-sm text-muted-foreground">
              © 2026 idcd.com. 保留所有权利。
            </p>
            <span className="text-sm text-muted-foreground">
              京ICP备XXXXXXXX号
            </span>
          </div>
          <div>
            <a
              href="/privacy"
              className="text-sm text-muted-foreground hover:text-foreground transition-colors"
            >
              隐私协议
            </a>
          </div>
        </div>
      </div>
    </footer>
  )
}