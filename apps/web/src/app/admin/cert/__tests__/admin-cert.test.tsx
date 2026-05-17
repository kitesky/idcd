import { render, screen, fireEvent, waitFor } from "@testing-library/react"
import { describe, it, expect, vi, beforeEach } from "vitest"
import {
  buildQuery,
  formatRate,
  formatPercent,
  listOrders,
  forceFail,
  banAccount,
  getCAQuota,
  getDNSHealth,
  type AdminCertOrder,
  type AdminCAQuotaResponse,
  type AdminDNSHealthResponse,
} from "../admin-cert-api"
import { OrdersAdminClient } from "../orders/orders-admin-client"
import { CAQuotaClient } from "../quota/quota-client"
import { DNSHealthClient } from "../dns-health/dns-health-client"
import { AbuseClient } from "../abuse/abuse-client"

// ---------- admin-cert-api.ts -------------------------------------------------

describe("admin-cert-api utilities", () => {
  describe("buildQuery", () => {
    it("returns empty string when all values are empty", () => {
      expect(buildQuery({})).toBe("")
      expect(buildQuery({ a: undefined, b: null, c: "" })).toBe("")
    })
    it("trims values and skips blanks", () => {
      expect(buildQuery({ status: "  ", ca: "le" })).toBe("?ca=le")
    })
    it("encodes multiple params", () => {
      const q = buildQuery({ status: "issued", account_id: 42, ca: "le" })
      // URLSearchParams order is insertion order
      expect(q).toContain("status=issued")
      expect(q).toContain("account_id=42")
      expect(q).toContain("ca=le")
      expect(q.startsWith("?")).toBe(true)
    })
  })

  describe("formatRate", () => {
    it("returns unknown label for -1 sentinel", () => {
      expect(formatRate(-1)).toBe("—")
      expect(formatRate(-1, "n/a")).toBe("n/a")
    })
    it("returns unknown label for NaN", () => {
      expect(formatRate(Number.NaN)).toBe("—")
    })
    it("renders 0..1 as percentage", () => {
      expect(formatRate(0.95)).toBe("95.0%")
      expect(formatRate(1)).toBe("100.0%")
    })
  })

  describe("formatPercent", () => {
    it("clamps values to 0..1", () => {
      expect(formatPercent(1.5)).toBe("100.0%")
      expect(formatPercent(-0.2)).toBe("0.0%")
    })
    it("handles NaN", () => {
      expect(formatPercent(Number.NaN)).toBe("—")
    })
    it("renders mid range", () => {
      expect(formatPercent(0.5)).toBe("50.0%")
    })
  })
})

describe("admin-cert-api fetch wrappers", () => {
  const fetchMock = vi.fn()
  beforeEach(() => {
    fetchMock.mockReset()
    global.fetch = fetchMock as unknown as typeof fetch
  })

  it("listOrders returns null on non-ok", async () => {
    fetchMock.mockResolvedValue({ ok: false, status: 500, json: async () => ({}) })
    expect(await listOrders({})).toBeNull()
  })

  it("listOrders returns null on network exception", async () => {
    fetchMock.mockRejectedValue(new Error("offline"))
    expect(await listOrders({})).toBeNull()
  })

  it("listOrders returns data on ok", async () => {
    fetchMock.mockResolvedValue({ ok: true, json: async () => ({ orders: [], limit: 50, offset: 0 }) })
    const out = await listOrders({ status: "issued" })
    expect(out).toEqual({ orders: [], limit: 50, offset: 0 })
    expect(fetchMock).toHaveBeenCalledWith(expect.stringContaining("status=issued"), expect.any(Object))
  })

  it("forceFail surfaces error message body", async () => {
    fetchMock.mockResolvedValue({
      ok: false,
      status: 409,
      json: async () => ({ message: "already terminal" }),
    })
    const res = await forceFail(7, "x")
    expect(res.ok).toBe(false)
    if (!res.ok) {
      expect(res.message).toBe("already terminal")
      expect(res.status).toBe(409)
    }
  })

  it("forceFail falls back to HTTP status when no message body", async () => {
    fetchMock.mockResolvedValue({
      ok: false,
      status: 500,
      json: async () => { throw new Error("nojson") },
    })
    const res = await forceFail(7, "x")
    expect(res.ok).toBe(false)
    if (!res.ok) expect(res.message).toBe("HTTP 500")
  })

  it("forceFail handles network error", async () => {
    fetchMock.mockRejectedValue(new Error("offline"))
    const res = await forceFail(7, "x")
    expect(res.ok).toBe(false)
    if (!res.ok) expect(res.status).toBe(0)
  })

  it("forceFail returns ok=true with data on success", async () => {
    fetchMock.mockResolvedValue({ ok: true, json: async () => ({ order_id: 7, status: "failed" }) })
    const res = await forceFail(7, "x")
    expect(res.ok).toBe(true)
    if (res.ok) expect(res.data.status).toBe("failed")
  })

  it("getCAQuota / getDNSHealth / banAccount call the right paths", async () => {
    fetchMock.mockResolvedValue({ ok: true, json: async () => ({ rows: [], switch_threshold: 0.7 }) })
    await getCAQuota()
    expect(fetchMock).toHaveBeenLastCalledWith("/v1/admin/cert/ca-quota", expect.any(Object))

    fetchMock.mockResolvedValue({ ok: true, json: async () => ({ rows: [] }) })
    await getDNSHealth()
    expect(fetchMock).toHaveBeenLastCalledWith("/v1/admin/cert/dns-health", expect.any(Object))

    fetchMock.mockResolvedValue({ ok: true, json: async () => ({ account_id: 42, status: "banned" }) })
    const res = await banAccount(42, "fraud")
    expect(fetchMock).toHaveBeenLastCalledWith(
      "/v1/admin/cert/accounts/42/ban",
      expect.objectContaining({ method: "POST" }),
    )
    expect(res.ok).toBe(true)
  })
})

// ---------- Component tests --------------------------------------------------

const mockOrder: AdminCertOrder = {
  id: 101,
  account_id: 42,
  sans: ["example.com"],
  sans_unicode: ["example.com"],
  status: "validating",
  tier: "free-dv",
  ca: "lets-encrypt",
  challenge_type: "dns-01",
  retry_count: 0,
  created_at: "2026-05-15T10:00:00Z",
}

describe("OrdersAdminClient", () => {
  it("renders empty state when no orders", () => {
    render(<OrdersAdminClient initialOrders={[]} />)
    expect(screen.getByText("暂无订单")).toBeInTheDocument()
  })

  it("renders order rows and disables force-fail for terminal statuses", () => {
    const terminal: AdminCertOrder = { ...mockOrder, id: 7, status: "issued" }
    render(<OrdersAdminClient initialOrders={[mockOrder, terminal]} />)
    expect(screen.getByText("101")).toBeInTheDocument()
    expect(screen.getByText("7")).toBeInTheDocument()
    // Two force-fail buttons in total — one disabled, one enabled.
    const buttons = screen.getAllByRole("button", { name: "强制失败" })
    expect(buttons).toHaveLength(2)
    const enabled = buttons.filter((b) => !(b as HTMLButtonElement).disabled)
    expect(enabled).toHaveLength(1)
  })

  it("opens AlertDialog on force-fail click and shows confirm/cancel", async () => {
    render(<OrdersAdminClient initialOrders={[mockOrder]} />)
    const btn = screen.getByRole("button", { name: "强制失败" })
    fireEvent.click(btn)
    await waitFor(() => {
      expect(screen.getByText("确认强制失败？")).toBeInTheDocument()
    })
    expect(screen.getByRole("button", { name: "取消" })).toBeInTheDocument()
    expect(screen.getByRole("button", { name: "确认" })).toBeInTheDocument()
  })
})

describe("CAQuotaClient", () => {
  const initial: AdminCAQuotaResponse = {
    switch_threshold: 0.7,
    rows: [
      { ca: "lets-encrypt", per_account_3h: 0.5, per_registered_domain: 0.8, switched: true },
      { ca: "zerossl", per_account_3h: 0.1, per_registered_domain: 0.1, switched: false },
      { ca: "buypass", per_account_3h: 0, per_registered_domain: 0, switched: false, err: "boom" },
    ],
  }

  it("renders rows with status badges", () => {
    render(<CAQuotaClient initial={initial} />)
    expect(screen.getByText("lets-encrypt")).toBeInTheDocument()
    expect(screen.getByText("zerossl")).toBeInTheDocument()
    expect(screen.getByText("buypass")).toBeInTheDocument()
    expect(screen.getByText("已切换")).toBeInTheDocument()
    expect(screen.getByText("未知")).toBeInTheDocument()
  })

  it("renders empty state when no rows", () => {
    render(<CAQuotaClient initial={{ rows: [], switch_threshold: 0.7 }} />)
    expect(screen.getAllByText("暂无数据").length).toBeGreaterThan(0)
  })
})

describe("DNSHealthClient", () => {
  const initial: AdminDNSHealthResponse = {
    rows: [
      { provider: "cloudflare", success_rate: 0.99, samples: 100, window_hours: 24 },
      { provider: "manual", success_rate: -1, samples: 0, window_hours: 24 },
      { provider: "aliyun", success_rate: 0.8, samples: 20, window_hours: 24 },
    ],
  }
  it("renders provider rows with rate badges", () => {
    render(<DNSHealthClient initial={initial} />)
    expect(screen.getByText("cloudflare")).toBeInTheDocument()
    expect(screen.getByText("99.0%")).toBeInTheDocument()
    expect(screen.getByText("80.0%")).toBeInTheDocument()
    // unknown rate is shown as the unknown-label badge text
    expect(screen.getByText("—")).toBeInTheDocument()
  })

  it("renders empty state when no rows", () => {
    render(<DNSHealthClient initial={{ rows: [] }} />)
    expect(screen.getByText("暂无数据")).toBeInTheDocument()
  })
})

describe("AbuseClient", () => {
  beforeEach(() => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ account_id: 42, status: "banned" }),
    }) as unknown as typeof fetch
  })

  it("disables ban button until a valid account ID is entered", () => {
    render(<AbuseClient />)
    const btn = screen.getByRole("button", { name: "封禁账号" })
    expect((btn as HTMLButtonElement).disabled).toBe(true)
    const input = screen.getByLabelText("账号 ID") as HTMLInputElement
    fireEvent.change(input, { target: { value: "42" } })
    expect((btn as HTMLButtonElement).disabled).toBe(false)
  })

  it("does not enable for non-positive ids", () => {
    render(<AbuseClient />)
    const input = screen.getByLabelText("账号 ID") as HTMLInputElement
    fireEvent.change(input, { target: { value: "abc" } })
    const btn = screen.getByRole("button", { name: "封禁账号" })
    expect((btn as HTMLButtonElement).disabled).toBe(true)
    fireEvent.change(input, { target: { value: "0" } })
    expect((btn as HTMLButtonElement).disabled).toBe(true)
  })

  it("opens confirmation dialog when ban button clicked", async () => {
    render(<AbuseClient />)
    fireEvent.change(screen.getByLabelText("账号 ID"), { target: { value: "42" } })
    fireEvent.click(screen.getByRole("button", { name: "封禁账号" }))
    await waitFor(() => {
      expect(screen.getByText("确认操作账号 42？")).toBeInTheDocument()
    })
  })

  it("renders unban button alongside ban", () => {
    render(<AbuseClient />)
    expect(screen.getByRole("button", { name: "封禁账号" })).toBeInTheDocument()
    expect(screen.getByRole("button", { name: "解除封禁" })).toBeInTheDocument()
  })
})
