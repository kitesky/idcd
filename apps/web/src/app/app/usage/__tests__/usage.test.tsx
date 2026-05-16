import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, waitFor } from "@testing-library/react"
import { UsageClient } from "../usage-client"

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
}))

vi.mock("next-intl", () => ({
  useTranslations: () => (key: string) => key,
  useLocale: () => "zh",
}))

vi.mock("@/lib/api", () => ({
  apiRequest: vi.fn(),
  API_BASE: "http://localhost:8080",
}))

import { apiRequest } from "@/lib/api"
const mockApiRequest = vi.mocked(apiRequest)

const MOCK_QUOTA = {
  plan: "free",
  monitors: { used: 2, limit: 3 },
  channels: { used: 1, limit: 1 },
  status_pages: { used: 0, limit: 1 },
  api_calls: { used: 47, limit: 100, reset_at: Math.floor(Date.now() / 1000) + 86400 },
  min_interval_s: 60,
  max_nodes: 3,
}

const MOCK_POINTS = {
  balance: 1250,
  total_earned: 2000,
}

function setupMocks() {
  mockApiRequest.mockImplementation((path: string) => {
    if (path === "/v1/account/quota") {
      return Promise.resolve({ data: MOCK_QUOTA })
    }
    if (path === "/v1/account/points") {
      return Promise.resolve({ data: MOCK_POINTS })
    }
    return Promise.resolve({})
  })
}

describe("UsageClient", () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setupMocks()
  })

  it("renders the usage page", () => {
    render(<UsageClient />)
    expect(screen.getByTestId("usage-page")).toBeInTheDocument()
  })

  it("shows skeleton loading states initially", () => {
    render(<UsageClient />)
    expect(screen.getByTestId("skeleton-api-calls")).toBeInTheDocument()
    expect(screen.getByTestId("skeleton-monitors")).toBeInTheDocument()
  })

  it("renders usage stats after loading", async () => {
    render(<UsageClient />)
    await waitFor(() => expect(screen.getByTestId("usage-stats")).toBeInTheDocument())
    expect(screen.getByTestId("progress-api-calls")).toBeInTheDocument()
    expect(screen.getByTestId("progress-monitors")).toBeInTheDocument()
  })

  it("shows actual usage numbers after data loads", async () => {
    render(<UsageClient />)
    await waitFor(() => expect(screen.getByTestId("progress-api-calls")).toBeInTheDocument())
    // "47" appears in both the quota card and the trend chart bar label
    expect(screen.getAllByText("47").length).toBeGreaterThanOrEqual(1)
    expect(screen.getAllByText("2").length).toBeGreaterThanOrEqual(1)
  })

  it("shows points balance from API", async () => {
    render(<UsageClient />)
    await waitFor(() =>
      expect(screen.getByTestId("points-value")).toHaveTextContent("1,250")
    )
  })

  it("shows skeleton while points are loading", () => {
    render(<UsageClient />)
    expect(screen.getByTestId("skeleton-points")).toBeInTheDocument()
  })

  it("renders the points balance card", async () => {
    render(<UsageClient />)
    await waitFor(() => expect(screen.getByTestId("points-balance-card")).toBeInTheDocument())
    expect(screen.getByTestId("points-badge")).toBeInTheDocument()
    expect(screen.getByTestId("redeem-button")).toBeInTheDocument()
  })

  it("renders the API trend chart", () => {
    render(<UsageClient />)
    expect(screen.getByTestId("api-trend-card")).toBeInTheDocument()
    expect(screen.getByTestId("api-trend-chart")).toBeInTheDocument()
  })

  it("shows 0 points when points API fails", async () => {
    mockApiRequest.mockImplementation((path: string) => {
      if (path === "/v1/account/quota") return Promise.resolve({ data: MOCK_QUOTA })
      if (path === "/v1/account/points") return Promise.reject(new Error("failed"))
      return Promise.resolve({})
    })
    render(<UsageClient />)
    await waitFor(() => expect(screen.getByTestId("points-value")).toBeInTheDocument())
    expect(screen.getByTestId("points-value")).toHaveTextContent("0")
  })

  it("calls quota and points APIs on mount", async () => {
    render(<UsageClient />)
    await waitFor(() => expect(mockApiRequest).toHaveBeenCalledWith("/v1/account/quota"))
    expect(mockApiRequest).toHaveBeenCalledWith("/v1/account/points")
  })
})
