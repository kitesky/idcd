import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, waitFor } from "@testing-library/react"
import "@testing-library/jest-dom"
import NodesPage from "../page"
import * as api from "@/lib/api"

// Mock getNodes API
vi.mock("@/lib/api", () => ({
  getNodes: vi.fn(),
}))

// Mock NodesClient component (避免 ECharts SSR 问题)
vi.mock("../nodes-client", () => ({
  NodesClient: ({ initialNodes }: { initialNodes: any[] }) => (
    <div data-testid="nodes-client">
      <div>节点数量: {initialNodes.length}</div>
    </div>
  ),
}))

describe("NodesPage", () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it("应该渲染页面标题和描述", async () => {
    vi.mocked(api.getNodes).mockResolvedValue({ data: [] })

    render(await NodesPage())

    expect(screen.getByText("全球监控节点")).toBeInTheDocument()
    expect(
      screen.getByText(/idcd 在全球部署了多个监控节点/)
    ).toBeInTheDocument()
  })

  it("应该成功加载节点数据", async () => {
    const mockNodes = [
      {
        id: "node-1",
        name: "北京节点",
        country_code: "CN",
        region: "北京",
        city: "北京",
        asn: "AS4134",
        isp: "中国电信",
        tier: "tier1_cn",
        status: "active",
        is_active: true,
      },
      {
        id: "node-2",
        name: "东京节点",
        country_code: "JP",
        region: "东京",
        city: "东京",
        asn: "AS2516",
        isp: "KDDI",
        tier: "tier1_overseas",
        status: "active",
        is_active: true,
      },
    ]

    vi.mocked(api.getNodes).mockResolvedValue({ data: mockNodes })

    render(await NodesPage())

    await waitFor(() => {
      expect(screen.getByTestId("nodes-client")).toBeInTheDocument()
      expect(screen.getByText("节点数量: 2")).toBeInTheDocument()
    })
  })

  it("应该处理 API 错误", async () => {
    vi.mocked(api.getNodes).mockRejectedValue(new Error("网络错误"))

    render(await NodesPage())

    await waitFor(() => {
      expect(screen.getByText("网络错误")).toBeInTheDocument()
      expect(screen.queryByTestId("nodes-client")).not.toBeInTheDocument()
    })
  })

  it("应该处理空节点列表", async () => {
    vi.mocked(api.getNodes).mockResolvedValue({ data: [] })

    render(await NodesPage())

    await waitFor(() => {
      expect(screen.getByTestId("nodes-client")).toBeInTheDocument()
      expect(screen.getByText("节点数量: 0")).toBeInTheDocument()
    })
  })
})
