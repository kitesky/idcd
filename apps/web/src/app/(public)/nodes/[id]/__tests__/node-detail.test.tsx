import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen } from "@testing-library/react"
import "@testing-library/jest-dom"
import NodeDetailPage from "../page"

function buildHealthTrend() {
  const now = new Date()
  now.setMinutes(0, 0, 0)
  return Array.from({ length: 24 }, (_, i) => {
    const h = new Date(now.getTime() - i * 3600 * 1000)
    const rate = i === 3 ? 87.5 : i === 7 ? 94.2 : 100.0
    const latency = 30 + Math.sin(i * 0.4) * 8 + (i === 3 ? 45 : 0)
    return {
      hour: h.toISOString(),
      success_rate: rate,
      avg_latency: parseFloat(latency.toFixed(1)),
    }
  })
}

const MOCK_DIAGNOSTICS = {
  data: {
    node_id: "jp-tok-ntt-01",
    name: "Tokyo JP — NTT",
    location: { country: "JP", city: "Tokyo", asn: "AS2914", isp: "NTT" },
    status: "active",
    uptime_24h: 99.97,
    checks_24h: 1440,
    latency_distribution: { p50: 32.5, p90: 45.2, p95: 58.1, p99: 124.7, min: 18.2, max: 312.5 },
    health_trend: buildHealthTrend(),
    last_seen: new Date().toISOString(),
  },
}

beforeEach(() => {
  vi.stubGlobal(
    "fetch",
    vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => MOCK_DIAGNOSTICS,
    }),
  )
})

describe("NodeDetailPage", () => {
  it("渲染不崩溃 — 已知节点", async () => {
    const page = await NodeDetailPage({ params: Promise.resolve({ id: "jp-tok-ntt-01" }) })
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

  it("404 时显示节点不存在提示", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: false,
        status: 404,
        json: async () => ({ error: { message: "node not found" } }),
      }),
    )
    const page = await NodeDetailPage({ params: Promise.resolve({ id: "nonexistent-node" }) })
    render(page)
    expect(screen.getByTestId("not-found-state")).toBeInTheDocument()
    // Text may be split across elements due to <span> for node id
    expect(screen.getByTestId("not-found-state").textContent).toContain("不存在或已下线")
  })

  it("网络错误时显示错误提示", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockRejectedValue(new Error("network error")),
    )
    const page = await NodeDetailPage({ params: Promise.resolve({ id: "jp-tok-ntt-01" }) })
    render(page)
    expect(screen.getByTestId("error-state")).toBeInTheDocument()
  })
})
