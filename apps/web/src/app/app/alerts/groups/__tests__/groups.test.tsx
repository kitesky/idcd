import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, waitFor, fireEvent } from "@testing-library/react"
import "@testing-library/jest-dom"

vi.mock("next/navigation", () => ({
  usePathname: () => "/app/alerts/groups",
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
import AlertGroupsPage from "../page"

const mockApiRequest = apiRequest as ReturnType<typeof vi.fn>

const MOCK_GROUPS = [
  {
    id: "grp_001",
    user_id: "u_001",
    name: "生产环境告警组",
    group_by: "monitor_prefix",
    group_value: "api-",
    wait_seconds: 60,
    created_at: "2024-03-01T09:00:00Z",
  },
  {
    id: "grp_002",
    user_id: "u_001",
    name: "数据库告警组",
    group_by: "tag",
    group_value: "db",
    wait_seconds: 120,
    created_at: "2024-03-05T14:30:00Z",
  },
]

describe("AlertGroupsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it("加载时显示 skeleton", () => {
    // Return a never-resolving promise to keep loading state
    mockApiRequest.mockReturnValue(new Promise(() => {}))
    render(<AlertGroupsPage />)
    const skeletons = document.querySelectorAll(".animate-pulse, [data-slot='skeleton']")
    expect(skeletons.length).toBeGreaterThan(0)
  })

  it("显示分组列表", async () => {
    mockApiRequest.mockResolvedValue({ items: MOCK_GROUPS })
    render(<AlertGroupsPage />)
    await waitFor(() => {
      expect(screen.getByText("生产环境告警组")).toBeInTheDocument()
    })
    expect(screen.getByText("数据库告警组")).toBeInTheDocument()
  })

  it("空状态时显示提示", async () => {
    mockApiRequest.mockResolvedValue({ items: [] })
    render(<AlertGroupsPage />)
    await waitFor(() => {
      expect(screen.getByText("暂无告警分组")).toBeInTheDocument()
    })
  })

  it("点击创建分组打开 dialog", async () => {
    mockApiRequest.mockResolvedValue({ items: [] })
    render(<AlertGroupsPage />)
    // Wait for loading to finish
    await waitFor(() => {
      expect(screen.getByText("暂无告警分组")).toBeInTheDocument()
    })
    const createButton = screen.getByRole("button", { name: /新建分组/ })
    fireEvent.click(createButton)
    await waitFor(() => {
      expect(screen.getByText("新建告警分组")).toBeInTheDocument()
    })
  })
})
