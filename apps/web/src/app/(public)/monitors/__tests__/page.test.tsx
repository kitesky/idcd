import { describe, it, expect } from "vitest"
import { render, screen } from "@testing-library/react"
import "@testing-library/jest-dom"
import MonitorsLandingPage from "../page"

describe("MonitorsLandingPage", () => {
  it("renders the hero headline and primary CTA", () => {
    render(<MonitorsLandingPage />)
    expect(
      screen.getByRole("heading", { level: 1, name: /全球节点替你/ }),
    ).toBeInTheDocument()
    const ctas = screen.getAllByRole("link", { name: /免费创建/ })
    expect(ctas.length).toBeGreaterThan(0)
    expect(ctas[0]).toHaveAttribute("href", "/auth/register")
  })

  it("renders all 7 monitor type cards", () => {
    render(<MonitorsLandingPage />)
    for (const name of [
      "HTTP / HTTPS",
      "Ping",
      "TCP 端口",
      "DNS 解析",
      "SSL 到期",
      "域名到期",
      "关键字检测",
    ]) {
      expect(screen.getByText(name)).toBeInTheDocument()
    }
  })

  it("renders the 4-step workflow", () => {
    render(<MonitorsLandingPage />)
    for (const step of ["创建任务", "多节点并发拨测", "异常告警", "证据 + 报表"]) {
      expect(screen.getByText(step)).toBeInTheDocument()
    }
  })

  it("links to user-center monitors and to pricing", () => {
    render(<MonitorsLandingPage />)
    const demo = screen.getByRole("link", { name: /查看演示/ })
    expect(demo).toHaveAttribute("href", "/app/monitors")
    const pricing = screen.getByRole("link", { name: /查看完整定价/ })
    expect(pricing).toHaveAttribute("href", "/pricing")
  })
})
