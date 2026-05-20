import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"
import { render, screen, waitFor } from "@testing-library/react"

vi.mock("@/lib/api/verdict", async (orig) => {
  const actual = await orig<typeof import("@/lib/api/verdict")>()
  return {
    ...actual,
    getVerdictOrder: vi.fn(),
    getVerdictReport: vi.fn(),
  }
})

import { getVerdictOrder, getVerdictReport } from "@/lib/api/verdict"
import { VerdictOrderDetailClient } from "../[id]/verdict-order-detail-client"

const mockGetOrder = vi.mocked(getVerdictOrder)
const mockGetReport = vi.mocked(getVerdictReport)

const baseOrder = {
  id: "ord_xyz",
  status: "delivered" as const,
  template: "incident" as const,
  target: "example.com",
  time_window_start: "2026-05-01T00:00:00Z",
  time_window_end: "2026-05-02T00:00:00Z",
  price_cny: 199,
  report_id: "rep_1",
  paid_at: "2026-05-01T01:00:00Z",
  delivered_at: "2026-05-01T01:30:00Z",
}

describe("VerdictOrderDetailClient", () => {
  beforeEach(() => {
    mockGetOrder.mockReset()
    mockGetReport.mockReset()
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it("renders skeleton while loading then order info", async () => {
    mockGetOrder.mockResolvedValueOnce(baseOrder)
    mockGetReport.mockResolvedValueOnce({
      id: "rep_1",
      order_id: "ord_xyz",
      pdf_url: "https://cdn/x.pdf",
      content_hash: "sha256:beef",
      tsa_provider: "digicert",
      tsa_time: "2026-05-01T01:25:00Z",
      self_verify_status: "pass",
      archived_url: "https://archive/x.pdf",
    })

    render(<VerdictOrderDetailClient orderId="ord_xyz" />)
    expect(screen.getByTestId("loading")).toBeInTheDocument()

    await waitFor(() => {
      expect(screen.getByTestId("verdict-order-detail")).toBeInTheDocument()
    })

    expect(screen.getByTestId("status-badge")).toHaveTextContent("已交付")
    expect(screen.getByTestId("download-pdf-btn")).toBeInTheDocument()
    expect(screen.getByTestId("archive-btn")).toBeInTheDocument()
    expect(screen.getByTestId("verify-cta")).toBeInTheDocument()
  })

  it("shows polling alert while generating", async () => {
    mockGetOrder.mockResolvedValueOnce({ ...baseOrder, status: "generating", report_id: undefined })

    render(<VerdictOrderDetailClient orderId="ord_xyz" />)
    await waitFor(() => {
      expect(screen.getByTestId("polling-alert")).toBeInTheDocument()
    })
    expect(screen.queryByTestId("report-card")).not.toBeInTheDocument()
  })

  it("renders failed alert when order failed", async () => {
    mockGetOrder.mockResolvedValueOnce({ ...baseOrder, status: "failed", report_id: undefined })
    render(<VerdictOrderDetailClient orderId="ord_xyz" />)
    await waitFor(() => {
      expect(screen.getByTestId("failed-alert")).toBeInTheDocument()
    })
  })

  it("renders top-level error when initial load fails", async () => {
    mockGetOrder.mockRejectedValueOnce(new Error("not found"))
    render(<VerdictOrderDetailClient orderId="ord_xyz" />)
    await waitFor(() => {
      expect(screen.getByTestId("order-error")).toBeInTheDocument()
    })
    expect(screen.getByText(/not found/)).toBeInTheDocument()
  })
})
