"use client"

import { useState } from "react"
import { Menu, X } from "lucide-react"
import { Button } from "@/components/ui"

const navigation = [
  { name: "工具", href: "/tools" },
  { name: "节点", href: "/nodes" },
  { name: "定价", href: "/pricing" },
  { name: "文档", href: "/docs" },
]

export function Nav() {
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false)

  return (
    <header className="sticky top-0 z-50 w-full border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
      <nav className="mx-auto flex max-w-7xl items-center justify-between px-4 py-3 sm:px-6 lg:px-8">
        {/* Logo */}
        <a
          href="/"
          className="flex items-center"
          onClick={() => setMobileMenuOpen(false)}
        >
          <span className="font-mono font-bold text-primary text-xl">
            idcd
          </span>
        </a>

        {/* Desktop Navigation */}
        <div className="hidden md:flex md:items-center md:space-x-8">
          {navigation.map((item) => (
            <a
              key={item.name}
              href={item.href}
              className="text-sm font-medium text-foreground hover:text-primary transition-colors"
            >
              {item.name}
            </a>
          ))}
        </div>

        {/* Desktop Auth Buttons */}
        <div className="hidden md:flex md:items-center md:space-x-4">
          <Button variant="outline">
            <a href="/login">登录</a>
          </Button>
          <Button>
            <a href="/register">注册</a>
          </Button>
        </div>

        {/* Mobile menu button */}
        <div className="flex md:hidden">
          <button
            type="button"
            className="inline-flex items-center justify-center rounded-md p-2 text-foreground hover:bg-accent hover:text-accent-foreground focus:outline-none focus:ring-2 focus:ring-primary focus:ring-offset-2"
            onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
          >
            <span className="sr-only">打开主菜单</span>
            {mobileMenuOpen ? (
              <X className="h-6 w-6" aria-hidden="true" />
            ) : (
              <Menu className="h-6 w-6" aria-hidden="true" />
            )}
          </button>
        </div>
      </nav>

      {/* Mobile menu */}
      {mobileMenuOpen && (
        <div className="md:hidden">
          <div className="space-y-1 px-4 pb-3 pt-2 border-t">
            {navigation.map((item) => (
              <a
                key={item.name}
                href={item.href}
                className="block px-3 py-2 text-sm font-medium text-foreground hover:bg-accent hover:text-accent-foreground rounded-md"
                onClick={() => setMobileMenuOpen(false)}
              >
                {item.name}
              </a>
            ))}
            <div className="border-t pt-4">
              <div className="flex flex-col space-y-3 px-3">
                <Button variant="outline" className="justify-center">
                  <a href="/login" onClick={() => setMobileMenuOpen(false)}>
                    登录
                  </a>
                </Button>
                <Button className="justify-center">
                  <a href="/register" onClick={() => setMobileMenuOpen(false)}>
                    注册
                  </a>
                </Button>
              </div>
            </div>
          </div>
        </div>
      )}
    </header>
  )
}