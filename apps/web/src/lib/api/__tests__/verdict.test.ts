import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"

vi.mock("@/lib/api", () => ({
  apiRequest: vi.fn(),
  API_BASE: "http://api.test",
}))

import { apiRequest } from "@/lib/api"
import {
  createVerdictOrder,
  getVerdictOrder,
  getVerdictReport,
  isPollingStatus,
  statusBadgeVariant,
  verifyPdf,
  VERDICT_STATUS_LABELS,
  VERDICT_TEMPLATE_LABELS,
} from "../verdict"

const mockApiRequest = vi.mocked(apiRequest)

describe("verdict api helpers", () => {
  beforeEach(() => {
    mockApiRequest.mockReset()
  })

  describe("statusBadgeVariant", () => {
    it("maps delivered -> success", () => {
      expect(statusBadgeVariant("delivered")).toBe("success")
    })
    it("maps paid + generating -> info", () => {
      expect(statusBadgeVariant("paid")).toBe("info")
      expect(statusBadgeVariant("generating")).toBe("info")
    })
    it("maps pending -> warning", () => {
      expect(statusBadgeVariant("pending")).toBe("warning")
    })
    it("maps failed + refunded -> destructive", () => {
      expect(statusBadgeVariant("failed")).toBe("destructive")
      expect(statusBadgeVariant("refunded")).toBe("destructive")
    })
  })

  describe("isPollingStatus", () => {
    it("polls while transient", () => {
      expect(isPollingStatus("paid")).toBe(true)
      expect(isPollingStatus("generating")).toBe(true)
    })
    it("does not poll terminal states", () => {
      expect(isPollingStatus("delivered")).toBe(false)
      expect(isPollingStatus("failed")).toBe(false)
      expect(isPollingStatus("refunded")).toBe(false)
      expect(isPollingStatus("pending")).toBe(false)
    })
  })

  describe("label maps", () => {
    it("has all 6 status labels", () => {
      expect(Object.keys(VERDICT_STATUS_LABELS)).toHaveLength(6)
    })
    it("has 4 templates", () => {
      expect(Object.keys(VERDICT_TEMPLATE_LABELS)).toHaveLength(4)
      expect(VERDICT_TEMPLATE_LABELS.sla).toBe("服务等级")
      expect(VERDICT_TEMPLATE_LABELS.legal).toBe("法律证据")
    })
  })

  describe("createVerdictOrder", () => {
    it("posts to /v1/verdict/orders and unwraps {data}", async () => {
      mockApiRequest.mockResolvedValueOnce({
        data: { order_id: "ord_1", pay_url: "https://pay/x", price_cny: 299 },
      })
      const out = await createVerdictOrder({
        template: "sla",
        target: "example.com",
        time_window_start: "2026-01-01T00:00:00Z",
        time_window_end: "2026-01-02T00:00:00Z",
        channel: "paddle",
        return_url: "https://idcd.com/app/verdict/{order_id}",
      })
      expect(out.order_id).toBe("ord_1")
      expect(out.pay_url).toBe("https://pay/x")
      expect(mockApiRequest).toHaveBeenCalledWith(
        "/v1/verdict/orders",
        expect.objectContaining({ method: "POST" }),
      )
    })

    it("accepts a bare (un-wrapped) response", async () => {
      mockApiRequest.mockResolvedValueOnce({
        order_id: "ord_2",
        pay_url: "https://pay/y",
        price_cny: 299,
      })
      const out = await createVerdictOrder({
        template: "incident",
        target: "example.com",
        time_window_start: "2026-01-01T00:00:00Z",
        time_window_end: "2026-01-02T00:00:00Z",
        channel: "alipay",
      })
      expect(out.order_id).toBe("ord_2")
    })
  })

  describe("getVerdictOrder", () => {
    it("calls /v1/verdict/orders/:id with encoded id", async () => {
      mockApiRequest.mockResolvedValueOnce({
        data: {
          id: "abc",
          status: "delivered",
          template: "sla",
          target: "example.com",
          time_window_start: "2026-01-01T00:00:00Z",
          time_window_end: "2026-01-02T00:00:00Z",
          price_cny: 99,
        },
      })
      const o = await getVerdictOrder("abc/with slash")
      expect(o.id).toBe("abc")
      expect(o.time_window_start).toBe("2026-01-01T00:00:00Z")
      expect(mockApiRequest).toHaveBeenCalledWith("/v1/verdict/orders/abc%2Fwith%20slash")
    })
  })

  describe("getVerdictReport", () => {
    it("fetches report by id", async () => {
      mockApiRequest.mockResolvedValueOnce({
        data: {
          id: "r1",
          order_id: "o1",
          pdf_url: "https://cdn/x.pdf",
          content_hash: "sha256:deadbeef",
          tsa_provider: "digicert",
          tsa_time: "2026-01-01T00:00:00Z",
          self_verify_status: "pass",
        },
      })
      const r = await getVerdictReport("r1")
      expect(r.pdf_url).toBe("https://cdn/x.pdf")
    })
  })
})

describe("verifyPdf", () => {
  const realFetch = global.fetch

  beforeEach(() => {
    global.fetch = vi.fn()
  })
  afterEach(() => {
    global.fetch = realFetch
  })

  it("posts multipart and unwraps response", async () => {
    ;(global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        valid: true,
        signature_chain: "chain",
        public_key_fingerprint: "fp",
        signed_at: "2026-01-01T00:00:00Z",
        tsa_provider: "digicert",
        content_hash: "sha256:abc",
        report_type: "observation_only",
        legal_disclaimer: "本报告为 idcd 提供的一手观测数据,不构成司法鉴定结论。",
      }),
    })

    const file = new File([new Uint8Array([1, 2, 3])], "report.pdf", {
      type: "application/pdf",
    })
    const result = await verifyPdf(file)
    expect(result.valid).toBe(true)
    expect(result.report_type).toBe("observation_only")
    expect(result.legal_disclaimer).toContain("一手观测数据")

    const [url, init] = (global.fetch as ReturnType<typeof vi.fn>).mock.calls[0]!
    expect(url).toBe("http://api.test/v1/attest/verify")
    expect(init.method).toBe("POST")
    expect(init.body).toBeInstanceOf(FormData)
  })

  it("throws backend error message on non-2xx", async () => {
    ;(global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: false,
      statusText: "Bad Request",
      json: async () => ({ error: { message: "invalid pdf" } }),
    })
    const file = new File([new Uint8Array([1])], "bad.pdf", { type: "application/pdf" })
    await expect(verifyPdf(file)).rejects.toThrow("invalid pdf")
  })
})
