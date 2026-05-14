import { describe, it, expect, vi } from "vitest"
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

describe("NodesPage", () => {
  it("应该渲染页面标题和描述", () => {
    render(<NodesPage />)
    expect(screen.getByText("全球监控节点")).toBeInTheDocument()
    expect(screen.getByText(/idcd 在全球部署了多个监控节点/)).toBeInTheDocument()
  })

  it("应该渲染节点客户端组件", () => {
    render(<NodesPage />)
    expect(screen.getByTestId("nodes-client")).toBeInTheDocument()
  })

  it("应该渲染 mock 节点数据", () => {
    render(<NodesPage />)
    // MOCK_NODES has nodes — should show a count > 0
    const countText = screen.getByText(/节点数量: \d+/)
    expect(countText).toBeInTheDocument()
    const match = countText.textContent?.match(/\d+/)
    expect(Number(match?.[0])).toBeGreaterThan(0)
  })

  it("应该渲染完整页面结构", () => {
    const { container } = render(<NodesPage />)
    expect(container.firstChild).toBeTruthy()
  })
})
