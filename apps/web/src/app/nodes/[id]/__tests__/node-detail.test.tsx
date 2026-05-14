import { describe, it, expect } from "vitest"
import { render, screen } from "@testing-library/react"
import "@testing-library/jest-dom"
import NodeDetailPage from "../page"

describe("NodeDetailPage", () => {
  it("渲染不崩溃 — 已知节点", async () => {
    const page = await NodeDetailPage({ params: Promise.resolve({ id: "jp-tok-ntt-01" }) })
    const { container } = render(page)
    expect(container.firstChild).toBeTruthy()
  })

  it("渲染不崩溃 — 未知节点 fallback", async () => {
    const page = await NodeDetailPage({ params: Promise.resolve({ id: "unknown-node-xyz" }) })
    const { container } = render(page)
    expect(container.firstChild).toBeTruthy()
  })

  it("显示节点 ID", async () => {
    const page = await NodeDetailPage({ params: Promise.resolve({ id: "jp-tok-ntt-01" }) })
    render(page)
    expect(screen.getByText("Tokyo JP — NTT")).toBeInTheDocument()
  })

  it("显示延迟分位数", async () => {
    const page = await NodeDetailPage({ params: Promise.resolve({ id: "jp-tok-ntt-01" }) })
    render(page)
    const dist = screen.getByTestId("latency-distribution")
    expect(dist).toBeInTheDocument()
    expect(dist.textContent).toContain("P50")
    expect(dist.textContent).toContain("P90")
    expect(dist.textContent).toContain("P95")
    expect(dist.textContent).toContain("P99")
  })

  it("显示健康趋势区域", async () => {
    const page = await NodeDetailPage({ params: Promise.resolve({ id: "jp-tok-ntt-01" }) })
    render(page)
    const trend = screen.getByTestId("health-trend")
    expect(trend).toBeInTheDocument()
  })

  it("显示 24h 可用率", async () => {
    const page = await NodeDetailPage({ params: Promise.resolve({ id: "jp-tok-ntt-01" }) })
    render(page)
    const elements = screen.getAllByText("99.97%")
    expect(elements.length).toBeGreaterThan(0)
  })

  it("包含返回节点列表链接", async () => {
    const page = await NodeDetailPage({ params: Promise.resolve({ id: "jp-tok-ntt-01" }) })
    render(page)
    const link = screen.getByRole("link", { name: /返回节点列表/ })
    expect(link).toHaveAttribute("href", "/nodes")
  })
})
