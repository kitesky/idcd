import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, waitFor, fireEvent } from "@testing-library/react"
import { BillingClient } from "../billing/billing-client"
import { UsageClient } from "../usage/usage-client"
import { StatusPagesClient } from "../status-pages/status-pages-client"

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: vi.fn() }),
  useSearchParams: () => ({ get: () => null }),
}))

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() },
}))

vi.mock("@/lib/api", () => ({
  apiRequest: vi.fn(),
  API_BASE: "http://localhost:8080",
  getSubscription: vi.fn(),
  getInvoices: vi.fn(),
  subscribePlan: vi.fn(),
  cancelSubscription: vi.fn(),
}))

import { apiRequest, getSubscription, getInvoices } from "@/lib/api"
const mockApiRequest = vi.mocked(apiRequest)
const mockGetSubscription = vi.mocked(getSubscription)
const mockGetInvoices = vi.mocked(getInvoices)

const emptyInvoices = { invoices: [], total: 0, page: 1, page_size: 20 }

// ── BillingClient tests ────────────────────────────────────────────────────────

describe("BillingClient", () => {
  beforeEach(() => {
    mockGetSubscription.mockResolvedValue(null) // no subscription = free
    mockGetInvoices.mockResolvedValue(emptyInvoices)
  })

  it("renders the billing page container", () => {
    render(<BillingClient />)
    expect(screen.getByTestId("billing-page")).toBeInTheDocument()
  })

  it("shows loading skeletons while fetching subscription", () => {
    mockGetSubscription.mockImplementation(() => new Promise(() => {}))
    render(<BillingClient />)
    expect(screen.getByTestId("current-plan-card")).toBeInTheDocument()
  })

  it("shows Free badge after subscription loads (no active subscription)", async () => {
    render(<BillingClient />)
    await waitFor(() => {
      const badge = screen.getByTestId("current-plan-badge")
      expect(badge.textContent).toBe("Free")
    })
  })

  it("shows upgrade button for free users", async () => {
    render(<BillingClient />)
    await waitFor(() => {
      expect(screen.getByTestId("upgrade-button")).toBeInTheDocument()
    })
  })

  it("upgrade button text says '升级到 Pro'", async () => {
    render(<BillingClient />)
    await waitFor(() => {
      expect(screen.getByTestId("upgrade-button").textContent).toContain("升级到 Pro")
    })
  })

  it("pricing table renders 4 plan columns", async () => {
    render(<BillingClient />)
    await waitFor(() => {
      expect(screen.getByTestId("pricing-table")).toBeInTheDocument()
      expect(screen.getByTestId("plan-button-free")).toBeInTheDocument()
      expect(screen.getByTestId("plan-button-pro")).toBeInTheDocument()
      expect(screen.getByTestId("plan-button-team")).toBeInTheDocument()
      expect(screen.getByTestId("plan-button-business")).toBeInTheDocument()
    })
  })

  it("Free plan button is disabled (current plan for free user)", async () => {
    render(<BillingClient />)
    await waitFor(() => {
      expect(screen.getByTestId("plan-button-free")).toBeDisabled()
    })
  })

  it("non-free plan buttons are enabled for free user", async () => {
    render(<BillingClient />)
    await waitFor(() => {
      expect(screen.getByTestId("plan-button-pro")).not.toBeDisabled()
      expect(screen.getByTestId("plan-button-team")).not.toBeDisabled()
      expect(screen.getByTestId("plan-button-business")).not.toBeDisabled()
    })
  })

  it("shows empty invoice section", async () => {
    render(<BillingClient />)
    await waitFor(() => {
      expect(screen.getByTestId("invoice-section")).toBeInTheDocument()
      expect(screen.getByTestId("empty-invoice-text")).toBeInTheDocument()
    })
  })

  it("pricing table shows custom domain row", async () => {
    render(<BillingClient />)
    await waitFor(() => {
      expect(screen.getByText("自定义域名")).toBeInTheDocument()
    })
  })

  it("upgrade button opens payment channel dialog", async () => {
    render(<BillingClient />)
    const btn = await screen.findByTestId("upgrade-button")
    fireEvent.click(btn)
    await screen.findByTestId("upgrade-dialog")
    expect(screen.getByTestId("upgrade-dialog")).toBeInTheDocument()
  })

  it("dialog shows alipay and wechat_pay options", async () => {
    render(<BillingClient />)
    const btn = await screen.findByTestId("upgrade-button")
    fireEvent.click(btn)
    await screen.findByText("支付宝")
    expect(screen.getByText("微信支付")).toBeInTheDocument()
  })

  it("shows Pro badge and cancel button when active on Pro plan", async () => {
    mockGetSubscription.mockResolvedValue({
      id: "sub_001",
      plan: "pro",
      status: "active",
      provider: "payment_hub",
      current_period_end: "2026-06-15T00:00:00Z",
      created_at: "2026-05-15T00:00:00Z",
    })
    render(<BillingClient />)
    await waitFor(() => {
      expect(screen.getByTestId("current-plan-badge").textContent).toBe("Pro")
      expect(screen.getByTestId("cancel-button")).toBeInTheDocument()
    })
  })

  it("shows invoice rows when invoices are returned", async () => {
    mockGetInvoices.mockResolvedValue({
      invoices: [
        {
          id: "inv_001",
          amount_cents: 9900,
          currency: "CNY",
          status: "paid",
          provider: "payment_hub",
          paid_at: "2026-05-15T10:00:00Z",
          created_at: "2026-05-15T10:00:00Z",
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
    })
    render(<BillingClient />)
    await waitFor(() => {
      expect(screen.getByTestId("invoice-row-inv_001")).toBeInTheDocument()
      expect(screen.getByText("¥99.00")).toBeInTheDocument()
    })
  })

  it("shows past_due alert when subscription is overdue", async () => {
    mockGetSubscription.mockResolvedValue({
      id: "sub_002",
      plan: "pro",
      status: "past_due",
      provider: "payment_hub",
      created_at: "2026-04-01T00:00:00Z",
    })
    render(<BillingClient />)
    await waitFor(() => {
      expect(screen.getByTestId("past-due-alert")).toBeInTheDocument()
    })
  })
})

// ── UsageClient tests ──────────────────────────────────────────────────────────

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

describe("UsageClient", () => {
  beforeEach(() => {
    mockApiRequest.mockImplementation((path: string) => {
      if (path === "/v1/account/quota") {
        return Promise.resolve(mockQuotaData)
      }
      if (path === "/v1/account/points") {
        return Promise.resolve({ data: { balance: 0, total_earned: 0 } })
      }
      return Promise.resolve({})
    })
  })

  it("renders usage page container", () => {
    render(<UsageClient />)
    expect(screen.getByTestId("usage-page")).toBeInTheDocument()
  })

  it("shows loading skeletons before data arrives", () => {
    mockApiRequest.mockImplementation(() => new Promise(() => {}))
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

const mockStatusPageApiData = {
  data: {
    status_pages: [
      {
        id: "sp-001",
        name: "acme.com 服务状态",
        slug: "demo",
        is_public: true,
        overall_status: "operational",
        created_at: "2026-05-01T00:00:00Z",
      },
    ],
  },
}

describe("StatusPagesClient", () => {
  beforeEach(() => {
    mockApiRequest.mockImplementation((url: string) => {
      if (url === "/v1/account/quota") return Promise.resolve({ data: { plan: "free" } })
      return Promise.resolve(mockStatusPageApiData)
    })
  })

  it("renders status pages page container", () => {
    render(<StatusPagesClient />)
    expect(screen.getByTestId("status-pages-page")).toBeInTheDocument()
  })

  it("shows free plan upgrade notice for free users", async () => {
    render(<StatusPagesClient />)
    await waitFor(() => {
      expect(screen.getByTestId("free-plan-notice")).toBeInTheDocument()
    })
  })

  it("renders status pages list after load", async () => {
    render(<StatusPagesClient />)
    await waitFor(() => {
      expect(screen.getByTestId("status-pages-list")).toBeInTheDocument()
      expect(screen.getByTestId("status-page-card-sp-001")).toBeInTheDocument()
      expect(screen.getByText("acme.com 服务状态")).toBeInTheDocument()
    })
  })

  it("status page card shows slug", async () => {
    render(<StatusPagesClient />)
    await waitFor(() => {
      const card = screen.getByTestId("status-page-card-sp-001")
      expect(card.textContent).toContain("demo")
    })
  })

  it("status page card has external link", async () => {
    render(<StatusPagesClient />)
    await waitFor(() => {
      const link = screen.getByTestId("status-page-link-sp-001")
      expect(link).toBeInTheDocument()
      expect(link).toHaveAttribute("href", "https://demo.status.idcd.com")
    })
  })
})
