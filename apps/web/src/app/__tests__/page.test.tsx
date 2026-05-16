import { render, screen } from "@testing-library/react"
import { describe, it, expect, vi } from "vitest"
import HomePage from "../(public)/page"

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
      screen.getByRole("heading", { name: /网络拨测工具/ })
    ).toBeInTheDocument()
  })

  it("renders the diagnosis input", () => {
    render(<HomePage />)

    const input = screen.getByRole("textbox")
    expect(input).toBeInTheDocument()
    expect(input).toHaveAttribute("placeholder", "请输入域名或 IP")
  })

  it("renders the diagnosis button", () => {
    render(<HomePage />)

    expect(
      screen.getByRole("button", { name: /一键诊断/ })
    ).toBeInTheDocument()
  })

  it("renders feature cards", () => {
    render(<HomePage />)

    expect(screen.getByText("网络质量监控")).toBeInTheDocument()
    expect(screen.getByText("DNS 监测")).toBeInTheDocument()
    expect(screen.getByText("API 监测")).toBeInTheDocument()
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