import { describe, it, expect } from "vitest"
import { render, screen } from "@testing-library/react"
import { BillingClient } from "../billing/billing-client"
import { UsageClient } from "../usage/usage-client"
import { StatusPagesClient } from "../status-pages/status-pages-client"

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
  it("renders usage page container", () => {
    render(<UsageClient />)
    expect(screen.getByTestId("usage-page")).toBeInTheDocument()
  })

  it("renders 4 usage stat cards", () => {
    render(<UsageClient />)
    expect(screen.getByTestId("usage-card-monitors")).toBeInTheDocument()
    expect(screen.getByTestId("usage-card-api-calls")).toBeInTheDocument()
    expect(screen.getByTestId("usage-card-retention")).toBeInTheDocument()
    expect(screen.getByTestId("usage-card-alert-channels")).toBeInTheDocument()
  })

  it("monitors card shows 2/3", () => {
    render(<UsageClient />)
    const card = screen.getByTestId("usage-card-monitors")
    expect(card.textContent).toContain("2")
    expect(card.textContent).toContain("3")
  })

  it("progress bars render for capped resources", () => {
    render(<UsageClient />)
    expect(screen.getByTestId("progress-monitors")).toBeInTheDocument()
    expect(screen.getByTestId("progress-api-calls")).toBeInTheDocument()
  })

  it("near-limit badge shown for alert-channels (at 100%)", () => {
    render(<UsageClient />)
    expect(screen.getByTestId("near-limit-badge-alert-channels")).toBeInTheDocument()
  })

  it("API trend chart renders 7 bars", () => {
    render(<UsageClient />)
    const chart = screen.getByTestId("api-trend-chart")
    expect(chart).toBeInTheDocument()
    // 7 days
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
