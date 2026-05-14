import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, fireEvent, waitFor } from "@testing-library/react"
import "@testing-library/jest-dom"

vi.mock("next/navigation", () => ({
  usePathname: () => "/app/status-pages",
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
}))

const mockStatusPages = [
  {
    id: "sp-001",
    name: "acme.com 服务状态",
    slug: "acme",
    is_public: true,
    overall_status: "operational",
    created_at: "2026-05-01T00:00:00Z",
  },
  {
    id: "sp-002",
    name: "beta 状态页",
    slug: "beta",
    is_public: false,
    overall_status: "degraded",
    created_at: "2026-05-02T00:00:00Z",
  },
]

vi.mock("@/lib/api", () => ({
  apiRequest: vi.fn(),
}))

import { apiRequest } from "@/lib/api"
import { StatusPagesClient } from "../status-pages-client"

const mockedApiRequest = apiRequest as ReturnType<typeof vi.fn>

beforeEach(() => {
  vi.clearAllMocks()
})

describe("StatusPagesClient", () => {
  it("renders the page container", async () => {
    mockedApiRequest.mockResolvedValueOnce({ data: { status_pages: mockStatusPages } })
    render(<StatusPagesClient />)
    expect(screen.getByTestId("status-pages-page")).toBeInTheDocument()
  })

  it("shows skeleton while loading", () => {
    mockedApiRequest.mockReturnValueOnce(new Promise(() => {}))
    render(<StatusPagesClient />)
    expect(screen.getByTestId("status-pages-skeleton")).toBeInTheDocument()
  })

  it("renders status pages list after load", async () => {
    mockedApiRequest.mockResolvedValueOnce({ data: { status_pages: mockStatusPages } })
    render(<StatusPagesClient />)
    await waitFor(() => {
      expect(screen.getByTestId("status-pages-list")).toBeInTheDocument()
    })
  })

  it("renders each status page card", async () => {
    mockedApiRequest.mockResolvedValueOnce({ data: { status_pages: mockStatusPages } })
    render(<StatusPagesClient />)
    await waitFor(() => {
      expect(screen.getByTestId("status-page-card-sp-001")).toBeInTheDocument()
      expect(screen.getByTestId("status-page-card-sp-002")).toBeInTheDocument()
    })
  })

  it("renders visit links for each status page", async () => {
    mockedApiRequest.mockResolvedValueOnce({ data: { status_pages: mockStatusPages } })
    render(<StatusPagesClient />)
    await waitFor(() => {
      expect(screen.getByTestId("status-page-link-sp-001")).toBeInTheDocument()
      expect(screen.getByTestId("status-page-link-sp-002")).toBeInTheDocument()
    })
  })

  it("shows empty state when no status pages", async () => {
    mockedApiRequest.mockResolvedValueOnce({ data: { status_pages: [] } })
    render(<StatusPagesClient />)
    await waitFor(() => {
      expect(screen.getByTestId("sp-empty-state")).toBeInTheDocument()
    })
  })

  it("shows error alert when API fails", async () => {
    mockedApiRequest.mockRejectedValueOnce(new Error("Server error"))
    render(<StatusPagesClient />)
    await waitFor(() => {
      expect(screen.getByTestId("sp-error-alert")).toBeInTheDocument()
    })
  })

  it("shows free-plan upgrade notice", async () => {
    mockedApiRequest.mockResolvedValueOnce({ data: { status_pages: [] } })
    render(<StatusPagesClient />)
    expect(screen.getByTestId("free-plan-notice")).toBeInTheDocument()
  })

  it("clicking 新建状态页 opens upgrade dialog on free plan", async () => {
    mockedApiRequest.mockResolvedValueOnce({ data: { status_pages: [] } })
    render(<StatusPagesClient />)
    fireEvent.click(screen.getByTestId("new-page-button"))
    await waitFor(() => {
      expect(screen.getByTestId("upgrade-dialog")).toBeInTheDocument()
    })
  })

  it("clicking delete shows confirm dialog", async () => {
    mockedApiRequest.mockResolvedValueOnce({ data: { status_pages: mockStatusPages } })
    render(<StatusPagesClient />)
    await waitFor(() => {
      expect(screen.getByTestId("delete-sp-btn-sp-001")).toBeInTheDocument()
    })
    fireEvent.click(screen.getByTestId("delete-sp-btn-sp-001"))
    await waitFor(() => {
      expect(screen.getByTestId("delete-confirm-dialog")).toBeInTheDocument()
    })
  })

  it("confirming delete calls DELETE API and removes item", async () => {
    mockedApiRequest.mockResolvedValueOnce({ data: { status_pages: mockStatusPages } })
    mockedApiRequest.mockResolvedValueOnce({}) // DELETE response
    render(<StatusPagesClient />)
    await waitFor(() => {
      expect(screen.getByTestId("delete-sp-btn-sp-001")).toBeInTheDocument()
    })
    fireEvent.click(screen.getByTestId("delete-sp-btn-sp-001"))
    await waitFor(() => {
      expect(screen.getByTestId("delete-confirm-button")).toBeInTheDocument()
    })
    fireEvent.click(screen.getByTestId("delete-confirm-button"))
    await waitFor(() => {
      expect(mockedApiRequest).toHaveBeenCalledWith("/v1/status-pages/sp-001", { method: "DELETE" })
    })
  })
})
