import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, waitFor } from "@testing-library/react"
import "@testing-library/jest-dom"

// ─── Mock next/navigation ─────────────────────────────────────────────────────
vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "ev_001" }),
  useRouter: () => ({ replace: vi.fn() }),
  usePathname: () => "/app/incidents/ev_001",
}))

// ─── Mock @/lib/api ───────────────────────────────────────────────────────────
vi.mock("@/lib/api", () => ({
  apiRequest: vi.fn(),
}))

import { apiRequest } from "@/lib/api"
import PostmortemDetailPage from "../page"

const mockApiRequest = vi.mocked(apiRequest)

const MOCK_PM = {
  id: "pm_001",
  alert_event_id: "ev_001",
  monitor_id: "mon_api",
  title: "[high] API Gateway 服务中断（47 分钟）",
  status: "published",
  severity: "high",
  impact: "API Gateway（http）检测到异常，影响持续约 47 分钟",
  timeline: [
    { time: "2026-05-13T14:23:00Z", event: "故障开始" },
    { time: "2026-05-13T15:10:00Z", event: "故障结束" },
  ],
  root_cause: "基础设施异常，节点过载",
  resolution: "扩容后恢复服务",
  action_items: [
    { item: "检查服务器负载", owner: "张三", due_date: "2026-05-21" },
  ],
  created_at: "2026-05-13T15:12:00Z",
  updated_at: "2026-05-13T16:00:00Z",
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe("PostmortemDetailPage", () => {
  it("显示 Skeleton 加载状态（pending 状态）", () => {
    // Never resolve so we stay in loading state
    mockApiRequest.mockImplementation(() => new Promise(() => {}))
    render(<PostmortemDetailPage />)
    expect(screen.getByTestId("postmortem-detail-page")).toBeInTheDocument()
  })

  it("成功加载后渲染复盘标题", async () => {
    mockApiRequest.mockResolvedValueOnce({ data: MOCK_PM })
    render(<PostmortemDetailPage />)
    await waitFor(() => {
      expect(screen.getByTestId("postmortem-title")).toHaveTextContent(
        "[high] API Gateway 服务中断（47 分钟）"
      )
    })
  })

  it("渲染 severity badge", async () => {
    mockApiRequest.mockResolvedValueOnce({ data: MOCK_PM })
    render(<PostmortemDetailPage />)
    await waitFor(() => {
      expect(screen.getByText("high")).toBeInTheDocument()
    })
  })

  it("渲染影响范围 Card", async () => {
    mockApiRequest.mockResolvedValueOnce({ data: MOCK_PM })
    render(<PostmortemDetailPage />)
    await waitFor(() => {
      expect(screen.getByTestId("impact-card")).toBeInTheDocument()
      expect(
        screen.getByText("API Gateway（http）检测到异常，影响持续约 47 分钟")
      ).toBeInTheDocument()
    })
  })

  it("渲染时间线 Card", async () => {
    mockApiRequest.mockResolvedValueOnce({ data: MOCK_PM })
    render(<PostmortemDetailPage />)
    await waitFor(() => {
      expect(screen.getByTestId("timeline-card")).toBeInTheDocument()
      expect(screen.getByText("故障开始")).toBeInTheDocument()
      expect(screen.getByText("故障结束")).toBeInTheDocument()
    })
  })

  it("渲染根因分析 Card", async () => {
    mockApiRequest.mockResolvedValueOnce({ data: MOCK_PM })
    render(<PostmortemDetailPage />)
    await waitFor(() => {
      expect(screen.getByTestId("rootcause-card")).toBeInTheDocument()
      expect(screen.getByText("基础设施异常，节点过载")).toBeInTheDocument()
    })
  })

  it("渲染处置方案 Card", async () => {
    mockApiRequest.mockResolvedValueOnce({ data: MOCK_PM })
    render(<PostmortemDetailPage />)
    await waitFor(() => {
      expect(screen.getByTestId("resolution-card")).toBeInTheDocument()
      expect(screen.getByText("扩容后恢复服务")).toBeInTheDocument()
    })
  })

  it("渲染改进措施 Card", async () => {
    mockApiRequest.mockResolvedValueOnce({ data: MOCK_PM })
    render(<PostmortemDetailPage />)
    await waitFor(() => {
      expect(screen.getByTestId("action-items-card")).toBeInTheDocument()
      expect(screen.getByText("检查服务器负载")).toBeInTheDocument()
      expect(screen.getByText(/张三/)).toBeInTheDocument()
    })
  })

  it("API 返回 not found 时显示 404 提示", async () => {
    mockApiRequest.mockRejectedValueOnce(new Error("not found"))
    render(<PostmortemDetailPage />)
    await waitFor(() => {
      expect(screen.getByTestId("postmortem-not-found")).toBeInTheDocument()
      expect(screen.getByText("未找到复盘记录")).toBeInTheDocument()
    })
  })

  it("API 返回通用错误时显示错误 Alert", async () => {
    mockApiRequest.mockRejectedValueOnce(new Error("网络错误"))
    render(<PostmortemDetailPage />)
    await waitFor(() => {
      expect(screen.getByTestId("postmortem-error")).toBeInTheDocument()
      expect(screen.getByText("网络错误")).toBeInTheDocument()
    })
  })

  it("调用正确的 API 路径 /v1/incidents/ev_001/postmortem", async () => {
    mockApiRequest.mockResolvedValueOnce({ data: MOCK_PM })
    render(<PostmortemDetailPage />)
    await waitFor(() => {
      expect(mockApiRequest).toHaveBeenCalledWith(
        "/v1/incidents/ev_001/postmortem"
      )
    })
  })
})
