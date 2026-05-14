import { describe, it, expect, vi } from "vitest"
import { render, screen } from "@testing-library/react"
import "@testing-library/jest-dom"
import NodesPage from "../page"

// NodesClient has DOM interactions not needed in unit tests — mock it
vi.mock("../nodes-client", () => ({
  NodesClient: ({ nodes }: { nodes: { id: string }[] }) => (
    <div data-testid="nodes-client">
      <span>节点数量: {nodes.length}</span>
    </div>
  ),
}))

describe("NodesPage", () => {
  it("应该渲染页面标题", () => {
    render(<NodesPage />)
    expect(screen.getByText("节点健康看板")).toBeInTheDocument()
  })

  it("应该渲染 NodesClient", () => {
    render(<NodesPage />)
    expect(screen.getByTestId("nodes-client")).toBeInTheDocument()
  })

  it("应该传入非空 mock 节点数据", () => {
    render(<NodesPage />)
    const countText = screen.getByText(/节点数量: \d+/)
    const match = countText.textContent?.match(/\d+/)
    expect(Number(match?.[0])).toBeGreaterThan(0)
  })

  it("应该渲染副标题说明", () => {
    render(<NodesPage />)
    expect(screen.getByText(/mock 数据/)).toBeInTheDocument()
  })
})
