import { describe, it, expect, vi } from "vitest"
import { render, screen, fireEvent, within } from "@testing-library/react"
import "@testing-library/jest-dom"
import { AlertsClient } from "../alerts-client"
import {
  MOCK_ALERT_EVENTS,
  MOCK_ALERT_CHANNELS,
  MOCK_ALERT_POLICIES,
} from "../mock-data"

// AlertsClient uses only standard React state — no external mocks needed.

describe("AlertsClient — 事件历史 Tab", () => {
  it("默认显示事件历史 Tab", () => {
    render(<AlertsClient />)
    const tab = screen.getByRole("tab", { name: "事件历史" })
    expect(tab).toHaveAttribute("aria-selected", "true")
  })

  it("渲染所有 5 个告警事件行", () => {
    render(<AlertsClient />)
    MOCK_ALERT_EVENTS.forEach((evt) => {
      expect(screen.getByTestId(`event-row-${evt.id}`)).toBeInTheDocument()
    })
  })

  it("firing 事件显示红色 destructive Badge（告警中）", () => {
    render(<AlertsClient />)
    const firingBadges = screen.getAllByText("告警中")
    const firingCount = MOCK_ALERT_EVENTS.filter((e) => e.status === "firing").length
    expect(firingBadges.length).toBe(firingCount)
  })

  it("resolved 事件显示绿色 Badge（已恢复）", () => {
    render(<AlertsClient />)
    const resolvedBadges = screen.getAllByText("已恢复")
    const resolvedCount = MOCK_ALERT_EVENTS.filter((e) => e.status === "resolved").length
    expect(resolvedBadges.length).toBe(resolvedCount)
  })

  it("acknowledged 事件显示 gray Badge（已确认）", () => {
    render(<AlertsClient />)
    const ackBadges = screen.getAllByText("已确认")
    const ackCount = MOCK_ALERT_EVENTS.filter((e) => e.status === "acknowledged").length
    expect(ackBadges.length).toBe(ackCount)
  })

  it("firing 事件行有 Acknowledge 按钮", () => {
    render(<AlertsClient />)
    const firingEvents = MOCK_ALERT_EVENTS.filter((e) => e.status === "firing")
    firingEvents.forEach((evt) => {
      expect(screen.getByTestId(`ack-btn-${evt.id}`)).toBeInTheDocument()
    })
  })

  it("非 firing 事件没有 Acknowledge 按钮", () => {
    render(<AlertsClient />)
    const nonFiringEvents = MOCK_ALERT_EVENTS.filter((e) => e.status !== "firing")
    nonFiringEvents.forEach((evt) => {
      expect(screen.queryByTestId(`ack-btn-${evt.id}`)).not.toBeInTheDocument()
    })
  })

  it("顶部 Alert 展示当前 firing 事件数量", () => {
    render(<AlertsClient />)
    const firingCount = MOCK_ALERT_EVENTS.filter((e) => e.status === "firing").length
    const alert = screen.getByTestId("firing-alert")
    expect(alert).toBeInTheDocument()
    expect(alert.textContent).toContain(String(firingCount))
  })

  it("点击 Acknowledge 按钮后，该事件状态变为已确认", () => {
    render(<AlertsClient />)
    const firstFiring = MOCK_ALERT_EVENTS.find((e) => e.status === "firing")!
    const ackBtn = screen.getByTestId(`ack-btn-${firstFiring.id}`)
    fireEvent.click(ackBtn)
    // After acknowledging, no more Acknowledge button for this event
    expect(screen.queryByTestId(`ack-btn-${firstFiring.id}`)).not.toBeInTheDocument()
  })

  it("监控名称显示在表格中", () => {
    render(<AlertsClient />)
    MOCK_ALERT_EVENTS.forEach((evt) => {
      const row = screen.getByTestId(`event-row-${evt.id}`)
      expect(within(row).getByText(evt.monitorName)).toBeInTheDocument()
    })
  })
})

describe("AlertsClient — 告警通道 Tab", () => {
  it("切换到通道 Tab 显示通道内容", () => {
    render(<AlertsClient />)
    fireEvent.click(screen.getByTestId("tab-channels"))
    expect(screen.getByTestId("add-channel-btn")).toBeInTheDocument()
  })

  it("渲染所有 3 个通道卡片", () => {
    render(<AlertsClient />)
    fireEvent.click(screen.getByTestId("tab-channels"))
    MOCK_ALERT_CHANNELS.forEach((ch) => {
      expect(screen.getByTestId(`channel-card-${ch.id}`)).toBeInTheDocument()
    })
  })

  it("通道卡片显示通道名称", () => {
    render(<AlertsClient />)
    fireEvent.click(screen.getByTestId("tab-channels"))
    MOCK_ALERT_CHANNELS.forEach((ch) => {
      const card = screen.getByTestId(`channel-card-${ch.id}`)
      expect(within(card).getByText(ch.name)).toBeInTheDocument()
    })
  })

  it("已验证通道显示已验证 Badge", () => {
    render(<AlertsClient />)
    fireEvent.click(screen.getByTestId("tab-channels"))
    const verifiedChannels = MOCK_ALERT_CHANNELS.filter((c) => c.verified)
    const verifiedBadges = screen.getAllByText("已验证")
    expect(verifiedBadges.length).toBe(verifiedChannels.length)
  })

  it("未验证通道显示未验证 Badge", () => {
    render(<AlertsClient />)
    fireEvent.click(screen.getByTestId("tab-channels"))
    const unverifiedChannels = MOCK_ALERT_CHANNELS.filter((c) => !c.verified)
    const unverifiedBadges = screen.getAllByText("未验证")
    expect(unverifiedBadges.length).toBe(unverifiedChannels.length)
  })

  it("每个通道有测试发送按钮", () => {
    render(<AlertsClient />)
    fireEvent.click(screen.getByTestId("tab-channels"))
    MOCK_ALERT_CHANNELS.forEach((ch) => {
      expect(screen.getByTestId(`test-channel-btn-${ch.id}`)).toBeInTheDocument()
    })
  })

  it("每个通道有删除按钮", () => {
    render(<AlertsClient />)
    fireEvent.click(screen.getByTestId("tab-channels"))
    MOCK_ALERT_CHANNELS.forEach((ch) => {
      expect(screen.getByTestId(`delete-channel-btn-${ch.id}`)).toBeInTheDocument()
    })
  })

  it("点击添加通道按钮打开侧滑 Sheet", () => {
    render(<AlertsClient />)
    fireEvent.click(screen.getByTestId("tab-channels"))
    fireEvent.click(screen.getByTestId("add-channel-btn"))
    expect(screen.getByTestId("side-sheet")).toBeInTheDocument()
    expect(screen.getByText("添加告警通道")).toBeInTheDocument()
  })

  it("点击删除通道按钮打开确认对话框", () => {
    render(<AlertsClient />)
    fireEvent.click(screen.getByTestId("tab-channels"))
    const deleteBtn = screen.getByTestId(`delete-channel-btn-${MOCK_ALERT_CHANNELS[0].id}`)
    fireEvent.click(deleteBtn)
    expect(screen.getByTestId("confirm-dialog")).toBeInTheDocument()
    expect(screen.getByText("删除通道")).toBeInTheDocument()
  })
})

describe("AlertsClient — 告警策略 Tab", () => {
  it("切换到策略 Tab 显示策略内容", () => {
    render(<AlertsClient />)
    fireEvent.click(screen.getByTestId("tab-policies"))
    expect(screen.getByTestId("add-policy-btn")).toBeInTheDocument()
  })

  it("渲染所有 2 条策略行", () => {
    render(<AlertsClient />)
    fireEvent.click(screen.getByTestId("tab-policies"))
    MOCK_ALERT_POLICIES.forEach((pol) => {
      expect(screen.getByTestId(`policy-row-${pol.id}`)).toBeInTheDocument()
    })
  })

  it("策略行显示策略名", () => {
    render(<AlertsClient />)
    fireEvent.click(screen.getByTestId("tab-policies"))
    MOCK_ALERT_POLICIES.forEach((pol) => {
      expect(screen.getByText(pol.name)).toBeInTheDocument()
    })
  })

  it("每条策略有启用/关闭 Switch", () => {
    render(<AlertsClient />)
    fireEvent.click(screen.getByTestId("tab-policies"))
    MOCK_ALERT_POLICIES.forEach((pol) => {
      expect(screen.getByTestId(`policy-toggle-${pol.id}`)).toBeInTheDocument()
    })
  })

  it("每条策略有编辑按钮", () => {
    render(<AlertsClient />)
    fireEvent.click(screen.getByTestId("tab-policies"))
    MOCK_ALERT_POLICIES.forEach((pol) => {
      expect(screen.getByTestId(`edit-policy-btn-${pol.id}`)).toBeInTheDocument()
    })
  })

  it("每条策略有删除按钮", () => {
    render(<AlertsClient />)
    fireEvent.click(screen.getByTestId("tab-policies"))
    MOCK_ALERT_POLICIES.forEach((pol) => {
      expect(screen.getByTestId(`delete-policy-btn-${pol.id}`)).toBeInTheDocument()
    })
  })

  it("点击 Toggle 切换策略启用状态", () => {
    render(<AlertsClient />)
    fireEvent.click(screen.getByTestId("tab-policies"))
    const firstPol = MOCK_ALERT_POLICIES[0]
    const toggle = screen.getByTestId(`policy-toggle-${firstPol.id}`) as HTMLInputElement
    const initialChecked = toggle.checked
    fireEvent.click(toggle)
    expect(toggle.checked).toBe(!initialChecked)
  })

  it("点击编辑按钮打开侧滑 Sheet（编辑模式）", () => {
    render(<AlertsClient />)
    fireEvent.click(screen.getByTestId("tab-policies"))
    const editBtn = screen.getByTestId(`edit-policy-btn-${MOCK_ALERT_POLICIES[0].id}`)
    fireEvent.click(editBtn)
    expect(screen.getByTestId("side-sheet")).toBeInTheDocument()
    expect(screen.getByText("编辑告警策略")).toBeInTheDocument()
  })

  it("点击新建策略按钮打开侧滑 Sheet（新建模式）", () => {
    render(<AlertsClient />)
    fireEvent.click(screen.getByTestId("tab-policies"))
    fireEvent.click(screen.getByTestId("add-policy-btn"))
    expect(screen.getByTestId("side-sheet")).toBeInTheDocument()
    expect(screen.getByText("新建告警策略")).toBeInTheDocument()
  })

  it("点击删除策略按钮打开确认对话框", () => {
    render(<AlertsClient />)
    fireEvent.click(screen.getByTestId("tab-policies"))
    const deleteBtn = screen.getByTestId(`delete-policy-btn-${MOCK_ALERT_POLICIES[0].id}`)
    fireEvent.click(deleteBtn)
    expect(screen.getByTestId("confirm-dialog")).toBeInTheDocument()
    expect(screen.getByText("删除策略")).toBeInTheDocument()
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
    fireEvent.click(channelsTab)
    expect(channelsTab).toHaveAttribute("aria-selected", "true")
    expect(screen.getByTestId("tab-events")).toHaveAttribute("aria-selected", "false")
  })
})
