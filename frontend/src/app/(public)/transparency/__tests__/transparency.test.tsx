import { render, screen } from "@testing-library/react"
import { describe, it, expect, vi, beforeEach } from "vitest"
import TransparencyPage from "../page"

const MOCK_TRANSPARENCY = {
  data: {
    overall_status: "operational",
    last_updated: "2026-05-15T00:00:00Z",
    platform_uptime: { "30d": 99.97, "90d": 99.95, "365d": 99.92 },
    nodes: { total: 127, online: 124, tier1: 60 },
    kms: {
      status: "operational",
      provider: "AWS KMS",
    },
    tsa: {
      providers: [
        { name: "DigiCert", status: "operational", last_check: "2026-05-15T00:00:00Z" },
        { name: "GlobalSign", status: "operational", last_check: "2026-05-15T00:00:00Z" },
      ],
    },
    recent_incidents: [
      {
        date: "2026-05-10",
        title: "API 网关短暂延迟升高",
        duration_min: 12,
        severity: "low",
        resolved: true,
      },
    ],
    appeal_stats: { total: 10, resolved: 9, avg_hours: 4.2 },
  },
}

beforeEach(() => {
  vi.stubGlobal(
    "fetch",
    vi.fn().mockResolvedValue({
      ok: true,
      json: async () => MOCK_TRANSPARENCY,
    }),
  )
})

describe("TransparencyPage", () => {
  it("renders without crashing", async () => {
    const page = await TransparencyPage()
    render(page)
    expect(screen.getByText("透明度报告")).toBeInTheDocument()
  })

  it("shows overall system status", async () => {
    const page = await TransparencyPage()
    render(page)
    expect(screen.getByTestId("overall-status")).toBeInTheDocument()
    expect(screen.getByText("● 所有系统运行正常")).toBeInTheDocument()
  })

  it("shows KMS card", async () => {
    const page = await TransparencyPage()
    render(page)
    expect(screen.getByTestId("kms-card")).toBeInTheDocument()
    expect(screen.getByText("KMS 信任根")).toBeInTheDocument()
  })

  it("shows TSA providers", async () => {
    const page = await TransparencyPage()
    render(page)
    expect(screen.getByTestId("tsa-providers")).toBeInTheDocument()
    expect(screen.getByText("DigiCert")).toBeInTheDocument()
    expect(screen.getByText("GlobalSign")).toBeInTheDocument()
  })

  it("shows node stats", async () => {
    const page = await TransparencyPage()
    render(page)
    expect(screen.getByText("节点覆盖")).toBeInTheDocument()
    expect(screen.getByText("总节点")).toBeInTheDocument()
    expect(screen.getByText("活跃节点")).toBeInTheDocument()
  })

  it("shows error state when fetch fails", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockRejectedValue(new Error("network error")),
    )
    const page = await TransparencyPage()
    render(page)
    expect(screen.getByTestId("error-state")).toBeInTheDocument()
  })

  it("shows error state when API returns non-ok", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({ ok: false, status: 500 }),
    )
    const page = await TransparencyPage()
    render(page)
    expect(screen.getByTestId("error-state")).toBeInTheDocument()
  })
})
