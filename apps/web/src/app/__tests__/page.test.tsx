import { render, screen } from "@testing-library/react"
import { describe, it, expect, vi } from "vitest"
import HomePage from "../page"

// Mock useRouter
const mockPush = vi.fn()
vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push: mockPush,
  }),
}))

describe("HomePage", () => {
  it("renders the main heading", () => {
    render(<HomePage />)

    expect(
      screen.getByRole("heading", { name: /多地网络诊断，秒级定位问题/ })
    ).toBeInTheDocument()
  })

  it("renders the diagnosis input", () => {
    render(<HomePage />)

    const input = screen.getByRole("textbox")
    expect(input).toBeInTheDocument()
    expect(input).toHaveAttribute("placeholder", "输入域名或 IP，如 github.com")
  })

  it("renders the diagnosis button", () => {
    render(<HomePage />)

    expect(
      screen.getByRole("button", { name: /一键诊断/ })
    ).toBeInTheDocument()
  })

  it("renders feature cards", () => {
    render(<HomePage />)

    expect(screen.getByText("全球节点覆盖")).toBeInTheDocument()
    expect(screen.getByText("实时多地并发")).toBeInTheDocument()
    expect(screen.getByText("SSL/安全检测")).toBeInTheDocument()
  })

  it("renders tools section", () => {
    render(<HomePage />)

    expect(screen.getByText("常用网络诊断工具")).toBeInTheDocument()
    // Tools appear in the tools card section
    expect(screen.getAllByText("HTTP检测").length).toBeGreaterThanOrEqual(1)
    expect(screen.getAllByText("Ping测试").length).toBeGreaterThanOrEqual(1)
    expect(screen.getAllByText("DNS查询").length).toBeGreaterThanOrEqual(1)
    expect(screen.getAllByText("SSL检查").length).toBeGreaterThanOrEqual(1)
  })
})