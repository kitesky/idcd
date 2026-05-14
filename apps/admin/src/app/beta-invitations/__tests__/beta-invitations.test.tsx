import { describe, it, expect } from "vitest"
import { render, screen } from "@testing-library/react"
import "@testing-library/jest-dom"
import BetaInvitationsPage from "../page"

describe("BetaInvitationsPage", () => {
  it("渲染不崩溃", () => {
    const { container } = render(<BetaInvitationsPage />)
    expect(container.firstChild).toBeTruthy()
  })

  it("显示邀请码列表（至少 3 行）", () => {
    render(<BetaInvitationsPage />)
    expect(screen.getByText("ABC12345")).toBeInTheDocument()
    expect(screen.getByText("XY67890Z")).toBeInTheDocument()
    expect(screen.getByText("PQRS1234")).toBeInTheDocument()
  })

  it("状态 Badge 颜色对应正确状态文字", () => {
    render(<BetaInvitationsPage />)
    const pendingBadge = screen.getByText("pending")
    expect(pendingBadge).toBeInTheDocument()
    const approvedBadge = screen.getByText("approved")
    expect(approvedBadge).toBeInTheDocument()
    const usedBadge = screen.getByText("used")
    expect(usedBadge).toBeInTheDocument()
  })

  it("pending 状态显示审批按钮", () => {
    render(<BetaInvitationsPage />)
    expect(screen.getByText("审批")).toBeInTheDocument()
  })

  it("approved 状态显示撤销按钮", () => {
    render(<BetaInvitationsPage />)
    expect(screen.getByText("撤销")).toBeInTheDocument()
  })
})
