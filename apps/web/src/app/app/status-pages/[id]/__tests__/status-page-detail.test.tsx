import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, waitFor, fireEvent } from "@testing-library/react"
import "@testing-library/jest-dom"

vi.mock("@/lib/api", () => ({ apiRequest: vi.fn() }))

vi.mock("next/navigation", () => ({
  useParams: vi.fn(() => ({ id: "sp_001" })),
  useRouter: vi.fn(() => ({ push: vi.fn() })),
}))

vi.mock("next/link", () => ({
  default: ({
    children,
    href,
    ...rest
  }: {
    children: React.ReactNode
    href: string
    [key: string]: unknown
  }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}))

import { apiRequest } from "@/lib/api"
import StatusPageDetailPage from "../page"

const mockedApiRequest = apiRequest as ReturnType<typeof vi.fn>

const MOCK_STATUS_PAGE = {
  id: "sp_001",
  name: "我的服务状态",
  slug: "my-service",
  is_public: true,
  overall_status: "operational",
  created_at: "2026-01-01T00:00:00Z",
}

const MOCK_MONITORS: {
  id: string
  monitor_id: string
  name: string
  type: string
  target: string
  status: string
  position: number
}[] = [
  {
    id: "lm_001",
    monitor_id: "mon_001",
    name: "API Health Check",
    type: "http",
    target: "https://api.example.com/health",
    status: "up",
    position: 0,
  },
  {
    id: "lm_002",
    monitor_id: "mon_002",
    name: "DB Latency",
    type: "tcp",
    target: "db.example.com:5432",
    status: "up",
    position: 1,
  },
]

beforeEach(() => {
  vi.clearAllMocks()
})

describe("StatusPageDetailPage", () => {
  it("加载时显示 skeleton", () => {
    // Keep the promises pending so loading stays true
    mockedApiRequest.mockReturnValue(new Promise(() => {}))
    render(<StatusPageDetailPage />)
    // The Skeleton component renders divs with an animate-pulse class
    const skeletons = document.querySelectorAll(".animate-pulse")
    expect(skeletons.length).toBeGreaterThan(0)
  })

  it("显示状态页名称和 slug", async () => {
    // First call: GET /v1/status-pages/sp_001 → returns single status page
    mockedApiRequest
      .mockResolvedValueOnce({ data: { status_page: MOCK_STATUS_PAGE } })
      // Second call: GET /v1/status-pages/sp_001/monitors
      .mockResolvedValueOnce({ data: { monitors: [] } })

    render(<StatusPageDetailPage />)

    await waitFor(() => {
      // Name input should contain the page name
      const nameInput = screen.getByLabelText("名称") as HTMLInputElement
      expect(nameInput.value).toBe("我的服务状态")
    })

    const slugInput = screen.getByLabelText("Slug") as HTMLInputElement
    expect(slugInput.value).toBe("my-service")
  })

  it("显示已关联监控列表", async () => {
    mockedApiRequest
      .mockResolvedValueOnce({ data: { status_page: MOCK_STATUS_PAGE } })
      .mockResolvedValueOnce({ data: { monitors: MOCK_MONITORS } })

    render(<StatusPageDetailPage />)

    await waitFor(() => {
      expect(screen.getByText("API Health Check")).toBeInTheDocument()
    })
    expect(screen.getByText("DB Latency")).toBeInTheDocument()
  })

  it("点击添加监控打开 dialog", async () => {
    mockedApiRequest
      .mockResolvedValueOnce({ data: { status_page: MOCK_STATUS_PAGE } })
      .mockResolvedValueOnce({ data: { monitors: [] } })
      // Third call triggered by opening the dialog: GET /v1/monitors
      .mockResolvedValueOnce({ data: { items: [] } })

    render(<StatusPageDetailPage />)

    // Wait for the page to finish loading
    await waitFor(() => {
      // Wait until the page content is loaded (name input appears)
      const nameInput = screen.getByLabelText("名称") as HTMLInputElement
      expect(nameInput.value).toBe("我的服务状态")
    })

    // Click the "添加监控" button (CardHeader button, not the empty-state one)
    // Use getByRole to find the button with partial text match
    const addButton = screen.getByRole("button", { name: /添加监控/ })
    fireEvent.click(addButton)

    // The Dialog containing "选择要关联到此状态页的监控项目" should appear
    await waitFor(() => {
      expect(screen.getByText("选择要关联到此状态页的监控项目")).toBeInTheDocument()
    })
  })
})
