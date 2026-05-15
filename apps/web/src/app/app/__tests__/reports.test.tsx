import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, waitFor } from "@testing-library/react"
import ReportsPage from "../reports/page"

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: vi.fn() }),
  useSearchParams: () => ({ get: () => null }),
}))

vi.mock("@/lib/api", () => ({
  apiRequest: vi.fn(),
}))

import { apiRequest } from "@/lib/api"
const mockApiRequest = vi.mocked(apiRequest)

describe("ReportsPage", () => {
  beforeEach(() => {
    mockApiRequest.mockResolvedValue({ data: { entries: [] } })
  })

  it("renders the reports page container", async () => {
    render(<ReportsPage />)
    await waitFor(() => {
      expect(screen.getByTestId("reports-page")).toBeInTheDocument()
    })
  })

  it("renders the PDF export button", async () => {
    render(<ReportsPage />)
    await waitFor(() => {
      expect(screen.getByTestId("pdf-export-btn")).toBeInTheDocument()
    })
  })

  it("renders the CSV download button", async () => {
    render(<ReportsPage />)
    await waitFor(() => {
      expect(screen.getByTestId("csv-export-btn")).toBeInTheDocument()
    })
  })

  it("renders the granularity select with monthly as default", async () => {
    render(<ReportsPage />)
    await waitFor(() => {
      expect(screen.getByTestId("granularity-select")).toBeInTheDocument()
    })
  })

  it("renders the reports page container when granularity select is present", async () => {
    render(<ReportsPage />)
    await waitFor(() => {
      expect(screen.getByTestId("reports-page")).toBeInTheDocument()
      expect(screen.getByTestId("granularity-select")).toBeInTheDocument()
    })
  })
})
