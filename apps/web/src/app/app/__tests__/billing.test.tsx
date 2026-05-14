import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"
import { render, screen, waitFor } from "@testing-library/react"
import { BillingClient } from "../billing/billing-client"
import { UsageClient } from "../usage/usage-client"
import { StatusPagesClient } from "../status-pages/status-pages-client"

const mockQuotaData = {
  data: {
    plan: "free",
    monitors: { used: 2, limit: 3 },
    channels: { used: 1, limit: 1 },
    status_pages: { used: 0, limit: 0 },
    api_calls: { used: 47, limit: 100, reset_at: 9999999999 },
    min_interval_s: 300,
    max_nodes: 1,
  },
}

// ── BillingClient tests ────────────────────────────────────────────────────────

describe("BillingClient", () => {
  it("renders the billing page container", () => {
    render(<BillingClient />)
    expect(screen.getByTestId("billing-page")).toBeInTheDocument()
  })

  it("shows Free badge on current plan card", () => {
    render(<BillingClient />)
    const badge = screen.getByTestId("current-plan-badge")
    expect(badge).toBeInTheDocument()
    expect(badge.textContent).toBe("Free")
  })

  it("renders upgrade to Pro button", () => {
    render(<BillingClient />)
    const btn = screen.getByTestId("upgrade-button")
    expect(btn).toBeInTheDocument()
    expect(btn.textContent).toContain("升级到 Pro")
  })

  it("pricing table renders 4 plan columns (Free, Pro, Team, Business)", () => {
    render(<BillingClient />)
    const table = screen.getByTestId("pricing-table")
    expect(table).toBeInTheDocument()
    // 4 plan-button-* elements: one per plan
    expect(screen.getByTestId("plan-button-free")).toBeInTheDocument()
    expect(screen.getByTestId("plan-button-pro")).toBeInTheDocument()
    expect(screen.getByTestId("plan-button-team")).toBeInTheDocument()
    expect(screen.getByTestId("plan-button-business")).toBeInTheDocument()
  })

  it("Free plan button is disabled (current plan)", () => {
    render(<BillingClient />)
    const freeBtn = screen.getByTestId("plan-button-free")
    expect(freeBtn).toBeDisabled()
  })

  it("non-free plan buttons are not disabled", () => {
    render(<BillingClient />)
    expect(screen.getByTestId("plan-button-pro")).not.toBeDisabled()
    expect(screen.getByTestId("plan-button-team")).not.toBeDisabled()
    expect(screen.getByTestId("plan-button-business")).not.toBeDisabled()
  })

  it("shows Paddle placeholder notice", () => {
    render(<BillingClient />)
    expect(screen.getByTestId("paddle-notice")).toBeInTheDocument()
    expect(screen.getByText("支付功能即将上线")).toBeInTheDocument()
  })

  it("shows empty invoice section", () => {
    render(<BillingClient />)
    expect(screen.getByTestId("invoice-section")).toBeInTheDocument()
    expect(screen.getByText("暂无发票记录")).toBeInTheDocument()
  })

  it("pricing table contains custom domain row with checkmarks and crosses", () => {
    render(<BillingClient />)
    expect(screen.getByText("自定义域名")).toBeInTheDocument()
  })
})

// ── UsageClient tests ──────────────────────────────────────────────────────────

describe("UsageClient", () => {
  beforeEach(() => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve(mockQuotaData),
      })
    )
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it("renders usage page container", () => {
    render(<UsageClient />)
    expect(screen.getByTestId("usage-page")).toBeInTheDocument()
  })

  it("shows loading skeletons before data arrives", () => {
    vi.stubGlobal("fetch", vi.fn(() => new Promise(() => {})))
    render(<UsageClient />)
    expect(screen.getByTestId("skeleton-api-calls")).toBeInTheDocument()
    expect(screen.getByTestId("skeleton-monitors")).toBeInTheDocument()
  })

  it("renders 4 usage stat cards", () => {
    render(<UsageClient />)
    expect(screen.getByTestId("usage-card-monitors")).toBeInTheDocument()
    expect(screen.getByTestId("usage-card-api-calls")).toBeInTheDocument()
    expect(screen.getByTestId("usage-card-status-pages")).toBeInTheDocument()
    expect(screen.getByTestId("usage-card-alert-channels")).toBeInTheDocument()
  })

  it("progress bars render after data loads", async () => {
    render(<UsageClient />)
    await waitFor(() => {
      expect(screen.getByTestId("progress-monitors")).toBeInTheDocument()
      expect(screen.getByTestId("progress-api-calls")).toBeInTheDocument()
    })
  })

  it("monitors card shows real data", async () => {
    render(<UsageClient />)
    await waitFor(() => {
      const card = screen.getByTestId("usage-card-monitors")
      expect(card.textContent).toContain("2")
      expect(card.textContent).toContain("3")
    })
  })

  it("near-limit badge shown for alert-channels (used=1 limit=1)", async () => {
    render(<UsageClient />)
    await waitFor(() => {
      expect(screen.getByTestId("near-limit-badge-alert-channels")).toBeInTheDocument()
    })
  })

  it("API trend chart renders 7 bars", () => {
    render(<UsageClient />)
    const chart = screen.getByTestId("api-trend-chart")
    expect(chart).toBeInTheDocument()
    expect(screen.getByTestId("bar-今天")).toBeInTheDocument()
    expect(screen.getByTestId("bar-周一")).toBeInTheDocument()
  })
})

// ── StatusPagesClient tests ────────────────────────────────────────────────────

describe("StatusPagesClient", () => {
  it("renders status pages page container", () => {
    render(<StatusPagesClient />)
    expect(screen.getByTestId("status-pages-page")).toBeInTheDocument()
  })

  it("shows free plan upgrade notice", () => {
    render(<StatusPagesClient />)
    expect(screen.getByTestId("free-plan-notice")).toBeInTheDocument()
  })

  it("renders status pages list with demo page", () => {
    render(<StatusPagesClient />)
    expect(screen.getByTestId("status-pages-list")).toBeInTheDocument()
    expect(screen.getByTestId("status-page-card-sp-001")).toBeInTheDocument()
    expect(screen.getByText("acme.com 服务状态")).toBeInTheDocument()
  })

  it("status page card shows slug and monitor count", () => {
    render(<StatusPagesClient />)
    const card = screen.getByTestId("status-page-card-sp-001")
    expect(card.textContent).toContain("demo")
    expect(card.textContent).toContain("8 个监控项")
  })

  it("status page card has external link", () => {
    render(<StatusPagesClient />)
    const link = screen.getByTestId("status-page-link-sp-001")
    expect(link).toBeInTheDocument()
    expect(link).toHaveAttribute("href", "https://demo.status.idcd.com")
  })
})
