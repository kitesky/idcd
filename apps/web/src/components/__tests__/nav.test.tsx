import { render, screen, waitFor } from "@testing-library/react"
import { describe, it, expect, vi, beforeEach } from "vitest"
import { Nav } from "../nav"

vi.mock("next/navigation", () => ({
  usePathname: () => "/",
  useRouter: () => ({ push: vi.fn(), refresh: vi.fn(), replace: vi.fn() }),
  useSearchParams: () => new URLSearchParams(""),
}))

// next-themes uses matchMedia and storage; provide a minimal mock
vi.mock("next-themes", () => ({
  useTheme: () => ({ resolvedTheme: "dark", setTheme: vi.fn() }),
}))

// NavUserMenu fetches /v1/account/profile on mount. Default to 401 (logged out)
// so the login/register buttons render. Individual tests can override.
beforeEach(() => {
  global.fetch = vi.fn().mockResolvedValue({
    ok: false,
    status: 401,
    json: () => Promise.resolve({}),
  }) as unknown as typeof fetch
})

vi.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const map: Record<string, string> = {
      "links.tools": "工具",
      "links.agent": "AI Agent",
      "links.nodes": "节点",
      "links.becomeNode": "成为节点",
      "links.pricing": "定价",
      "links.docs": "文档",
      "auth.login": "登录",
      "auth.register": "立即注册",
      "auth.dashboard": "控制台",
      "locale.label": "语言",
      "locale.zh": "简体中文",
      "locale.en": "English",
      "menu.open": "打开菜单",
      "menu.close": "关闭菜单",
      "search.placeholder": "搜索工具...",
      "search.popular": "热门工具",
      "search.results": "工具",
      "search.close": "关闭",
      "featured.label": "精选",
      "featured.viewAll": "查看全部",
    }
    return map[key] ?? key
  },
}))

function renderNav() {
  return render(<Nav />)
}

describe("Nav", () => {
  it("renders the logo", () => {
    renderNav()

    const logo = screen.getByText("idcd")
    expect(logo).toBeInTheDocument()
    expect(logo).toHaveClass("font-mono", "font-bold", "text-foreground")
  })

  it("renders main navigation links", () => {
    renderNav()

    expect(screen.getByText("工具")).toBeInTheDocument()
    expect(screen.getByText("节点")).toBeInTheDocument()
    expect(screen.getByText("定价")).toBeInTheDocument()
    expect(screen.getByText("文档")).toBeInTheDocument()
  })

  it("renders auth buttons when logged out", async () => {
    renderNav()

    // NavUserMenu starts in loading state and resolves after the profile fetch.
    // Default mock returns 401, so login/register links appear after the await.
    await waitFor(() => {
      expect(screen.getByRole("link", { name: /登录/ })).toBeInTheDocument()
      expect(screen.getByRole("link", { name: /注册/ })).toBeInTheDocument()
    })
  })

  it("renders mobile menu toggle", () => {
    renderNav()

    expect(screen.getByRole("button", { name: /打开菜单/ })).toBeInTheDocument()
  })
})