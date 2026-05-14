import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, fireEvent, waitFor } from "@testing-library/react"
import "@testing-library/jest-dom"
import { RefundClient, type RefundFailedPayment } from "../refund-client"

const MOCK_PAYMENTS: RefundFailedPayment[] = [
  {
    id: "pay_test_001",
    user_id: "u_user1",
    invoice_id: "inv_abc",
    amount_cents: 9900,
    currency: "CNY",
    refund_retry_count: 2,
    refund_failed_at: "2026-05-14T08:00:00Z",
    created_at: "2026-05-10T10:00:00Z",
  },
  {
    id: "pay_test_002",
    user_id: "u_user2",
    amount_cents: 4900,
    currency: "CNY",
    refund_retry_count: 1,
    refund_failed_at: "2026-05-14T09:00:00Z",
    created_at: "2026-05-12T10:00:00Z",
  },
]

describe("RefundClient", () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it("应该渲染提示 alert（有记录时）", () => {
    render(<RefundClient initialPayments={MOCK_PAYMENTS} />)
    expect(screen.getByText("待处理退款失败记录")).toBeInTheDocument()
    // The alert description contains text split across elements: "共 <strong>2</strong> 笔..."
    // Check the combined textContent of the alert element
    const alert = screen.getByRole("alert")
    expect(alert.textContent).toContain("共")
    expect(alert.textContent).toContain("2")
    expect(alert.textContent).toContain("笔")
  })

  it("空列表时显示无记录 alert", () => {
    render(<RefundClient initialPayments={[]} />)
    expect(screen.getByText("无待处理记录")).toBeInTheDocument()
  })

  it("应该渲染所有支付行", () => {
    render(<RefundClient initialPayments={MOCK_PAYMENTS} />)
    expect(screen.getByText("pay_test_001")).toBeInTheDocument()
    expect(screen.getByText("pay_test_002")).toBeInTheDocument()
  })

  it("应该显示格式化金额", () => {
    render(<RefundClient initialPayments={MOCK_PAYMENTS} />)
    // 9900 cents = CNY 99.00
    expect(screen.getByText("CNY 99.00")).toBeInTheDocument()
    // 4900 cents = CNY 49.00
    expect(screen.getByText("CNY 49.00")).toBeInTheDocument()
  })

  it("应该渲染手动重试按钮", () => {
    render(<RefundClient initialPayments={MOCK_PAYMENTS} />)
    const retryButtons = screen.getAllByText("手动重试")
    expect(retryButtons).toHaveLength(2)
  })

  it("重试成功后从列表中移除该条记录", async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ data: { id: "pay_test_001", status: "refunded" } }),
    })
    vi.stubGlobal("fetch", mockFetch)

    render(<RefundClient initialPayments={MOCK_PAYMENTS} />)

    const retryButtons = screen.getAllByText("手动重试")
    fireEvent.click(retryButtons[0])

    await waitFor(() => {
      expect(screen.queryByText("pay_test_001")).not.toBeInTheDocument()
    })
    // pay_test_002 should still be visible
    expect(screen.getByText("pay_test_002")).toBeInTheDocument()
  })

  it("重试失败时显示错误信息", async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      json: async () => ({ error: { message: "internal server error" } }),
    })
    vi.stubGlobal("fetch", mockFetch)

    render(<RefundClient initialPayments={[MOCK_PAYMENTS[0]]} />)

    const retryButton = screen.getByText("手动重试")
    fireEvent.click(retryButton)

    await waitFor(() => {
      expect(screen.getByText("internal server error")).toBeInTheDocument()
    })
    // Payment should remain in list
    expect(screen.getByText("pay_test_001")).toBeInTheDocument()
  })

  it("组件渲染不崩溃（smoke test）", () => {
    const { container } = render(<RefundClient initialPayments={MOCK_PAYMENTS} />)
    expect(container.firstChild).toBeTruthy()
  })
})
