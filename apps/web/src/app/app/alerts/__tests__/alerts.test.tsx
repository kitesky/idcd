import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, fireEvent, within, waitFor } from "@testing-library/react"
import "@testing-library/jest-dom"
import { AlertsClient } from "../alerts-client"
import {
  MOCK_ALERT_EVENTS,
  MOCK_ALERT_CHANNELS,
  MOCK_ALERT_POLICIES,
} from "../types"

// Mock the API module so tests don't hit a real server
vi.mock("@/lib/api", () => ({
  apiRequest: vi.fn(),
  API_BASE: "http://localhost:8080",
}))

// Mock next/navigation hooks used by alerts-client.tsx
vi.mock('next/navigation', () => ({
  useSearchParams: vi.fn(() => new URLSearchParams()),
  useRouter: vi.fn(() => ({ replace: vi.fn(), push: vi.fn(), back: vi.fn() })),
  usePathname: vi.fn(() => '/app/alerts'),
}))

// Mock next-intl so t('key') returns the key string
vi.mock('next-intl', () => ({
  useTranslations: () => (key: string, params?: Record<string, unknown>) => {
    if (params && typeof params === 'object') {
      return Object.entries(params).reduce<string>(
        (str, [k, v]) => str.replace(new RegExp(`\\{${k}\\}`, 'g'), String(v)),
        key
      )
    }
    return key
  },
  useLocale: () => 'zh',
}))

import { apiRequest } from "@/lib/api"
const mockApiRequest = vi.mocked(apiRequest)

// Notifications per channel returned by the mock API
const MOCK_API_NOTIFICATIONS: Record<string, { id: string; alert_event_id: string; status: "sent" | "failed" | "pending"; sent_at: string | null; error: string | null }[]> = {
  "ch-001": [
    { id: "n-001", alert_event_id: "ae-001", status: "sent", sent_at: new Date(Date.now() - 15 * 60_000).toISOString(), error: null },
    { id: "n-002", alert_event_id: "ae-002", status: "failed", sent_at: new Date(Date.now() - 3 * 3600_000).toISOString(), error: "connection timeout" },
    { id: "n-003", alert_event_id: "ae-003", status: "sent", sent_at: new Date(Date.now() - 5 * 3600_000).toISOString(), error: null },
  ],
  "ch-002": [
    { id: "n-004", alert_event_id: "ae-001", status: "sent", sent_at: new Date(Date.now() - 15 * 60_000).toISOString(), error: null },
    { id: "n-005", alert_event_id: "ae-002", status: "pending", sent_at: null, error: null },
  ],
  "ch-003": [],
}

const MOCK_MONITORS = [
  { id: "mon-001", name: "idcd.com 主站" },
  { id: "mon-002", name: "API 网关健康检查" },
  { id: "mon-003", name: "香港节点 Ping" },
]

function setupDefaultMocks() {
  mockApiRequest.mockImplementation(async (path: string, options?: RequestInit) => {
    const method = options?.method?.toUpperCase() ?? "GET"

    // Events
    if (path.startsWith("/v1/alert-events") && !path.includes("/ack") && method === "GET") {
      return { data: { events: MOCK_ALERT_EVENTS } }
    }
    if (path.startsWith("/v1/alert-events/") && path.endsWith("/ack") && method === "POST") {
      return {}
    }

    // Channels
    if (path === "/v1/alert-channels" && method === "GET") {
      return { data: { items: MOCK_ALERT_CHANNELS } }
    }
    if (path === "/v1/alert-channels" && method === "POST") {
      const body = JSON.parse(options?.body as string)
      const newCh = {
        id: `ch-new-${Date.now()}`,
        name: body.name,
        type: body.type,
        config: body.config?.target ?? "",
        verified: false,
      }
      return { data: { channel: newCh } }
    }
    if (path.startsWith("/v1/alert-channels/") && path.includes("/notifications") && method === "GET") {
      const channelId = path.split("/")[3]!
      return { data: { notifications: MOCK_API_NOTIFICATIONS[channelId] ?? [] } }
    }
    if (path.startsWith("/v1/alert-channels/") && method === "DELETE") {
      return {}
    }

    // Monitors (for PolicyForm)
    if (path === "/v1/monitors" && method === "GET") {
      return { data: { items: MOCK_MONITORS } }
    }

    // Policies
    if (path === "/v1/alert-policies" && method === "GET") {
      return { data: { items: MOCK_ALERT_POLICIES } }
    }
    if (path === "/v1/alert-policies" && method === "POST") {
      const body = JSON.parse(options?.body as string)
      const newPol = { id: `pol-new-${Date.now()}`, ...body }
      return { data: { policy: newPol } }
    }
    if (path.startsWith("/v1/alert-policies/") && method === "PATCH") {
      const id = path.split("/")[3]!
      const body = JSON.parse(options?.body as string)
      const existing = MOCK_ALERT_POLICIES.find((p) => p.id === id) ?? MOCK_ALERT_POLICIES[0]!
      return { data: { policy: { ...existing, ...body } } }
    }
    if (path.startsWith("/v1/alert-policies/") && method === "DELETE") {
      return {}
    }

    throw new Error(`Unmocked API call: ${method} ${path}`)
  })
}

beforeEach(() => {
  vi.clearAllMocks()
  setupDefaultMocks()
})

describe("AlertsClient — 事件历史 Tab", () => {
  it("默认显示事件历史 Tab", async () => {
    render(<AlertsClient />)
    const tab = screen.getByRole("tab", { name: "tabs.events" })
    expect(tab).toHaveAttribute("aria-selected", "true")
  })

  it("渲染所有 5 个告警事件行", async () => {
    render(<AlertsClient />)
    await waitFor(() => {
      MOCK_ALERT_EVENTS.forEach((evt) => {
        expect(screen.getByTestId(`event-row-${evt.id}`)).toBeInTheDocument()
      })
    })
  })

  it("firing 事件显示红色 destructive Badge（告警中）", async () => {
    render(<AlertsClient />)
    await waitFor(() => {
      const firingBadges = screen.getAllByText("events.status.firing")
      const firingCount = MOCK_ALERT_EVENTS.filter((e) => e.status === "firing").length
      expect(firingBadges.length).toBe(firingCount)
    })
  })

  it("resolved 事件显示绿色 Badge（已恢复）", async () => {
    render(<AlertsClient />)
    await waitFor(() => {
      const resolvedBadges = screen.getAllByText("events.status.resolved")
      const resolvedCount = MOCK_ALERT_EVENTS.filter((e) => e.status === "resolved").length
      expect(resolvedBadges.length).toBe(resolvedCount)
    })
  })

  it("acknowledged 事件显示 gray Badge（已确认）", async () => {
    render(<AlertsClient />)
    await waitFor(() => {
      const ackBadges = screen.getAllByText("events.status.acknowledged")
      const ackCount = MOCK_ALERT_EVENTS.filter((e) => e.status === "acknowledged").length
      expect(ackBadges.length).toBe(ackCount)
    })
  })

  it("firing 事件行有 Acknowledge 按钮", async () => {
    render(<AlertsClient />)
    await waitFor(() => {
      const firingEvents = MOCK_ALERT_EVENTS.filter((e) => e.status === "firing")
      firingEvents.forEach((evt) => {
        expect(screen.getByTestId(`ack-btn-${evt.id}`)).toBeInTheDocument()
      })
    })
  })

  it("非 firing 事件没有 Acknowledge 按钮", async () => {
    render(<AlertsClient />)
    await waitFor(() => {
      const nonFiringEvents = MOCK_ALERT_EVENTS.filter((e) => e.status !== "firing")
      nonFiringEvents.forEach((evt) => {
        expect(screen.queryByTestId(`ack-btn-${evt.id}`)).not.toBeInTheDocument()
      })
    })
  })

  it("顶部 Alert 展示当前 firing 事件数量", async () => {
    render(<AlertsClient />)
    await waitFor(() => {
      // firing-alert is rendered when there are firing events
      expect(screen.getByTestId("firing-alert")).toBeInTheDocument()
    })
  })

  it("点击 Acknowledge 按钮后，该事件状态变为已确认", async () => {
    render(<AlertsClient />)
    const firstFiring = MOCK_ALERT_EVENTS.find((e) => e.status === "firing")!
    await waitFor(() => {
      expect(screen.getByTestId(`ack-btn-${firstFiring.id}`)).toBeInTheDocument()
    })
    const ackBtn = screen.getByTestId(`ack-btn-${firstFiring.id}`)
    fireEvent.click(ackBtn)
    await waitFor(() => {
      expect(screen.queryByTestId(`ack-btn-${firstFiring.id}`)).not.toBeInTheDocument()
    })
  })

  it("监控名称显示在表格中", async () => {
    render(<AlertsClient />)
    await waitFor(() => {
      MOCK_ALERT_EVENTS.forEach((evt) => {
        const row = screen.getByTestId(`event-row-${evt.id}`)
        expect(within(row).getByText(evt.monitorName)).toBeInTheDocument()
      })
    })
  })
})

describe("AlertsClient — 告警通道 Tab", () => {
  it("切换到通道 Tab 显示通道内容", async () => {
    render(<AlertsClient />)
    fireEvent.mouseDown(screen.getByTestId("tab-channels"))
    await waitFor(() => {
      expect(screen.getByTestId("add-channel-btn")).toBeInTheDocument()
    })
  })

  it("渲染所有 3 个通道卡片", async () => {
    render(<AlertsClient />)
    fireEvent.mouseDown(screen.getByTestId("tab-channels"))
    await waitFor(() => {
      MOCK_ALERT_CHANNELS.forEach((ch) => {
        expect(screen.getByTestId(`channel-card-${ch.id}`)).toBeInTheDocument()
      })
    })
  })

  it("通道卡片显示通道名称", async () => {
    render(<AlertsClient />)
    fireEvent.mouseDown(screen.getByTestId("tab-channels"))
    await waitFor(() => {
      MOCK_ALERT_CHANNELS.forEach((ch) => {
        const card = screen.getByTestId(`channel-card-${ch.id}`)
        expect(within(card).getByText(ch.name)).toBeInTheDocument()
      })
    })
  })

  it("已验证通道显示已验证 Badge", async () => {
    render(<AlertsClient />)
    fireEvent.mouseDown(screen.getByTestId("tab-channels"))
    await waitFor(() => {
      const verifiedChannels = MOCK_ALERT_CHANNELS.filter((c) => c.verified)
      const verifiedBadges = screen.getAllByText("channels.verified")
      expect(verifiedBadges.length).toBe(verifiedChannels.length)
    })
  })

  it("未验证通道显示未验证 Badge", async () => {
    render(<AlertsClient />)
    fireEvent.mouseDown(screen.getByTestId("tab-channels"))
    await waitFor(() => {
      const unverifiedChannels = MOCK_ALERT_CHANNELS.filter((c) => !c.verified)
      const unverifiedBadges = screen.getAllByText("channels.unverified")
      expect(unverifiedBadges.length).toBe(unverifiedChannels.length)
    })
  })

  it("每个通道有测试发送按钮", async () => {
    render(<AlertsClient />)
    fireEvent.mouseDown(screen.getByTestId("tab-channels"))
    await waitFor(() => {
      MOCK_ALERT_CHANNELS.forEach((ch) => {
        expect(screen.getByTestId(`test-channel-btn-${ch.id}`)).toBeInTheDocument()
      })
    })
  })

  it("每个通道有删除按钮", async () => {
    render(<AlertsClient />)
    fireEvent.mouseDown(screen.getByTestId("tab-channels"))
    await waitFor(() => {
      MOCK_ALERT_CHANNELS.forEach((ch) => {
        expect(screen.getByTestId(`delete-channel-btn-${ch.id}`)).toBeInTheDocument()
      })
    })
  })

  it("点击添加通道按钮打开侧滑 Sheet", async () => {
    render(<AlertsClient />)
    fireEvent.mouseDown(screen.getByTestId("tab-channels"))
    await waitFor(() => expect(screen.getByTestId("add-channel-btn")).toBeInTheDocument())
    fireEvent.click(screen.getByTestId("add-channel-btn"))
    expect(screen.getByTestId("side-sheet")).toBeInTheDocument()
    expect(screen.getByText("channels.addSheet")).toBeInTheDocument()
  })

  it("点击删除通道按钮打开确认对话框", async () => {
    render(<AlertsClient />)
    fireEvent.mouseDown(screen.getByTestId("tab-channels"))
    await waitFor(() =>
      expect(screen.getByTestId(`delete-channel-btn-${MOCK_ALERT_CHANNELS[0]!.id}`)).toBeInTheDocument()
    )
    const deleteBtn = screen.getByTestId(`delete-channel-btn-${MOCK_ALERT_CHANNELS[0]!.id}`)
    fireEvent.click(deleteBtn)
    expect(screen.getByTestId("confirm-dialog")).toBeInTheDocument()
    expect(screen.getByText("channels.delete.title")).toBeInTheDocument()
  })

  it("通道 Card 显示「查看交付记录」按钮", async () => {
    render(<AlertsClient />)
    fireEvent.mouseDown(screen.getByTestId("tab-channels"))
    await waitFor(() => {
      MOCK_ALERT_CHANNELS.forEach((ch) => {
        expect(screen.getByTestId(`delivery-history-toggle-${ch.id}`)).toBeInTheDocument()
      })
    })
  })

  it("点击交付记录按钮后展开显示通知列表", async () => {
    render(<AlertsClient />)
    fireEvent.mouseDown(screen.getByTestId("tab-channels"))
    const firstCh = MOCK_ALERT_CHANNELS[0]!
    await waitFor(() =>
      expect(screen.getByTestId(`delivery-history-toggle-${firstCh.id}`)).toBeInTheDocument()
    )
    const toggle = screen.getByTestId(`delivery-history-toggle-${firstCh.id}`)
    fireEvent.click(toggle)
    // Wait for content to appear and API response to load
    const notifications = MOCK_API_NOTIFICATIONS[firstCh.id] ?? []
    await waitFor(() => {
      const content = screen.getByTestId(`delivery-history-content-${firstCh.id}`)
      expect(content).toBeInTheDocument()
      notifications.slice(0, 10).forEach((n) => {
        expect(within(content).getByTestId(`notif-row-${n.id}`)).toBeInTheDocument()
      })
    })
  })
})

describe("AlertsClient — 告警策略 Tab", () => {
  it("切换到策略 Tab 显示策略内容", async () => {
    render(<AlertsClient />)
    fireEvent.mouseDown(screen.getByTestId("tab-policies"))
    await waitFor(() => {
      expect(screen.getByTestId("add-policy-btn")).toBeInTheDocument()
    })
  })

  it("渲染所有 2 条策略行", async () => {
    render(<AlertsClient />)
    fireEvent.mouseDown(screen.getByTestId("tab-policies"))
    await waitFor(() => {
      MOCK_ALERT_POLICIES.forEach((pol) => {
        expect(screen.getByTestId(`policy-row-${pol.id}`)).toBeInTheDocument()
      })
    })
  })

  it("策略行显示策略名", async () => {
    render(<AlertsClient />)
    fireEvent.mouseDown(screen.getByTestId("tab-policies"))
    await waitFor(() => {
      MOCK_ALERT_POLICIES.forEach((pol) => {
        expect(screen.getByText(pol.name)).toBeInTheDocument()
      })
    })
  })

  it("每条策略有启用/关闭 Switch", async () => {
    render(<AlertsClient />)
    fireEvent.mouseDown(screen.getByTestId("tab-policies"))
    await waitFor(() => {
      MOCK_ALERT_POLICIES.forEach((pol) => {
        expect(screen.getByTestId(`policy-toggle-${pol.id}`)).toBeInTheDocument()
      })
    })
  })

  it("每条策略有编辑按钮", async () => {
    render(<AlertsClient />)
    fireEvent.mouseDown(screen.getByTestId("tab-policies"))
    await waitFor(() => {
      MOCK_ALERT_POLICIES.forEach((pol) => {
        expect(screen.getByTestId(`edit-policy-btn-${pol.id}`)).toBeInTheDocument()
      })
    })
  })

  it("每条策略有删除按钮", async () => {
    render(<AlertsClient />)
    fireEvent.mouseDown(screen.getByTestId("tab-policies"))
    await waitFor(() => {
      MOCK_ALERT_POLICIES.forEach((pol) => {
        expect(screen.getByTestId(`delete-policy-btn-${pol.id}`)).toBeInTheDocument()
      })
    })
  })

  it("点击 Toggle 切换策略启用状态", async () => {
    render(<AlertsClient />)
    fireEvent.mouseDown(screen.getByTestId("tab-policies"))
    const firstPol = MOCK_ALERT_POLICIES[0]!
    await waitFor(() =>
      expect(screen.getByTestId(`policy-toggle-${firstPol.id}`)).toBeInTheDocument()
    )
    const toggle = screen.getByTestId(`policy-toggle-${firstPol.id}`)
    // shadcn Switch renders as <button role="switch" aria-checked=...>
    const initialChecked = toggle.getAttribute("aria-checked") === "true"
    fireEvent.click(toggle)
    expect(toggle.getAttribute("aria-checked")).toBe(String(!initialChecked))
  })

  it("点击编辑按钮打开侧滑 Sheet（编辑模式）", async () => {
    render(<AlertsClient />)
    fireEvent.mouseDown(screen.getByTestId("tab-policies"))
    await waitFor(() =>
      expect(screen.getByTestId(`edit-policy-btn-${MOCK_ALERT_POLICIES[0]!.id}`)).toBeInTheDocument()
    )
    const editBtn = screen.getByTestId(`edit-policy-btn-${MOCK_ALERT_POLICIES[0]!.id}`)
    fireEvent.click(editBtn)
    expect(screen.getByTestId("side-sheet")).toBeInTheDocument()
    expect(screen.getByText("policies.editSheet")).toBeInTheDocument()
  })

  it("点击新建策略按钮打开侧滑 Sheet（新建模式）", async () => {
    render(<AlertsClient />)
    fireEvent.mouseDown(screen.getByTestId("tab-policies"))
    await waitFor(() => expect(screen.getByTestId("add-policy-btn")).toBeInTheDocument())
    fireEvent.click(screen.getByTestId("add-policy-btn"))
    expect(screen.getByTestId("side-sheet")).toBeInTheDocument()
    expect(screen.getByText("policies.addSheet")).toBeInTheDocument()
  })

  it("点击删除策略按钮打开确认对话框", async () => {
    render(<AlertsClient />)
    fireEvent.mouseDown(screen.getByTestId("tab-policies"))
    await waitFor(() =>
      expect(screen.getByTestId(`delete-policy-btn-${MOCK_ALERT_POLICIES[0]!.id}`)).toBeInTheDocument()
    )
    const deleteBtn = screen.getByTestId(`delete-policy-btn-${MOCK_ALERT_POLICIES[0]!.id}`)
    fireEvent.click(deleteBtn)
    expect(screen.getByTestId("confirm-dialog")).toBeInTheDocument()
    expect(screen.getByText("policies.delete.title")).toBeInTheDocument()
  })
})

describe("AlertsClient — 通用", () => {
  it("页面初始渲染不崩溃", () => {
    const { container } = render(<AlertsClient />)
    expect(container.firstChild).toBeTruthy()
  })

  it("三个 Tab 都存在", () => {
    render(<AlertsClient />)
    expect(screen.getByTestId("tab-events")).toBeInTheDocument()
    expect(screen.getByTestId("tab-channels")).toBeInTheDocument()
    expect(screen.getByTestId("tab-policies")).toBeInTheDocument()
  })

  it("Tab 切换正确更新 aria-selected", () => {
    render(<AlertsClient />)
    const channelsTab = screen.getByTestId("tab-channels")
    fireEvent.mouseDown(channelsTab)
    expect(channelsTab).toHaveAttribute("aria-selected", "true")
    expect(screen.getByTestId("tab-events")).toHaveAttribute("aria-selected", "false")
  })

  it("加载失败时显示错误 Alert", async () => {
    // EventsTab first calls /v1/monitors (silently ignored on error),
    // then calls /v1/alert-events — reject both so events call also fails
    mockApiRequest
      .mockRejectedValueOnce(new Error("monitors 加载失败"))
      .mockRejectedValueOnce(new Error("网络错误"))
    render(<AlertsClient />)
    await waitFor(() => {
      expect(screen.getByTestId("events-error")).toBeInTheDocument()
    })
  })
})
