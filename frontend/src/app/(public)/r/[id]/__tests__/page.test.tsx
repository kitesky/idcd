import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen } from "@testing-library/react"
import "@testing-library/jest-dom"
import ReportPage from "../page"
import type { AnyReport } from "@/lib/diagnose-store"

const mockGetReport = vi.fn<(id: string) => Promise<AnyReport | null>>()

vi.mock("@/lib/diagnose-store", async () => {
  const actual = await vi.importActual<typeof import("@/lib/diagnose-store")>(
    "@/lib/diagnose-store",
  )
  return {
    ...actual,
    getReport: (id: string) => mockGetReport(id),
  }
})

vi.mock("next/navigation", () => ({
  notFound: () => {
    throw new Error("NEXT_NOT_FOUND")
  },
}))

describe("/r/[id] page", () => {
  beforeEach(() => {
    mockGetReport.mockReset()
  })

  it("404s when report is missing", async () => {
    mockGetReport.mockResolvedValue(null)
    await expect(
      ReportPage({ params: Promise.resolve({ id: "missing" }) }),
    ).rejects.toThrow("NEXT_NOT_FOUND")
  })

  it("renders the single-tool view for type=single reports", async () => {
    mockGetReport.mockResolvedValue({
      id: "rpt_single",
      type: "single",
      tool: "ping",
      target: "example.com",
      createdAt: "2026-05-16T10:00:00Z",
      taskId: "task_001",
      status: "completed",
      result: { node_id: "node-1", success: true, duration_ms: 24 },
    })
    const el = await ReportPage({ params: Promise.resolve({ id: "rpt_single" }) })
    render(el)
    expect(screen.getAllByText("example.com").length).toBeGreaterThan(0)
    expect(screen.getByText("Ping 拨测")).toBeInTheDocument()
    expect(screen.getByText(/返回 Ping 拨测/)).toBeInTheDocument()
  })

  it("renders the combo view for legacy reports without a type field", async () => {
    mockGetReport.mockResolvedValue({
      id: "rpt_combo",
      domain: "idcd.com",
      createdAt: "2026-05-16T10:00:00Z",
      doneCount: 5,
      errorCount: 2,
      checks: [
        { key: "dns", label: "DNS 解析", status: "done", summary: "OK" },
      ],
    })
    const el = await ReportPage({ params: Promise.resolve({ id: "rpt_combo" }) })
    render(el)
    expect(screen.getByText("idcd.com")).toBeInTheDocument()
    expect(screen.getByText("DNS 解析")).toBeInTheDocument()
    expect(screen.getByText(/返回诊断工具/)).toBeInTheDocument()
  })
})
