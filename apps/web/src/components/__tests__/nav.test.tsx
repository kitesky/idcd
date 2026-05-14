import { render, screen } from "@testing-library/react"
import { describe, it, expect, vi } from "vitest"
import { Nav } from "../nav"

vi.mock("next/navigation", () => ({
  usePathname: () => "/",
}))

describe("Nav", () => {
  it("renders the logo", () => {
    render(<Nav />)

    const logo = screen.getByText("idcd")
    expect(logo).toBeInTheDocument()
    expect(logo).toHaveClass("font-mono", "font-bold", "text-primary")
  })

  it("renders main navigation links", () => {
    render(<Nav />)

    expect(screen.getByText("工具")).toBeInTheDocument()
    expect(screen.getByText("节点")).toBeInTheDocument()
    expect(screen.getByText("定价")).toBeInTheDocument()
    expect(screen.getByText("文档")).toBeInTheDocument()
  })

  it("renders auth buttons", () => {
    render(<Nav />)

    expect(screen.getByRole("link", { name: /登录/ })).toBeInTheDocument()
    expect(screen.getByRole("link", { name: /注册/ })).toBeInTheDocument()
  })

  it("renders mobile menu toggle", () => {
    render(<Nav />)

    expect(screen.getByRole("button", { name: /打开菜单/ })).toBeInTheDocument()
  })
})