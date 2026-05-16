import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, fireEvent, waitFor } from "@testing-library/react"
import "@testing-library/jest-dom"

// ─── Mock clipboard API ───────────────────────────────────────────────────────
Object.defineProperty(navigator, "clipboard", {
  value: { writeText: vi.fn().mockResolvedValue(undefined) },
  writable: true,
})

// ─── Mock @/lib/api ───────────────────────────────────────────────────────────
vi.mock("@/lib/api", () => ({
  apiRequest: vi.fn(),
}))

// ─── Mock next-intl ───────────────────────────────────────────────────────────
vi.mock("next-intl", () => ({
  useTranslations: () => (key: string) => key,
  useLocale: () => "cn",
}))

import { apiRequest } from "@/lib/api"
import ReferralPage from "../page"

const mockApiRequest = vi.mocked(apiRequest)

const MOCK_CODE_RESPONSE = {
  data: {
    code: "IDCD-XYZ789",
    url: "https://idcd.com/?ref=IDCD-XYZ789",
    uses_count: 0,
  },
}

const MOCK_REWARDS_RESPONSE = {
  data: {
    rewards: [
      {
        id: "rwd-001",
        referred_user_id: "user_alice",
        status: "credited" as const,
        amount: "10.00",
        currency: "CNY",
        created_at: "2026-05-10T10:00:00Z",
      },
      {
        id: "rwd-002",
        referred_user_id: "user_bob",
        status: "pending" as const,
        amount: "10.00",
        currency: "CNY",
        created_at: "2026-05-12T14:00:00Z",
      },
      {
        id: "rwd-003",
        referred_user_id: "user_charlie",
        status: "pending" as const,
        amount: "10.00",
        currency: "CNY",
        created_at: "2026-05-14T09:00:00Z",
      },
    ],
  },
}

function setupSuccessMocks() {
  mockApiRequest
    .mockResolvedValueOnce(MOCK_CODE_RESPONSE)
    .mockResolvedValueOnce(MOCK_REWARDS_RESPONSE)
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe("ReferralPage — 推荐计划", () => {
  it("渲染不崩溃，显示推荐计划标题", async () => {
    setupSuccessMocks()
    render(<ReferralPage />)
    expect(screen.getByText("推荐计划")).toBeInTheDocument()
  })

  it("加载中显示 Skeleton（API pending 状态）", () => {
    mockApiRequest.mockImplementation(() => new Promise(() => {}))
    render(<ReferralPage />)
    // Page container still renders during loading
    expect(screen.getByText("推荐计划")).toBeInTheDocument()
  })

  it("成功加载后显示推荐码 IDCD-XYZ789", async () => {
    setupSuccessMocks()
    render(<ReferralPage />)
    await waitFor(() => {
      expect(screen.getByTestId("referral-code")).toHaveTextContent("IDCD-XYZ789")
    })
  })

  it("复制按钮存在，aria-label 包含复制字样", async () => {
    setupSuccessMocks()
    render(<ReferralPage />)
    await waitFor(() => {
      const copyBtn = screen.getByTestId("copy-button")
      expect(copyBtn).toBeInTheDocument()
      expect(copyBtn.getAttribute("aria-label")).toContain("复制")
    })
  })

  it("点击复制按钮调用 clipboard.writeText 传入 url", async () => {
    setupSuccessMocks()
    render(<ReferralPage />)
    await waitFor(() => {
      expect(screen.getByTestId("copy-button")).not.toBeDisabled()
    })
    fireEvent.click(screen.getByTestId("copy-button"))
    expect(navigator.clipboard.writeText).toHaveBeenCalledWith(
      "https://idcd.com/?ref=IDCD-XYZ789"
    )
  })

  it("奖励记录表格渲染，有 status badge", async () => {
    setupSuccessMocks()
    render(<ReferralPage />)
    await waitFor(() => {
      expect(screen.getByTestId("status-badge-rwd-001")).toBeInTheDocument()
      expect(screen.getByTestId("status-badge-rwd-001")).toHaveTextContent("credited")
    })
  })

  it("渲染 3 条奖励记录的 status badge", async () => {
    setupSuccessMocks()
    render(<ReferralPage />)
    await waitFor(() => {
      expect(screen.getByTestId("status-badge-rwd-002")).toHaveTextContent("pending")
      expect(screen.getByTestId("status-badge-rwd-003")).toHaveTextContent("pending")
    })
  })

  it("显示统计数字：已推荐人数 3", async () => {
    setupSuccessMocks()
    render(<ReferralPage />)
    await waitFor(() => {
      expect(screen.getByTestId("total-referrals")).toHaveTextContent("3")
    })
  })

  it("显示待结算金额 ¥20.00", async () => {
    setupSuccessMocks()
    render(<ReferralPage />)
    await waitFor(() => {
      expect(screen.getByTestId("total-pending")).toHaveTextContent("¥20.00")
    })
  })

  it("显示已结算金额 ¥10.00", async () => {
    setupSuccessMocks()
    render(<ReferralPage />)
    await waitFor(() => {
      expect(screen.getByTestId("total-credited")).toHaveTextContent("¥10.00")
    })
  })

  it("API 失败时显示错误 Alert", async () => {
    mockApiRequest.mockRejectedValueOnce(new Error("网络错误"))
    render(<ReferralPage />)
    await waitFor(() => {
      expect(screen.getByTestId("referral-error")).toBeInTheDocument()
      expect(screen.getByText("网络错误")).toBeInTheDocument()
    })
  })

  it("无奖励记录时显示暂无记录提示", async () => {
    mockApiRequest
      .mockResolvedValueOnce(MOCK_CODE_RESPONSE)
      .mockResolvedValueOnce({ data: { rewards: [] } })
    render(<ReferralPage />)
    await waitFor(() => {
      expect(screen.getByTestId("no-rewards")).toBeInTheDocument()
    })
  })

  it("调用正确的 API 路径", async () => {
    setupSuccessMocks()
    render(<ReferralPage />)
    await waitFor(() => {
      expect(mockApiRequest).toHaveBeenCalledWith("/v1/referral/code", { method: "POST" })
      expect(mockApiRequest).toHaveBeenCalledWith("/v1/referral/rewards")
    })
  })
})
