// Fetch abstraction layer for the cert module.
//
// IMPLEMENTATION NOTE (S1 W2): every function below operates on the in-module
// MOCK_* arrays from ./types so the UI can be exercised end-to-end while the
// backend handlers (`cert-svc`, currently 501) are still under construction.
//
// When the real endpoints land in W3 we keep the exported signatures unchanged
// and replace each body with the matching `fetch('/api/v1/cert/...')` call:
//
//   listOrders        →  GET    /api/v1/cert/orders
//   getOrder          →  GET    /api/v1/cert/orders/{id}
//   createOrder       →  POST   /api/v1/cert/orders
//   retryOrder        →  POST   /api/v1/cert/orders/{id}/retry
//   confirmManual     →  POST   /api/v1/cert/orders/{id}/confirm
//   listCerts         →  GET    /api/v1/cert/certs
//   getCert           →  GET    /api/v1/cert/certs/{id}
//   downloadCert      →  GET    /api/v1/cert/certs/{id}/download?format=…
//   revokeCert        →  POST   /api/v1/cert/certs/{id}/revoke
//   listDnsCreds      →  GET    /api/v1/cert/dns-credentials
//   createDnsCred     →  POST   /api/v1/cert/dns-credentials
//   testDnsCred       →  POST   /api/v1/cert/dns-credentials/{id}/test
//   deleteDnsCred     →  DELETE /api/v1/cert/dns-credentials/{id}
//   getQuota          →  GET    /api/v1/cert/quota

import {
  MOCK_CERTS,
  MOCK_DNS_CREDENTIALS,
  MOCK_ORDERS,
  MOCK_QUOTA,
  type Cert,
  type CertQuota,
  type CreateDnsCredentialRequest,
  type CreateOrderRequest,
  type DnsCredential,
  type DownloadFormat,
  type Order,
  type RevokeReason,
} from "./types"

// In-memory store so mutations inside the wizard persist for the lifetime of
// the JS bundle (matches the request/response surface the real API will give
// us once it ships).
const orderStore: Order[] = [...MOCK_ORDERS]
const certStore: Cert[] = [...MOCK_CERTS]
const credStore: DnsCredential[] = [...MOCK_DNS_CREDENTIALS]

function delay<T>(value: T, ms = 200): Promise<T> {
  return new Promise((resolve) => setTimeout(() => resolve(value), ms))
}

function newId(prefix: string): string {
  const rand = Math.random().toString(36).slice(2, 8)
  const ts = Date.now().toString(36).slice(-4)
  return `${prefix}-${ts}${rand}`
}

// ── Orders ──────────────────────────────────────────────────────────────────

export async function listOrders(): Promise<Order[]> {
  return delay([...orderStore].sort((a, b) => (a.createdAt < b.createdAt ? 1 : -1)))
}

export async function getOrder(id: string): Promise<Order | null> {
  const o = orderStore.find((x) => x.id === id) ?? null
  return delay(o)
}

export async function createOrder(
  req: CreateOrderRequest,
): Promise<{ id: string }> {
  const id = newId("ord")
  const nowIso = new Date().toISOString()
  const wildcard = req.san.some((s) => s.startsWith("*."))
  const order: Order = {
    id,
    san: req.san,
    tier: wildcard ? "wildcard" : "free",
    ca: req.ca,
    challenge: req.challenge,
    status: req.challenge === "dns01-manual" ? "validating" : "issuing",
    idempotencyKey: `idem-${id}`,
    createdAt: nowIso,
    updatedAt: nowIso,
    dnsCredentialId: req.dnsCredentialId,
    manualChallenges:
      req.challenge === "dns01-manual"
        ? req.san.map((host) => ({
            recordName: `_acme-challenge.${host.replace(/^\*\./, "")}`,
            recordValue: Math.random().toString(36).slice(2, 14) + Math.random().toString(36).slice(2, 14),
          }))
        : undefined,
    events: [{ at: nowIso, action: "order.created" }],
  }
  orderStore.unshift(order)
  return delay({ id })
}

export async function retryOrder(id: string): Promise<Order | null> {
  const o = orderStore.find((x) => x.id === id)
  if (!o) return delay(null)
  o.status = "issuing"
  o.updatedAt = new Date().toISOString()
  o.errorMessage = undefined
  o.events.push({ at: o.updatedAt, action: "order.retry" })
  return delay({ ...o })
}

export async function confirmManualChallenge(
  id: string,
): Promise<Order | null> {
  const o = orderStore.find((x) => x.id === id)
  if (!o) return delay(null)
  o.status = "issuing"
  o.updatedAt = new Date().toISOString()
  o.events.push({ at: o.updatedAt, action: "challenge.dns.confirmed" })
  return delay({ ...o })
}

// ── Certs ───────────────────────────────────────────────────────────────────

export async function listCerts(): Promise<Cert[]> {
  return delay(
    [...certStore].sort((a, b) => (a.notAfter < b.notAfter ? -1 : 1)),
  )
}

export async function getCert(id: string): Promise<Cert | null> {
  return delay(certStore.find((c) => c.id === id) ?? null)
}

export async function downloadCert(
  id: string,
  format: DownloadFormat,
): Promise<{ url: string; filename: string }> {
  // Mock: real API will issue a one-shot signed URL; we return a data URL with
  // a stub payload so the download button can demo end-to-end.
  const cert = certStore.find((c) => c.id === id)
  const subject = cert?.san[0] ?? id
  const ext =
    format === "pkcs12" ? "p12" : format === "nginx-fullchain" ? "pem" : "pem"
  const filename = `${subject}.${ext}`
  const stubPayload = `# mock-${format}\n# cert ${id} for ${subject}\n`
  const url = `data:application/octet-stream;base64,${typeof btoa === "function" ? btoa(stubPayload) : ""}`
  return delay({ url, filename })
}

export async function revokeCert(
  id: string,
  reason: RevokeReason,
): Promise<Cert | null> {
  const cert = certStore.find((c) => c.id === id)
  if (!cert) return delay(null)
  cert.status = "revoked"
  // Mirror the revocation onto the originating order's event stream so the
  // detail page reflects reality.
  const order = orderStore.find((o) => o.id === cert.orderId)
  if (order) {
    order.status = "revoked"
    order.updatedAt = new Date().toISOString()
    order.events.push({
      at: order.updatedAt,
      action: "cert.revoked",
      payload: { reason },
    })
  }
  return delay({ ...cert })
}

// ── DNS credentials ─────────────────────────────────────────────────────────

export async function listDnsCredentials(): Promise<DnsCredential[]> {
  return delay([...credStore])
}

export async function createDnsCredential(
  req: CreateDnsCredentialRequest,
): Promise<DnsCredential> {
  const cred: DnsCredential = {
    id: newId("dns"),
    provider: req.provider,
    displayName: req.displayName,
    health: "unknown",
    createdAt: new Date().toISOString(),
    fingerprint:
      req.apiToken && req.apiToken.length > 4
        ? `…${req.apiToken.slice(-4)}`
        : undefined,
  }
  credStore.unshift(cred)
  return delay(cred)
}

export async function testDnsCredential(
  id: string,
): Promise<{ ok: boolean; message: string }> {
  const cred = credStore.find((c) => c.id === id)
  if (!cred) return delay({ ok: false, message: "未找到凭据" })
  cred.health = "healthy"
  return delay({ ok: true, message: "连接成功" })
}

export async function deleteDnsCredential(id: string): Promise<void> {
  const idx = credStore.findIndex((c) => c.id === id)
  if (idx >= 0) credStore.splice(idx, 1)
  return delay(undefined)
}

// ── Quota ───────────────────────────────────────────────────────────────────

export async function getQuota(): Promise<CertQuota> {
  return delay({ ...MOCK_QUOTA, used: Math.min(MOCK_QUOTA.limit, orderStore.length) })
}
