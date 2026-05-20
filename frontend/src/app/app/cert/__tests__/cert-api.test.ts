import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"

import {
  CertAPIError,
  createDnsCredential,
  createOrder,
  deleteDnsCredential,
  downloadCert,
  getCert,
  getOrder,
  getQuota,
  listCerts,
  listDnsCredentials,
  listOrders,
  markManualReady,
  retryOrder,
  revokeCert,
  testDnsCredential,
} from "../cert-api"

// ── fetch helpers ───────────────────────────────────────────────────────────

interface FetchCall {
  url: string
  init: RequestInit
}

function makeResponse(
  body: unknown,
  init: { status?: number; contentType?: string } = {},
): Response {
  const status = init.status ?? 200
  const contentType = init.contentType ?? "application/json"
  const isJson = contentType.includes("application/json")
  const text = isJson ? JSON.stringify(body) : String(body ?? "")
  return new Response(text, {
    status,
    headers: { "Content-Type": contentType },
  })
}

function installFetch(
  responder: (url: string, init: RequestInit) => Response | Promise<Response>,
): FetchCall[] {
  const calls: FetchCall[] = []
  const fn = vi.fn(async (input: RequestInfo | URL, init: RequestInit = {}) => {
    const url = typeof input === "string" ? input : input.toString()
    calls.push({ url, init })
    return responder(url, init)
  })
  vi.stubGlobal("fetch", fn)
  return calls
}

beforeEach(() => {
  vi.unstubAllGlobals()
})

afterEach(() => {
  vi.unstubAllGlobals()
})

// ── tests ───────────────────────────────────────────────────────────────────

describe("cert-api / fetch wiring", () => {
  it("listOrders maps snake_case payload and sorts newest first", async () => {
    const calls = installFetch(() =>
      makeResponse({
        orders: [
          {
            id: "ord-a",
            sans: ["a.example.com"],
            ca: "letsencrypt",
            challenge: "dns01-auto",
            status: "issued",
            idempotency_key: "idem-a",
            created_at: "2025-05-10T00:00:00Z",
            updated_at: "2025-05-10T00:00:00Z",
            events: [],
          },
          {
            id: "ord-b",
            sans: ["b.example.com"],
            ca: "letsencrypt",
            challenge: "dns01-auto",
            status: "issuing",
            idempotency_key: "idem-b",
            created_at: "2025-05-15T00:00:00Z",
            updated_at: "2025-05-15T00:00:00Z",
            events: [],
          },
        ],
      }),
    )

    const orders = await listOrders()
    expect(calls[0]!.url).toMatch(/\/v1\/cert\/orders$/)
    expect(calls[0]!.init.credentials).toBe("include")
    expect(orders).toHaveLength(2)
    expect(orders[0]!.id).toBe("ord-b")
    expect(orders[0]!.san).toEqual(["b.example.com"])
  })

  it("listOrders unwraps a bare array payload", async () => {
    installFetch(() => makeResponse([]))
    const orders = await listOrders()
    expect(orders).toEqual([])
  })

  it("getOrder returns null on 404", async () => {
    installFetch(() =>
      makeResponse({ code: "CERT_NOT_FOUND", message: "not found" }, { status: 404 }),
    )
    expect(await getOrder("missing")).toBeNull()
  })

  it("getOrder throws CertAPIError with code on 5xx", async () => {
    installFetch(() =>
      makeResponse({ code: "CERT_INTERNAL", message: "boom" }, { status: 500 }),
    )
    await expect(getOrder("x")).rejects.toMatchObject({
      code: "CERT_INTERNAL",
      httpStatus: 500,
    })
  })

  it("getOrder maps manual_challenge (singular) into manualChallenges", async () => {
    installFetch(() =>
      makeResponse({
        id: "ord-1",
        san: ["api.example.com"],
        ca: "letsencrypt",
        challenge_type: "dns-01",
        dns_credential_id: null,
        status: "validating",
        idempotency_key: "k",
        created_at: "2025-05-15T00:00:00Z",
        updated_at: "2025-05-15T00:00:00Z",
        manual_challenge: {
          fqdn: "_acme-challenge.api.example.com",
          value: "abc123",
        },
        events: [],
      }),
    )

    const o = await getOrder("ord-1")
    expect(o?.challenge).toBe("dns01-manual")
    expect(o?.manualChallenges?.[0]).toEqual({
      recordName: "_acme-challenge.api.example.com",
      recordValue: "abc123",
    })
  })

  it("createOrder POSTs sans + idempotency_key and returns id", async () => {
    const calls = installFetch(() =>
      makeResponse({ id: "ord-new", status: "issuing" }, { status: 202 }),
    )

    const out = await createOrder({
      san: ["new.example.com"],
      ca: "letsencrypt",
      challenge: "dns01-auto",
      dnsCredentialId: "dns-1",
    })
    expect(out).toEqual({ id: "ord-new" })

    expect(calls[0]!.url).toMatch(/\/v1\/cert\/orders$/)
    expect(calls[0]!.init.method).toBe("POST")
    const body = JSON.parse(calls[0]!.init.body as string)
    expect(body.sans).toEqual(["new.example.com"])
    expect(body.ca).toBe("letsencrypt")
    expect(body.challenge).toBe("dns01-auto")
    expect(body.dns_credential_id).toBe("dns-1")
    expect(typeof body.idempotency_key).toBe("string")
    expect(body.idempotency_key.length).toBeGreaterThan(0)
  })

  it("createOrder surfaces a 422 field error as CertAPIError with fields", async () => {
    installFetch(() =>
      makeResponse(
        {
          code: "CERT_DOMAIN_INVALID",
          message: "invalid SAN",
          fields: { sans: "must not be empty" },
        },
        { status: 422 },
      ),
    )

    try {
      await createOrder({ san: [], ca: "letsencrypt", challenge: "dns01-auto" })
      throw new Error("should have thrown")
    } catch (err) {
      expect(err).toBeInstanceOf(CertAPIError)
      const ce = err as CertAPIError
      expect(ce.code).toBe("CERT_DOMAIN_INVALID")
      expect(ce.httpStatus).toBe(422)
      expect(ce.fields).toEqual({ sans: "must not be empty" })
    }
  })

  it("retryOrder POSTs to /retry and returns mapped order", async () => {
    const calls = installFetch(() =>
      makeResponse({
        id: "ord-1",
        san: ["a.example.com"],
        ca: "letsencrypt",
        challenge: "dns01-auto",
        status: "issuing",
        idempotency_key: "k",
        created_at: "2025-05-15T00:00:00Z",
        updated_at: "2025-05-15T01:00:00Z",
        events: [],
      }),
    )
    const o = await retryOrder("ord-1")
    expect(calls[0]!.url).toMatch(/\/v1\/cert\/orders\/ord-1\/retry$/)
    expect(calls[0]!.init.method).toBe("POST")
    expect(o?.status).toBe("issuing")
  })

  it("markManualReady POSTs fqdn + value", async () => {
    const calls = installFetch(() => new Response(null, { status: 204 }))
    // No body → re-fetches order
    installFetch((url) => {
      if (url.endsWith("/manual-ready")) return new Response(null, { status: 204 })
      return makeResponse({
        id: "ord-1",
        san: ["api.example.com"],
        ca: "letsencrypt",
        challenge: "dns01-manual",
        status: "validating",
        idempotency_key: "k",
        created_at: "2025-05-15T00:00:00Z",
        updated_at: "2025-05-15T00:00:00Z",
        events: [],
      })
    })

    void calls
    const out = await markManualReady(
      "ord-1",
      "_acme-challenge.api.example.com",
      "abc123",
    )
    expect(out?.id).toBe("ord-1")
  })

  it("markManualReady sends correct body", async () => {
    const calls = installFetch(() =>
      makeResponse({
        id: "ord-1",
        san: ["api.example.com"],
        ca: "letsencrypt",
        challenge: "dns01-manual",
        status: "issuing",
        idempotency_key: "k",
        created_at: "2025-05-15T00:00:00Z",
        updated_at: "2025-05-15T00:00:00Z",
        events: [],
      }),
    )

    await markManualReady("ord-1", "_acme-challenge.api.example.com", "v")
    expect(calls[0]!.url).toMatch(/\/v1\/cert\/orders\/ord-1\/manual-ready$/)
    const body = JSON.parse(calls[0]!.init.body as string)
    expect(body).toEqual({
      fqdn: "_acme-challenge.api.example.com",
      value: "v",
    })
  })

  it("listCerts maps payload and sorts by notAfter ascending", async () => {
    installFetch(() =>
      makeResponse({
        certs: [
          {
            id: "c1",
            order_id: "o1",
            sans: ["a.com"],
            issuer: "LE",
            serial: "01",
            fingerprint_sha256: "aa",
            not_before: "2025-01-01T00:00:00Z",
            not_after: "2025-12-01T00:00:00Z",
            status: "active",
          },
          {
            id: "c2",
            order_id: "o2",
            sans: ["b.com"],
            issuer: "LE",
            serial: "02",
            fingerprint_sha256: "bb",
            not_before: "2025-02-01T00:00:00Z",
            not_after: "2025-06-01T00:00:00Z",
            status: "active",
          },
        ],
      }),
    )

    const certs = await listCerts()
    expect(certs[0]!.id).toBe("c2")
    expect(certs[0]!.notAfter).toBe("2025-06-01T00:00:00Z")
  })

  it("getCert returns null on 404", async () => {
    installFetch(() => makeResponse({}, { status: 404 }))
    expect(await getCert("missing")).toBeNull()
  })

  it("downloadCert translates pkcs12 → pfx and returns url+filename", async () => {
    const calls = installFetch(() =>
      makeResponse({
        download_url: "https://example.com/dl/1",
        expires_at: "2025-05-15T00:00:00Z",
      }),
    )

    const out = await downloadCert("cert-1", "pkcs12")
    expect(out.url).toBe("https://example.com/dl/1")
    expect(out.filename).toBe("cert-1.p12")

    expect(calls[0]!.url).toMatch(/\/v1\/cert\/certs\/cert-1\/download$/)
    expect(calls[0]!.init.method).toBe("POST")
    expect(JSON.parse(calls[0]!.init.body as string)).toEqual({ format: "pfx" })
  })

  it("downloadCert translates nginx-fullchain → nginx", async () => {
    const calls = installFetch(() =>
      makeResponse({ download_url: "https://x", filename: "fullchain.pem" }),
    )
    const out = await downloadCert("cert-2", "nginx-fullchain")
    expect(out.filename).toBe("fullchain.pem")
    expect(JSON.parse(calls[0]!.init.body as string)).toEqual({ format: "nginx" })
  })

  it("revokeCert POSTs reason and returns mapped cert", async () => {
    const calls = installFetch(() =>
      makeResponse({
        id: "cert-1",
        order_id: "ord-1",
        sans: ["a.com"],
        issuer: "LE",
        serial: "01",
        fingerprint_sha256: "aa",
        not_before: "2025-01-01T00:00:00Z",
        not_after: "2025-04-01T00:00:00Z",
        status: "revoked",
      }),
    )
    const c = await revokeCert("cert-1", "keyCompromise")
    expect(c?.status).toBe("revoked")
    expect(calls[0]!.url).toMatch(/\/v1\/cert\/certs\/cert-1\/revoke$/)
    expect(JSON.parse(calls[0]!.init.body as string)).toEqual({
      reason: "keyCompromise",
    })
  })

  it("listDnsCredentials maps payload", async () => {
    installFetch(() =>
      makeResponse({
        dns_credentials: [
          {
            id: "dns-1",
            provider: "cloudflare",
            display_name: "Main",
            health: "healthy",
            created_at: "2025-05-01T00:00:00Z",
            fingerprint: "…abcd",
          },
        ],
      }),
    )
    const creds = await listDnsCredentials()
    expect(creds).toHaveLength(1)
    expect(creds[0]!.displayName).toBe("Main")
    expect(creds[0]!.health).toBe("healthy")
  })

  it("createDnsCredential POSTs provider + display_name + secrets and never echoes the raw token", async () => {
    const calls = installFetch(() =>
      makeResponse({
        id: "dns-new",
        provider: "cloudflare",
        display_name: "Test",
        health: "unknown",
        created_at: "2025-05-15T00:00:00Z",
        fingerprint: "…2345",
      }),
    )

    const cred = await createDnsCredential({
      provider: "cloudflare",
      displayName: "Test",
      apiToken: "supersecrettoken12345",
    })

    expect(cred.fingerprint).toBe("…2345")
    expect(JSON.stringify(cred)).not.toContain("supersecrettoken12345")

    const body = JSON.parse(calls[0]!.init.body as string)
    expect(body.provider).toBe("cloudflare")
    expect(body.display_name).toBe("Test")
    expect(body.secrets).toEqual({ api_token: "supersecrettoken12345" })
  })

  it("testDnsCredential returns ok:true when backend reports healthy", async () => {
    installFetch(() =>
      makeResponse({ ok: true, health: "healthy", message: "ok" }),
    )
    const res = await testDnsCredential("dns-1")
    expect(res).toEqual({ ok: true, message: "ok" })
  })

  it("testDnsCredential returns ok:false on backend error rather than throwing", async () => {
    installFetch(() =>
      makeResponse(
        { code: "CERT_DNS_PROVIDER_FAIL", message: "bad token" },
        { status: 502 },
      ),
    )
    const res = await testDnsCredential("dns-1")
    expect(res.ok).toBe(false)
    expect(res.message).toContain("bad token")
  })

  it("deleteDnsCredential DELETEs the right URL", async () => {
    const calls = installFetch(() => new Response(null, { status: 204 }))
    await deleteDnsCredential("dns-1")
    expect(calls[0]!.url).toMatch(/\/v1\/cert\/dns-credentials\/dns-1$/)
    expect(calls[0]!.init.method).toBe("DELETE")
  })

  it("getQuota maps snake_case fields", async () => {
    installFetch(() =>
      makeResponse({
        used: 3,
        limit: 5,
        expiring_soon: 1,
        monthly_success_rate: 96,
      }),
    )
    const q = await getQuota()
    expect(q).toEqual({
      used: 3,
      limit: 5,
      expiringSoon: 1,
      monthlySuccessRate: 96,
    })
  })

  it("propagates 501 as CERT_NOT_IMPL", async () => {
    installFetch(() => new Response(null, { status: 501 }))
    await expect(getQuota()).rejects.toMatchObject({
      code: "CERT_NOT_IMPL",
      httpStatus: 501,
    })
  })

  it("wraps network failures as CERT_NETWORK_ERROR", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => {
        throw new TypeError("Failed to fetch")
      }),
    )
    await expect(listOrders()).rejects.toMatchObject({
      code: "CERT_NETWORK_ERROR",
      httpStatus: 0,
    })
  })
})
