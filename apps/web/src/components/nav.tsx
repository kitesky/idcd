"use client"

import { usePathname } from "next/navigation"
import { Menu, Globe } from "lucide-react"
import { Button } from "@/components/ui/button"
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet"

function LangToggle() {
  const pathname = usePathname()
  const isEn = pathname?.startsWith("/en") ?? false
  return (
    <div className="flex items-center gap-0.5 rounded-md border p-0.5">
      <Button variant={isEn ? "ghost" : "secondary"} size="sm" className="h-7 px-2 text-xs" asChild>
        <a href="/" aria-label="切换为中文">中</a>
      </Button>
      <Button variant={isEn ? "secondary" : "ghost"} size="sm" className="h-7 px-2 text-xs" asChild>
        <a href="/en/" aria-label="Switch to English">
          <Globe className="h-3 w-3 mr-1" />
          EN
        </a>
      </Button>
    </div>
  )
}

const navigation = [
  { name: "工具", href: "/tools" },
  { name: "节点", href: "/nodes" },
  { name: "成为节点", href: "/nodes/apply" },
  { name: "定价", href: "/pricing" },
  { name: "文档", href: "/docs/api" },
]

export function Nav() {
  return (
    <header className="sticky top-0 z-50 w-full border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
      <nav className="mx-auto flex max-w-7xl items-center justify-between px-4 py-3 sm:px-6 lg:px-8">
        {/* Logo */}
        <a href="/" className="flex items-center">
          <span className="font-mono font-bold text-primary text-xl">idcd</span>
        </a>

        {/* Desktop Navigation */}
        <div className="hidden md:flex md:items-center md:gap-8">
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

        {/* Desktop Right */}
        <div className="hidden md:flex md:items-center md:gap-3">
          <LangToggle />
          <Button variant="outline" asChild>
            <a href="/auth/login">登录</a>
          </Button>
          <Button asChild>
            <a href="/auth/register">注册</a>
          </Button>
        </div>

        {/* Mobile menu — Sheet 组件 */}
        <Sheet>
          <SheetTrigger asChild>
            <Button variant="ghost" size="icon" className="md:hidden" aria-label="打开菜单">
              <Menu className="h-5 w-5" />
            </Button>
          </SheetTrigger>
          <SheetContent side="left" className="w-72 p-0">
            <SheetHeader className="border-b px-4 py-3">
              <SheetTitle asChild>
                <a href="/" className="font-mono font-bold text-primary text-xl">
                  idcd
                </a>
              </SheetTitle>
            </SheetHeader>
            <div className="flex flex-col gap-1 p-4">
              {navigation.map((item) => (
                <Button key={item.name} variant="ghost" className="justify-start" asChild>
                  <a href={item.href}>{item.name}</a>
                </Button>
              ))}
            </div>
            <div className="border-t p-4 flex flex-col gap-2">
              <LangToggle />
              <Button variant="outline" className="w-full" asChild>
                <a href="/auth/login">登录</a>
              </Button>
              <Button className="w-full" asChild>
                <a href="/auth/register">注册</a>
              </Button>
            </div>
          </SheetContent>
        </Sheet>
      </nav>
    </header>
  )
}
