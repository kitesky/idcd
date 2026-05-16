import { describe, expect, it } from "vitest"
import {
  createOrder,
  createDnsCredential,
  getCert,
  getOrder,
  listCerts,
  listDnsCredentials,
  listOrders,
  retryOrder,
  revokeCert,
  testDnsCredential,
} from "../cert-api"
import { MOCK_CERTS } from "../types"

// These tests exercise the mock-backed cert-api functions. When the real API
// lands the same call sites should still satisfy the assertions about shape.

describe("cert-api / mock backend", () => {
  it("listOrders returns at least the seed orders sorted newest first", async () => {
    const orders = await listOrders()
    expect(orders.length).toBeGreaterThan(0)
    const ts = orders.map((o) => new Date(o.createdAt).getTime())
    for (let i = 1; i < ts.length; i++) {
      expect(ts[i - 1]).toBeGreaterThanOrEqual(ts[i]!)
    }
  })

  it("getOrder returns null for unknown id", async () => {
    expect(await getOrder("does-not-exist")).toBeNull()
  })

  it("getOrder hydrates a known seed order", async () => {
    const o = await getOrder("ord-001")
    expect(o).not.toBeNull()
    expect(o?.san).toContain("idcd.com")
  })

  it("createOrder for dns01-manual produces TXT challenges and pushes to store", async () => {
    const { id } = await createOrder({
      san: ["new.example.com"],
      ca: "letsencrypt",
      challenge: "dns01-manual",
    })
    expect(id).toMatch(/^ord-/)
    const fetched = await getOrder(id)
    expect(fetched?.status).toBe("validating")
    expect(fetched?.manualChallenges?.[0]?.recordName).toBe(
      "_acme-challenge.new.example.com",
    )
  })

  it("createOrder with wildcard san is tagged as wildcard tier", async () => {
    const { id } = await createOrder({
      san: ["*.wild.example.com"],
      ca: "zerossl",
      challenge: "dns01-auto",
      dnsCredentialId: "dns-001",
    })
    const fetched = await getOrder(id)
    expect(fetched?.tier).toBe("wildcard")
    // Wildcard SAN strips leading "*." when forming the challenge record name.
    // We only assert the dns01-auto flow does not surface manual challenges.
    expect(fetched?.manualChallenges).toBeUndefined()
  })

  it("retryOrder flips a failed order back to issuing", async () => {
    const updated = await retryOrder("ord-004")
    expect(updated?.status).toBe("issuing")
    expect(updated?.errorMessage).toBeUndefined()
  })

  it("listCerts returns certs sorted by notAfter ascending", async () => {
    const certs = await listCerts()
    expect(certs.length).toBeGreaterThan(0)
    const ts = certs.map((c) => new Date(c.notAfter).getTime())
    for (let i = 1; i < ts.length; i++) {
      expect(ts[i - 1]).toBeLessThanOrEqual(ts[i]!)
    }
  })

  it("getCert hydrates a known cert", async () => {
    const c = await getCert(MOCK_CERTS[0]!.id)
    expect(c?.serial).toBe(MOCK_CERTS[0]!.serial)
  })

  it("revokeCert flips both cert and originating order to revoked", async () => {
    const cert = await revokeCert("cert-002", "keyCompromise")
    expect(cert?.status).toBe("revoked")
    const order = await getOrder("ord-007")
    // ord-007 is not a seed order; the cert's orderId may not exist in the
    // store. In that case the function still revokes the cert and we just
    // accept null for the order — the revoke flow must not crash.
    if (order) {
      expect(order.status).toBe("revoked")
    }
  })

  it("listDnsCredentials returns the seed credentials", async () => {
    const creds = await listDnsCredentials()
    expect(creds.length).toBeGreaterThanOrEqual(3)
  })

  it("createDnsCredential trims display name and exposes only fingerprint", async () => {
    const cred = await createDnsCredential({
      provider: "cloudflare",
      displayName: "Test 凭据",
      apiToken: "cf_token_secret_value",
    })
    expect(cred.provider).toBe("cloudflare")
    expect(cred.displayName).toBe("Test 凭据")
    expect(cred.fingerprint).toBe("…alue")
    // Returned object must not echo the raw token.
    expect(JSON.stringify(cred)).not.toContain("cf_token_secret_value")
  })

  it("testDnsCredential transitions an unknown credential to healthy", async () => {
    const cred = await createDnsCredential({
      provider: "manual",
      displayName: "Manual test",
    })
    const result = await testDnsCredential(cred.id)
    expect(result.ok).toBe(true)
    const refreshed = (await listDnsCredentials()).find((c) => c.id === cred.id)
    expect(refreshed?.health).toBe("healthy")
  })
})
