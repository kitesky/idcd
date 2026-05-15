import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"
import { NextRequest } from "next/server"

const mockSaveReport = vi.fn()
vi.mock("@/lib/diagnose-store", () => ({
  saveReport: mockSaveReport,
}))

const buildFetchResponse = (body: unknown, ok = true, status = 200) =>
  Promise.resolve({
    ok,
    status,
    statusText: ok ? "OK" : "Error",
    json: () => Promise.resolve(body),
    text: () => Promise.resolve(JSON.stringify(body)),
  } as Response)

const makeMockFetch = (overrides: Record<string, unknown> = {}) =>
  vi.fn((url: string, init?: RequestInit) => {
    const urlStr = typeof url === "string" ? url : String(url)

    if (urlStr.includes("/v1/info/dns"))
      return buildFetchResponse(
        overrides.dns ?? {
          domain: "example.com",
          type: "A",
          records: [{ value: "93.184.216.34", ttl: 3600 }],
        }
      )
    if (urlStr.includes("/v1/probe/http"))
      return buildFetchResponse(
        overrides.http ?? { task_id: "task-http-123", status: "queued" }
      )
    if (urlStr.includes("/v1/probe/ping"))
      return buildFetchResponse(
        overrides.ping ?? { task_id: "task-ping-456", status: "queued" }
      )
    if (urlStr.includes("/v1/probe/traceroute"))
      return buildFetchResponse(
        overrides.traceroute ?? { task_id: "task-tr-789", status: "queued" }
      )
    if (urlStr.includes("/v1/info/ssl"))
      return buildFetchResponse(
        overrides.ssl ?? {
          domain: "example.com",
          issuer: "Let's Encrypt",
          days_until_expiry: 90,
        }
      )
    if (urlStr.includes("/v1/info/icp"))
      return buildFetchResponse(
        overrides.icp ?? {
          domain: "example.com",
          icp_number: "",
          note: "ICP query will be implemented in S2",
        }
      )
    if (urlStr.includes("/v1/info/whois"))
      return buildFetchResponse(
        overrides.whois ?? {
          domain: "example.com",
          registrar: "GoDaddy LLC",
          expiry_date: "2027-03-12",
        }
      )
    return buildFetchResponse({}, false, 404)
  })

const collectSSEEvents = async (response: Response) => {
  const text = await response.text()
  return text
    .split("\n\n")
    .filter((chunk) => chunk.startsWith("data: "))
    .map((chunk) => JSON.parse(chunk.replace(/^data: /, "")) as Record<string, unknown>)
}

describe("GET /api/diagnose/stream", () => {
  let originalFetch: typeof global.fetch

  beforeEach(() => {
    originalFetch = global.fetch
    mockSaveReport.mockClear()
  })

  afterEach(() => {
    global.fetch = originalFetch
    vi.resetModules()
  })

  it("returns 400 when domain param is missing", async () => {
    const { GET } = await import("../route")
    const req = new NextRequest("http://localhost/api/diagnose/stream")
    const res = await GET(req)
    expect(res.status).toBe(400)
  })

  it("streams check_start for all 7 checks", async () => {
    global.fetch = makeMockFetch() as typeof global.fetch
    const { GET } = await import("../route")
    const req = new NextRequest(
      "http://localhost/api/diagnose/stream?domain=example.com"
    )
    const res = await GET(req)
    const events = await collectSSEEvents(res)
    const startEvents = events.filter((e) => e.type === "check_start")
    expect(startEvents).toHaveLength(7)
    const keys = startEvents.map((e) => e.key)
    expect(keys).toContain("dns")
    expect(keys).toContain("http")
    expect(keys).toContain("ping")
    expect(keys).toContain("traceroute")
    expect(keys).toContain("ssl")
    expect(keys).toContain("icp")
    expect(keys).toContain("whois")
  })

  it("streams check_done for each check with summary and detail", async () => {
    global.fetch = makeMockFetch() as typeof global.fetch
    const { GET } = await import("../route")
    const req = new NextRequest(
      "http://localhost/api/diagnose/stream?domain=example.com"
    )
    const res = await GET(req)
    const events = await collectSSEEvents(res)
    const doneEvents = events.filter((e) => e.type === "check_done")
    expect(doneEvents).toHaveLength(7)
    for (const ev of doneEvents) {
      expect(typeof ev.key).toBe("string")
      expect(typeof ev.summary).toBe("string")
      expect(ev.detail).toBeDefined()
    }
  })

  it("ends with a complete event containing a reportId", async () => {
    global.fetch = makeMockFetch() as typeof global.fetch
    const { GET } = await import("../route")
    const req = new NextRequest(
      "http://localhost/api/diagnose/stream?domain=example.com"
    )
    const res = await GET(req)
    const events = await collectSSEEvents(res)
    const completeEvent = events.find((e) => e.type === "complete")
    expect(completeEvent).toBeDefined()
    expect(typeof completeEvent!.reportId).toBe("string")
    expect((completeEvent!.reportId as string).length).toBeGreaterThan(0)
  })

  it("sets SSE content-type headers", async () => {
    global.fetch = makeMockFetch() as typeof global.fetch
    const { GET } = await import("../route")
    const req = new NextRequest(
      "http://localhost/api/diagnose/stream?domain=example.com"
    )
    const res = await GET(req)
    expect(res.headers.get("Content-Type")).toBe("text/event-stream")
    expect(res.headers.get("Cache-Control")).toContain("no-cache")
  })

  it("saves report with correct domain and check counts", async () => {
    global.fetch = makeMockFetch() as typeof global.fetch
    const { GET } = await import("../route")
    const req = new NextRequest(
      "http://localhost/api/diagnose/stream?domain=example.com"
    )
    const res = await GET(req)
    await res.text()

    expect(mockSaveReport).toHaveBeenCalledOnce()
    const saved = mockSaveReport.mock.calls[0]![0] as {
      domain: string
      checks: unknown[]
      doneCount: number
      errorCount: number
    }
    expect(saved.domain).toBe("example.com")
    expect(saved.checks).toHaveLength(7)
    expect(saved.errorCount).toBe(0)
    expect(saved.doneCount).toBe(7)
  })

  it("marks failed checks as error and increments errorCount", async () => {
    const fetchMock = makeMockFetch()
    const originalImpl = fetchMock.getMockImplementation()!
    fetchMock.mockImplementation((url: string, init?: RequestInit) => {
      if (typeof url === "string" && url.includes("/v1/info/ssl")) {
        return buildFetchResponse({}, false, 503)
      }
      return originalImpl(url, init)
    })
    global.fetch = fetchMock as typeof global.fetch
    const { GET } = await import("../route")
    const req = new NextRequest(
      "http://localhost/api/diagnose/stream?domain=example.com"
    )
    const res = await GET(req)
    await res.text()

    expect(mockSaveReport).toHaveBeenCalledOnce()
    const saved = mockSaveReport.mock.calls[0]![0] as {
      errorCount: number
      doneCount: number
      checks: { key: string; status: string }[]
    }
    expect(saved.errorCount).toBe(1)
    expect(saved.doneCount).toBe(6)
    const sslCheck = saved.checks.find((c) => c.key === "ssl")
    expect(sslCheck?.status).toBe("error")
  })

  it("dns summarize extracts record count and IPs", async () => {
    global.fetch = makeMockFetch({
      dns: {
        domain: "example.com",
        type: "A",
        records: [
          { value: "1.2.3.4", ttl: 300 },
          { value: "5.6.7.8", ttl: 300 },
        ],
      },
    }) as typeof global.fetch
    const { GET } = await import("../route")
    const req = new NextRequest(
      "http://localhost/api/diagnose/stream?domain=example.com"
    )
    const res = await GET(req)
    const events = await collectSSEEvents(res)
    const dnsDone = events.find((e) => e.type === "check_done" && e.key === "dns")
    expect(dnsDone?.summary).toContain("2")
    expect(dnsDone?.summary).toContain("1.2.3.4")
  })

  it("ssl summarize includes issuer and days remaining", async () => {
    global.fetch = makeMockFetch({
      ssl: {
        domain: "example.com",
        issuer: "Let's Encrypt",
        days_until_expiry: 127,
      },
    }) as typeof global.fetch
    const { GET } = await import("../route")
    const req = new NextRequest(
      "http://localhost/api/diagnose/stream?domain=example.com"
    )
    const res = await GET(req)
    const events = await collectSSEEvents(res)
    const sslDone = events.find((e) => e.type === "check_done" && e.key === "ssl")
    expect(sslDone?.summary).toContain("127")
    expect(sslDone?.summary).toContain("Let's Encrypt")
  })

  it("icp summarize shows icp_number when present", async () => {
    global.fetch = makeMockFetch({
      icp: {
        domain: "example.com",
        icp_number: "沪ICP备12345678号",
        company: "示例公司",
      },
    }) as typeof global.fetch
    const { GET } = await import("../route")
    const req = new NextRequest(
      "http://localhost/api/diagnose/stream?domain=example.com"
    )
    const res = await GET(req)
    const events = await collectSSEEvents(res)
    const icpDone = events.find((e) => e.type === "check_done" && e.key === "icp")
    expect(icpDone?.summary).toContain("沪ICP备12345678号")
    expect(icpDone?.summary).toContain("示例公司")
  })

  it("whois summarize shows registrar and expiry", async () => {
    global.fetch = makeMockFetch({
      whois: {
        domain: "example.com",
        registrar: "GoDaddy LLC",
        expiry_date: "2027-03-12",
      },
    }) as typeof global.fetch
    const { GET } = await import("../route")
    const req = new NextRequest(
      "http://localhost/api/diagnose/stream?domain=example.com"
    )
    const res = await GET(req)
    const events = await collectSSEEvents(res)
    const whoisDone = events.find((e) => e.type === "check_done" && e.key === "whois")
    expect(whoisDone?.summary).toContain("GoDaddy LLC")
    expect(whoisDone?.summary).toContain("2027-03-12")
  })
})
