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

vi.mock("next-intl", () => ({
  useTranslations: (ns: string) => (key: string, params?: Record<string, string | number>) => {
    const translations: Record<string, string> = {
      "status.statusPages.detail.backToList": "返回状态页列表",
      "status.statusPages.detail.notFound": "状态页不存在",
      "status.statusPages.detail.loadFailed": "加载失败",
      "status.statusPages.detail.publicUrl": "公开地址：",
      "status.statusPages.detail.basicInfo": "基本信息",
      "status.statusPages.detail.basicInfoDesc": "修改状态页名称、Slug 和可见性",
      "status.statusPages.detail.name": "名称",
      "status.statusPages.detail.namePlaceholder": "我的服务状态页",
      "status.statusPages.detail.slug": "Slug",
      "status.statusPages.detail.slugHint": "只允许小写字母、数字和连字符",
      "status.statusPages.detail.publicVisible": "公开可见",
      "status.statusPages.detail.publicVisibleDesc": "开启后，任何人可通过链接访问此状态页",
      "status.statusPages.detail.saving": "保存中…",
      "status.statusPages.detail.save": "保存",
      "status.statusPages.detail.saveSuccess": "保存成功",
      "status.statusPages.detail.nameSlugRequired": "名称和 Slug 不能为空",
      "status.statusPages.detail.saveFailed": "保存失败",
      "status.statusPages.detail.linkedMonitors": "关联监控",
      "status.statusPages.detail.linkedMonitorsDesc": "此状态页显示的监控项目",
      "status.statusPages.detail.addMonitor": "添加监控",
      "status.statusPages.detail.noLinkedMonitors": "暂无关联监控",
      "status.statusPages.detail.addFirstMonitor": "添加第一个监控",
      "status.statusPages.detail.removeMonitor": "移除关联监控",
      "status.statusPages.detail.removeMonitorDesc": "确定要从此状态页移除监控「{name}」吗？此操作不会删除监控本身。",
      "status.statusPages.detail.removing": "移除中…",
      "status.statusPages.detail.confirmRemove": "确认移除",
      "status.statusPages.detail.addMonitorDialog.title": "添加监控",
      "status.statusPages.detail.addMonitorDialog.desc": "选择要关联到此状态页的监控项目",
      "status.statusPages.detail.addMonitorDialog.noAvailable": "所有监控均已关联，或暂无可用监控",
      "status.statusPages.detail.addMonitorDialog.loading": "加载监控列表失败",
      "status.statusPages.detail.addMonitorDialog.adding": "添加中…",
      "status.statusPages.detail.addMonitorDialog.add": "添加",
      "status.statusPages.detail.addMonitorDialog.cancel": "取消",
    }
    const fullKey = `${ns}.${key}`
    const raw = translations[fullKey] ?? key
    if (!params) return raw
    return raw.replace(/\{(\w+)\}/g, (_, k) =>
      Object.prototype.hasOwnProperty.call(params, k) ? String(params[k]) : `{${k}}`,
    )
  },
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
