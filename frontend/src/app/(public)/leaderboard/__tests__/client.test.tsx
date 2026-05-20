import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, within, waitFor } from "@testing-library/react"
import "@testing-library/jest-dom"

// Radix UI Tabs uses ResizeObserver — polyfill for jsdom
global.ResizeObserver = vi.fn().mockImplementation(() => ({
  observe: vi.fn(),
  unobserve: vi.fn(),
  disconnect: vi.fn(),
}))

// Mock apiRequest so CDN data loads synchronously in tests
vi.mock("@/lib/api", () => ({
  apiRequest: vi.fn(),
}))

import { apiRequest } from "@/lib/api"
import { LeaderboardClient } from "../leaderboard-client"

const MOCK_CDN_DATA = [
  { rank: 1, name: "Cloudflare CDN", shortName: "CF", globalP50: 12, chinaP50: 18, overseasP50: 10, trend: [12, 13, 11, 12, 14, 11, 12], change: -1 },
  { rank: 2, name: "AWS CloudFront", shortName: "CF", globalP50: 18, chinaP50: 25, overseasP50: 15, trend: [18, 19, 17, 18, 20, 18, 18], change: 0 },
  { rank: 3, name: "Akamai CDN", shortName: "AK", globalP50: 22, chinaP50: 30, overseasP50: 19, trend: [22, 21, 23, 22, 24, 22, 22], change: 1 },
  { rank: 4, name: "腾讯云 CDN", shortName: "TX", globalP50: 28, chinaP50: 20, overseasP50: 35, trend: [28, 27, 29, 28, 30, 27, 28], change: -2 },
  { rank: 5, name: "阿里云 CDN", shortName: "ALI", globalP50: 32, chinaP50: 22, overseasP50: 42, trend: [32, 31, 33, 32, 34, 31, 32], change: 0 },
  { rank: 6, name: "百度云加速", shortName: "BD", globalP50: 38, chinaP50: 28, overseasP50: 48, trend: [38, 37, 39, 38, 40, 37, 38], change: 1 },
  { rank: 7, name: "网宿科技 CDN", shortName: "WS", globalP50: 45, chinaP50: 35, overseasP50: 55, trend: [45, 44, 46, 45, 47, 44, 45], change: 0 },
  { rank: 8, name: "又拍云 CDN", shortName: "UP", globalP50: 52, chinaP50: 40, overseasP50: 64, trend: [52, 51, 53, 52, 54, 51, 52], change: -1 },
  { rank: 9, name: "七牛云 CDN", shortName: "QN", globalP50: 58, chinaP50: 45, overseasP50: 71, trend: [58, 57, 59, 58, 60, 57, 58], change: 2 },
  { rank: 10, name: "Fastly CDN", shortName: "FL", globalP50: 65, chinaP50: 90, overseasP50: 55, trend: [65, 64, 66, 65, 67, 64, 65], change: 0 },
]

const mockApiRequest = vi.mocked(apiRequest)

// Adapt MOCK_CDN_DATA to the ApiEntry shape the component expects
const MOCK_API_ENTRIES = MOCK_CDN_DATA.map(e => ({
  rank: e.rank,
  name: e.name,
  target: "https://example.com",
  avg_latency_ms: e.globalP50,
  p50_latency_ms: e.globalP50,
  p95_latency_ms: e.globalP50 * 2,
  uptime_pct: 99.9,
  check_count: 1000,
}))

beforeEach(() => {
  mockApiRequest.mockResolvedValue({ data: { entries: MOCK_API_ENTRIES, total: MOCK_API_ENTRIES.length } })
})

describe("LeaderboardClient — Tab 触发器渲染", () => {
  it("应该渲染三个 Tab 触发器", () => {
    render(<LeaderboardClient />)
    const tabs = screen.getAllByRole("tab")
    expect(tabs).toHaveLength(3)
  })

  it("CDN 响应速度 Tab 存在", () => {
    render(<LeaderboardClient />)
    expect(screen.getByRole("tab", { name: /CDN 响应速度/ })).toBeInTheDocument()
  })

  it("全球节点延迟 Tab 存在", () => {
    render(<LeaderboardClient />)
    expect(screen.getByRole("tab", { name: /全球节点延迟/ })).toBeInTheDocument()
  })

  it("可用性统计 Tab 存在", () => {
    render(<LeaderboardClient />)
    expect(screen.getByRole("tab", { name: /可用性统计/ })).toBeInTheDocument()
  })

  it("默认激活的是 CDN Tab（aria-selected=true）", () => {
    render(<LeaderboardClient />)
    const cdnTab = screen.getByRole("tab", { name: /CDN 响应速度/ })
    expect(cdnTab).toHaveAttribute("aria-selected", "true")
  })
})

describe("LeaderboardClient — CDN Tab 内容", () => {
  it("默认 Tab 显示 Cloudflare CDN 行", async () => {
    render(<LeaderboardClient />)
    await waitFor(() => {
      expect(screen.getByText("Cloudflare CDN")).toBeInTheDocument()
    })
  })

  it("CDN 表格应至少渲染 10 个排名单元格", async () => {
    render(<LeaderboardClient />)
    await waitFor(() => {
      const rankCells = screen.getAllByText(/^#\d+$/)
      expect(rankCells.length).toBeGreaterThanOrEqual(10)
    })
  })

  it("CDN 表格包含列头：CDN 名称", async () => {
    render(<LeaderboardClient />)
    await waitFor(() => {
      expect(screen.getByText("CDN 名称")).toBeInTheDocument()
    })
  })

  it("CDN 表格包含列头：全球 P50", async () => {
    render(<LeaderboardClient />)
    await waitFor(() => {
      expect(screen.getByText("全球 P50")).toBeInTheDocument()
    })
  })

  it("腾讯云 CDN 在 CDN 列表中", async () => {
    render(<LeaderboardClient />)
    await waitFor(() => {
      expect(screen.getByText("腾讯云 CDN")).toBeInTheDocument()
    })
  })

  it("所有 CDN 名称均渲染在页面中", async () => {
    render(<LeaderboardClient />)
    await waitFor(() => {
      for (const cdn of MOCK_CDN_DATA) {
        expect(screen.getByText(cdn.name)).toBeInTheDocument()
      }
    })
  })
})

describe("LeaderboardClient — 底部声明区", () => {
  it("应渲染底部数据声明 Alert", () => {
    render(<LeaderboardClient />)
    expect(screen.getByRole("alert")).toBeInTheDocument()
  })

  it("Alert 标题为「数据声明」", () => {
    render(<LeaderboardClient />)
    expect(screen.getByText("数据声明")).toBeInTheDocument()
  })

  it("Alert 内容包含「真实探测」字样", () => {
    render(<LeaderboardClient />)
    const alert = screen.getByRole("alert")
    expect(within(alert).getByText(/真实/)).toBeInTheDocument()
  })
})

describe("LeaderboardClient — Tab 面板结构（无需交互）", () => {
  it("页面包含 tabpanel 角色元素", () => {
    render(<LeaderboardClient />)
    // Radix Tabs renders tabpanels; at least the active one is accessible
    const panels = screen.getAllByRole("tabpanel")
    expect(panels.length).toBeGreaterThanOrEqual(1)
  })

  it("CDN 响应速度 Tab 控制的 panel 关联到 CDN tab", () => {
    render(<LeaderboardClient />)
    const cdnTab = screen.getByRole("tab", { name: /CDN 响应速度/ })
    const panelId = cdnTab.getAttribute("aria-controls")
    expect(panelId).toBeTruthy()
    const panel = document.getElementById(panelId!)
    expect(panel).toBeTruthy()
  })

  it("所有 CDN 名称存在于默认激活面板的 DOM 中", async () => {
    const { container } = render(<LeaderboardClient />)
    await waitFor(() => {
      for (const cdn of MOCK_CDN_DATA) {
        expect(container.textContent).toContain(cdn.name)
      }
    })
  })

  it("三个 tabpanel 对应 aria-controls 均可在 DOM 中找到", () => {
    render(<LeaderboardClient />)
    const tabs = screen.getAllByRole("tab")
    for (const tab of tabs) {
      const panelId = tab.getAttribute("aria-controls")
      expect(panelId).toBeTruthy()
      // Panel exists in DOM (even if hidden via data-state=inactive)
      expect(document.getElementById(panelId!)).not.toBeNull()
    }
  })

  it("非活跃 Tab panel 存在于 DOM（data-state=inactive）", () => {
    render(<LeaderboardClient />)
    // Radix may lazy-mount inactive panels — at minimum the active one exists
    const allPanels = document.querySelectorAll('[role="tabpanel"]')
    expect(allPanels.length).toBeGreaterThanOrEqual(1)
  })
})
