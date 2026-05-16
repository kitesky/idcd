import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, waitFor, fireEvent } from "@testing-library/react"
import "@testing-library/jest-dom"

vi.mock("next/navigation", () => ({
  usePathname: () => "/app/nodes",
  useRouter: () => ({ replace: vi.fn() }),
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

vi.mock("@/lib/api", () => ({ apiRequest: vi.fn() }))

import { apiRequest } from "@/lib/api"
import NodesPage from "../page"

const mockApiRequest = apiRequest as ReturnType<typeof vi.fn>

const MOCK_APPLICATIONS = [
  {
    id: "app_001",
    user_id: "u_001",
    hostname: "node-sg-01.example.com",
    ip_address: "1.2.3.4",
    country: "SG",
    city: "Singapore",
    isp: "Tencent Cloud",
    status: "active" as const,
    created_at: "2024-01-15T10:00:00Z",
    updated_at: "2024-01-16T10:00:00Z",
  },
  {
    id: "app_002",
    user_id: "u_001",
    hostname: "node-us-01.example.com",
    ip_address: "5.6.7.8",
    country: "US",
    city: "Los Angeles",
    isp: "AWS",
    status: "pending" as const,
    created_at: "2024-02-10T08:00:00Z",
    updated_at: "2024-02-10T08:00:00Z",
  },
]

describe("NodesPage", () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it("加载时显示 skeleton", () => {
    // Return a never-resolving promise to keep loading state
    mockApiRequest.mockReturnValue(new Promise(() => {}))
    render(<NodesPage />)
    const skeletons = document.querySelectorAll(".animate-pulse, [data-slot='skeleton']")
    // Skeleton elements should be present while loading
    expect(skeletons.length).toBeGreaterThan(0)
  })

  it("显示节点申请列表", async () => {
    mockApiRequest.mockResolvedValue({ applications: MOCK_APPLICATIONS })
    render(<NodesPage />)
    await waitFor(() => {
      expect(screen.getByText("1.2.3.4")).toBeInTheDocument()
    })
    expect(screen.getByText("5.6.7.8")).toBeInTheDocument()
    expect(screen.getByText("SG")).toBeInTheDocument()
    expect(screen.getByText("US")).toBeInTheDocument()
  })

  it("空状态显示暂无节点申请", async () => {
    mockApiRequest.mockResolvedValue({ applications: [] })
    render(<NodesPage />)
    await waitFor(() => {
      expect(
        screen.getByText(/暂无节点申请/),
      ).toBeInTheDocument()
    })
  })

  it("点击申请新节点打开 dialog", async () => {
    mockApiRequest.mockResolvedValue({ applications: [] })
    render(<NodesPage />)
    // Wait for loading to finish
    await waitFor(() => {
      expect(screen.queryByText(/暂无节点申请/)).toBeInTheDocument()
    })
    const applyButton = screen.getByRole("button", { name: /申请新节点/ })
    fireEvent.click(applyButton)
    await waitFor(() => {
      expect(screen.getByText("申请贡献社区节点")).toBeInTheDocument()
    })
  })
})
