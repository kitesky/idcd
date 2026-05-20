import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen } from "@testing-library/react"
import "@testing-library/jest-dom"
import NodesPage from "../page"

// NodesClient uses ECharts which can't render in jsdom — mock it
vi.mock("../nodes-client", () => ({
  NodesClient: ({ nodes }: { nodes: { id: string }[] }) => (
    <div data-testid="nodes-client">
      <div>节点数量: {nodes.length}</div>
    </div>
  ),
}))

const MOCK_API_NODES = [
  {
    id: "cn-bj-ct-01",
    name: "北京-电信",
    region: "Asia",
    country: "CN",
    city: "北京",
    isp: "中国电信",
    ip: "61.135.169.121",
    status: "active",
    uptime_percent: 99.9,
    latency_ms: 12,
    last_seen_at: "2026-05-15T00:00:00Z",
  },
  {
    id: "hk-ct-pccw-01",
    name: "香港-PCCW",
    region: "Asia",
    country: "HK",
    city: "香港",
    isp: "PCCW",
    ip: "203.160.128.1",
    status: "active",
    uptime_percent: 99.5,
    latency_ms: 8,
    last_seen_at: "2026-05-15T00:00:00Z",
  },
]

beforeEach(() => {
  vi.stubGlobal(
    "fetch",
    vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ data: { nodes: MOCK_API_NODES } }),
    })
  )
})

describe("NodesPage", () => {
  it("应该渲染页面标题和描述", async () => {
    render(await NodesPage())
    expect(screen.getByText("全球监控节点")).toBeInTheDocument()
    expect(screen.getByText(/idcd 在全球部署了多个监控节点/)).toBeInTheDocument()
  })

  it("应该渲染节点客户端组件", async () => {
    render(await NodesPage())
    expect(screen.getByTestId("nodes-client")).toBeInTheDocument()
  })

  it("应该将 API 节点传给客户端组件", async () => {
    render(await NodesPage())
    const countText = screen.getByText(/节点数量: \d+/)
    expect(countText).toBeInTheDocument()
    const match = countText.textContent?.match(/\d+/)
    expect(Number(match?.[0])).toBe(MOCK_API_NODES.length)
  })

  it("fetch 失败时显示错误提示并传空节点数组", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: false,
        status: 503,
      })
    )
    render(await NodesPage())
    expect(screen.getByTestId("fetch-error")).toBeInTheDocument()
    const countText = screen.getByText(/节点数量: \d+/)
    const match = countText.textContent?.match(/\d+/)
    expect(Number(match?.[0])).toBe(0)
  })

  it("fetch 抛出异常时显示错误提示并传空节点数组", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("Network error")))
    render(await NodesPage())
    expect(screen.getByTestId("fetch-error")).toBeInTheDocument()
    const countText = screen.getByText(/节点数量: \d+/)
    const match = countText.textContent?.match(/\d+/)
    expect(Number(match?.[0])).toBe(0)
  })

  it("应该渲染完整页面结构", async () => {
    const { container } = render(await NodesPage())
    expect(container.firstChild).toBeTruthy()
  })
})
