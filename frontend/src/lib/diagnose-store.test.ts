import { describe, it, expect, vi, beforeEach } from "vitest"
import { saveReport, getReport } from "./diagnose-store"
import type { DiagnoseReport } from "./diagnose-store"

const mockReport: DiagnoseReport = {
  id: "rpt_test123",
  domain: "example.com",
  createdAt: "2026-05-15T00:00:00Z",
  checks: [
    {
      key: "dns",
      label: "DNS 解析",
      status: "done",
      summary: "解析到 1 条 A 记录",
    },
  ],
  doneCount: 1,
  errorCount: 0,
}

describe("diagnose-store", () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  describe("saveReport", () => {
    it("POSTs report to API and resolves without error", async () => {
      const fetchSpy = vi.spyOn(global, "fetch").mockResolvedValueOnce(
        new Response(JSON.stringify({ data: { id: "rpt_test123" } }), {
          status: 201,
        })
      )

      await expect(saveReport(mockReport)).resolves.toBeUndefined()

      expect(fetchSpy).toHaveBeenCalledOnce()
      const [url, opts] = fetchSpy.mock.calls[0]!
      expect(String(url)).toContain("/v1/diagnose/reports")
      expect(opts?.method).toBe("POST")
      const sent = JSON.parse(opts?.body as string) as DiagnoseReport
      expect(sent.id).toBe("rpt_test123")
      expect(sent.domain).toBe("example.com")
    })

    it("silently ignores fetch errors (best-effort)", async () => {
      vi.spyOn(global, "fetch").mockRejectedValueOnce(new Error("network error"))

      // Should resolve without throwing
      await expect(saveReport(mockReport)).resolves.toBeUndefined()
    })
  })

  describe("getReport", () => {
    it("returns parsed report on 200 response", async () => {
      vi.spyOn(global, "fetch").mockResolvedValueOnce(
        new Response(JSON.stringify(mockReport), { status: 200 })
      )

      const result = await getReport("rpt_test123")
      expect(result).not.toBeNull()
      expect(result?.id).toBe("rpt_test123")
      // getReport now returns AnyReport; narrow to DiagnoseReport for combo
      // assertions. The mock payload above has no `type` field so it lands on
      // the combo branch of the union.
      const combo = result as DiagnoseReport | null
      expect(combo?.domain).toBe("example.com")
      expect(combo?.checks).toHaveLength(1)
    })

    it("returns null on 404 response", async () => {
      vi.spyOn(global, "fetch").mockResolvedValueOnce(
        new Response(JSON.stringify({ error: { code: "NOT_FOUND" } }), {
          status: 404,
        })
      )

      const result = await getReport("nonexistent")
      expect(result).toBeNull()
    })

    it("returns null when fetch throws", async () => {
      vi.spyOn(global, "fetch").mockRejectedValueOnce(new Error("network error"))

      const result = await getReport("rpt_test123")
      expect(result).toBeNull()
    })

    it("calls the correct URL with no-store cache", async () => {
      const fetchSpy = vi.spyOn(global, "fetch").mockResolvedValueOnce(
        new Response(JSON.stringify(mockReport), { status: 200 })
      )

      await getReport("rpt_test123")

      const [url, opts] = fetchSpy.mock.calls[0]!
      expect(String(url)).toContain("/v1/diagnose/reports/rpt_test123")
      expect(opts?.cache).toBe("no-store")
    })
  })
})
