import { describe, it, expect, vi } from "vitest"
import { render, screen } from "@testing-library/react"
import "@testing-library/jest-dom"

// Mock clipboard API
Object.defineProperty(navigator, "clipboard", {
  value: { writeText: vi.fn().mockResolvedValue(undefined) },
  writable: true,
})

import ReferralPage from "../page"

describe("ReferralPage — 推荐计划", () => {
  it("渲染不崩溃", () => {
    render(<ReferralPage />)
    expect(screen.getByText("推荐计划")).toBeInTheDocument()
  })

  it("显示推荐码 IDCD-XYZ789", () => {
    render(<ReferralPage />)
    expect(screen.getByTestId("referral-code")).toHaveTextContent("IDCD-XYZ789")
  })

  it("复制按钮存在，aria-label 包含复制字样", () => {
    render(<ReferralPage />)
    const copyBtn = screen.getByTestId("copy-button")
    expect(copyBtn).toBeInTheDocument()
    expect(copyBtn.getAttribute("aria-label")).toContain("复制")
  })

  it("奖励记录表格渲染并有 credited badge", () => {
    render(<ReferralPage />)
    // Should have the credited badge for rwd-001
    expect(screen.getByTestId("status-badge-rwd-001")).toBeInTheDocument()
    expect(screen.getByTestId("status-badge-rwd-001")).toHaveTextContent("credited")
  })

  it("显示被推荐邮箱列表", () => {
    render(<ReferralPage />)
    expect(screen.getByText("alice@example.com")).toBeInTheDocument()
    expect(screen.getByText("bob@corp.com")).toBeInTheDocument()
    expect(screen.getByText("charlie@startup.io")).toBeInTheDocument()
  })

  it("显示统计数字：已推荐人数 3", () => {
    render(<ReferralPage />)
    expect(screen.getByText("3")).toBeInTheDocument()
  })

  it("显示待结算金额 ¥20.00", () => {
    render(<ReferralPage />)
    expect(screen.getByText("¥20.00")).toBeInTheDocument()
  })

  it("显示已结算金额 ¥10.00", () => {
    render(<ReferralPage />)
    const elements = screen.getAllByText("¥10.00")
    expect(elements.length).toBeGreaterThan(0)
  })
})
