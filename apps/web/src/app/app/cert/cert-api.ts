// Fetch layer for the cert module.
//
// S1 W3: wired to the real `/api/v1/cert/*` endpoints served by cert-svc.
// During the mock phase this file kept an in-memory store; that has been
// removed. Endpoints not yet implemented on the backend return HTTP 501,
// which we surface as `CertAPIError(code: "CERT_NOT_IMPL")` so callers can
// degrade gracefully.
//
// Naming: backend payloads are snake_case. To keep the existing UI components
// untouched (they consume camelCase `types.ts`) we apply a thin mapping layer
// here — mirrors the pattern used by apps/web/src/app/app/monitors.

import type {
  Cert,
  CertQuota,
  CreateDnsCredentialRequest,
  CreateOrderRequest,
  DnsCredential,
  DnsCredentialHealth,
  DnsProvider,
  DownloadFormat,
  Order,
  OrderEvent,
  CaProvider,
  CertStatus,
  ChallengeMode,
  CertTier,
  ManualChallenge,
  OrderStatus,
  RevokeReason,
} from "./types"

const API_BASE =
  (typeof process !== "undefined" &&
    (process.env.NEXT_PUBLIC_API_URL as string | undefined)) ||
  ""
const API_PREFIX = "/api/v1/cert"

const DEFAULT_TIMEOUT_MS = 30_000
const MUTATING = new Set(["POST", "PUT", "PATCH", "DELETE"])

function getCsrfToken(): string {
  if (typeof document === "undefined") return ""
  const m = document.cookie.match(/(?:^|;\s*)csrf_token=([^;]+)/)
  return m ? decodeURIComponent(m[1]!) : ""
}

function getCurrentLocale(): string {
  if (typeof document === "undefined") return "en"
  const cookies = document.cookie
  const fresh = cookies.match(/(?:^|;\s*)idcd_locale=([^;]+)/)
  if (fresh) return decodeURIComponent(fresh[1]!)
  const legacy = cookies.match(/(?:^|;\s*)locale=([^;]+)/)
  if (legacy) return decodeURIComponent(legacy[1]!)
  return "en"
}

// ── Error type ──────────────────────────────────────────────────────────────

export class CertAPIError extends Error {
  readonly code: string
  readonly httpStatus: number
  readonly fields?: Record<string, string>

  constructor(
    code: string,
    httpStatus: number,
    message: string,
    fields?: Record<string, string>,
  ) {
    super(message)
    this.name = "CertAPIError"
    this.code = code
    this.httpStatus = httpStatus
    this.fields = fields
  }
}

interface ApiError {
  code?: string
  message?: string
  fields?: Record<string, string>
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const url = `${API_BASE}${API_PREFIX}${path}`
  const method = (init.method ?? "GET").toUpperCase()
  const csrf = MUTATING.has(method) ? { "X-CSRF-Token": getCsrfToken() } : {}
  const ownController = init.signal ? null : new AbortController()
  const timeoutId = ownController
    ? setTimeout(() => ownController.abort(), DEFAULT_TIMEOUT_MS)
    : null
  let res: Response
  try {
    res = await fetch(url, {
      credentials: "include",
      ...init,
      headers: {
        Accept: "application/json",
        "Content-Type": "application/json",
        "X-Locale": getCurrentLocale(),
        ...csrf,
        ...(init.headers || {}),
      },
      signal: init.signal ?? ownController!.signal,
    })
  } catch (err) {
    if (timeoutId !== null) clearTimeout(timeoutId)
    throw new CertAPIError(
      "CERT_NETWORK_ERROR",
      0,
      err instanceof Error ? err.message : "网络错误",
    )
  }
  if (timeoutId !== null) clearTimeout(timeoutId)

  if (res.status === 204) {
    return undefined as T
  }

  // Try to parse JSON for both success and error paths.
  let body: unknown = null
  const ctype = res.headers.get("content-type") || ""
  if (ctype.includes("application/json")) {
    try {
      body = await res.json()
    } catch {
      body = null
    }
  }

  if (!res.ok) {
    const err = (body as ApiError) || {}
    // Some backends wrap errors under {error: {...}}.
    const wrapped = (body as { error?: ApiError } | null)?.error
    const e = wrapped || err
    const code =
      e.code ||
      (res.status === 501 ? "CERT_NOT_IMPL" : `CERT_HTTP_${res.status}`)
    throw new CertAPIError(
      code,
      res.status,
      e.message || res.statusText || "请求失败",
      e.fields,
    )
  }

  // Some endpoints wrap success bodies under {data: ...} — unwrap when present.
  if (body && typeof body === "object" && "data" in (body as object)) {
    return (body as { data: T }).data
  }
  return body as T
}

// ── Raw API shapes (snake_case) ─────────────────────────────────────────────

interface RawOrderEvent {
  at: string
  action: string
  payload?: Record<string, unknown>
}

interface RawManualChallenge {
  record_name?: string
  fqdn?: string
  record_value?: string
  value?: string
}

interface RawOrder {
  id?: string
  order_id?: string
  san?: string[]
  sans?: string[]
  tier?: CertTier
  ca?: CaProvider
  challenge?: ChallengeMode
  challenge_type?: string
  status?: OrderStatus
  idempotency_key?: string
  created_at?: string
  updated_at?: string
  dns_credential_id?: string | null
  manual_challenges?: RawManualChallenge[] | null
  manual_challenge?: RawManualChallenge | null
  events?: RawOrderEvent[] | null
  cert_id?: string | null
  error_message?: string | null
}

interface RawCert {
  id?: string
  cert_id?: string
  order_id?: string
  san?: string[]
  sans?: string[]
  issuer?: string
  serial?: string
  fingerprint_sha256?: string
  not_before?: string
  not_after?: string
  status?: CertStatus
}

interface RawDnsCredential {
  id?: string
  credential_id?: string
  provider?: DnsProvider
  display_name?: string
  health?: DnsCredentialHealth
  created_at?: string
  fingerprint?: string | null
}

interface RawQuota {
  used?: number
  limit?: number
  expiring_soon?: number
  monthly_success_rate?: number
}

// ── Mappers ─────────────────────────────────────────────────────────────────

function mapManualChallenge(raw: RawManualChallenge): ManualChallenge {
  return {
    recordName: raw.record_name ?? raw.fqdn ?? "",
    recordValue: raw.record_value ?? raw.value ?? "",
  }
}

function mapChallenge(raw: RawOrder): ChallengeMode {
  if (raw.challenge === "dns01-auto" || raw.challenge === "dns01-manual") {
    return raw.challenge
  }
  // Backend may emit "dns-01" + a manual flag derived from missing credential.
  if (raw.challenge_type === "dns-01" || raw.challenge_type === "dns01") {
    return raw.dns_credential_id ? "dns01-auto" : "dns01-manual"
  }
  return "dns01-auto"
}

function mapOrder(raw: RawOrder): Order {
  const manualList = raw.manual_challenges
    ? raw.manual_challenges.map(mapManualChallenge)
    : raw.manual_challenge
      ? [mapManualChallenge(raw.manual_challenge)]
      : undefined
  const events: OrderEvent[] = (raw.events ?? []).map((e) => ({
    at: e.at,
    action: e.action,
    payload: e.payload,
  }))
  const id = raw.id ?? raw.order_id ?? ""
  const san = raw.san ?? raw.sans ?? []
  const tier: CertTier =
    raw.tier ?? (san.some((s) => s.startsWith("*.")) ? "wildcard" : "free")
  return {
    id,
    san,
    tier,
    ca: raw.ca ?? "letsencrypt",
    challenge: mapChallenge(raw),
    status: raw.status ?? "draft",
    idempotencyKey: raw.idempotency_key ?? "",
    createdAt: raw.created_at ?? new Date().toISOString(),
    updatedAt: raw.updated_at ?? raw.created_at ?? new Date().toISOString(),
    dnsCredentialId: raw.dns_credential_id ?? undefined,
    manualChallenges: manualList,
    events,
    certId: raw.cert_id ?? undefined,
    errorMessage: raw.error_message ?? undefined,
  }
}

function mapCert(raw: RawCert): Cert {
  return {
    id: raw.id ?? raw.cert_id ?? "",
    orderId: raw.order_id ?? "",
    san: raw.san ?? raw.sans ?? [],
    issuer: raw.issuer ?? "",
    serial: raw.serial ?? "",
    fingerprintSha256: raw.fingerprint_sha256 ?? "",
    notBefore: raw.not_before ?? "",
    notAfter: raw.not_after ?? "",
    status: raw.status ?? "active",
  }
}

function mapDnsCredential(raw: RawDnsCredential): DnsCredential {
  return {
    id: raw.id ?? raw.credential_id ?? "",
    provider: raw.provider ?? "manual",
    displayName: raw.display_name ?? "",
    health: raw.health ?? "unknown",
    createdAt: raw.created_at ?? new Date().toISOString(),
    fingerprint: raw.fingerprint ?? undefined,
  }
}

function mapQuota(raw: RawQuota): CertQuota {
  return {
    used: raw.used ?? 0,
    limit: raw.limit ?? 0,
    expiringSoon: raw.expiring_soon ?? 0,
    monthlySuccessRate: raw.monthly_success_rate ?? 0,
  }
}

function unwrapList<R>(body: unknown, key?: string): R[] {
  if (Array.isArray(body)) return body as R[]
  if (body && typeof body === "object") {
    const obj = body as Record<string, unknown>
    for (const k of [key, "items", "orders", "certs", "dns_credentials", "data"]) {
      if (k && Array.isArray(obj[k])) return obj[k] as R[]
    }
  }
  return []
}

function newIdempotencyKey(): string {
  const rand = Math.random().toString(36).slice(2, 10)
  const ts = Date.now().toString(36)
  return `idem-${ts}-${rand}`
}

// ── Orders ──────────────────────────────────────────────────────────────────

export async function listOrders(): Promise<Order[]> {
  const body = await request<unknown>("/orders")
  const items = unwrapList<RawOrder>(body, "orders")
  return items
    .map(mapOrder)
    .sort((a, b) => (a.createdAt < b.createdAt ? 1 : -1))
}

export async function getOrder(id: string): Promise<Order | null> {
  try {
    const raw = await request<RawOrder>(`/orders/${encodeURIComponent(id)}`)
    return mapOrder(raw)
  } catch (err) {
    if (err instanceof CertAPIError && err.httpStatus === 404) return null
    throw err
  }
}

export async function createOrder(
  req: CreateOrderRequest,
): Promise<{ id: string }> {
  const payload = {
    sans: req.san,
    san: req.san, // include both keys for forward-compat
    ca: req.ca,
    challenge: req.challenge,
    challenge_type:
      req.challenge === "dns01-auto" || req.challenge === "dns01-manual"
        ? "dns-01"
        : req.challenge,
    dns_credential_id: req.dnsCredentialId,
    idempotency_key: newIdempotencyKey(),
  }
  const raw = await request<RawOrder>("/orders", {
    method: "POST",
    body: JSON.stringify(payload),
  })
  const id = raw.id ?? raw.order_id ?? ""
  return { id }
}

export async function retryOrder(id: string): Promise<Order | null> {
  try {
    const raw = await request<RawOrder>(
      `/orders/${encodeURIComponent(id)}/retry`,
      { method: "POST" },
    )
    // 204 path → request returns undefined; re-fetch the order so the UI gets
    // the new status/events.
    if (raw === undefined || raw === null) {
      return getOrder(id)
    }
    return mapOrder(raw)
  } catch (err) {
    if (err instanceof CertAPIError && err.httpStatus === 404) return null
    throw err
  }
}

/**
 * Mark a manual DNS-01 challenge as "TXT record added" so the worker can
 * resume validation. The caller passes the FQDN + value the user copied from
 * the order detail page; the backend cross-checks against the stored
 * challenge before proceeding.
 */
export async function markManualReady(
  orderID: string,
  fqdn: string,
  value: string,
): Promise<Order | null> {
  try {
    const raw = await request<RawOrder | undefined>(
      `/orders/${encodeURIComponent(orderID)}/manual-ready`,
      {
        method: "POST",
        body: JSON.stringify({ fqdn, value }),
      },
    )
    if (raw === undefined || raw === null) {
      return getOrder(orderID)
    }
    return mapOrder(raw)
  } catch (err) {
    if (err instanceof CertAPIError && err.httpStatus === 404) return null
    throw err
  }
}

/**
 * Back-compat alias kept for any callers still on the W2 wording. Prefer
 * {@link markManualReady}.
 */
export async function confirmManualChallenge(
  id: string,
  fqdn = "",
  value = "",
): Promise<Order | null> {
  return markManualReady(id, fqdn, value)
}

// ── Certs ───────────────────────────────────────────────────────────────────

export async function listCerts(): Promise<Cert[]> {
  const body = await request<unknown>("/certs")
  const items = unwrapList<RawCert>(body, "certs")
  return items.map(mapCert).sort((a, b) => (a.notAfter < b.notAfter ? -1 : 1))
}

export async function getCert(id: string): Promise<Cert | null> {
  try {
    const raw = await request<RawCert>(`/certs/${encodeURIComponent(id)}`)
    return mapCert(raw)
  } catch (err) {
    if (err instanceof CertAPIError && err.httpStatus === 404) return null
    throw err
  }
}

export async function downloadCert(
  id: string,
  format: DownloadFormat,
): Promise<{ url: string; filename: string }> {
  // The backend returns {download_url, expires_at}. We translate format
  // names to the server's vocabulary: nginx-fullchain → nginx, pkcs12 → pfx.
  const serverFormat =
    format === "pkcs12"
      ? "pfx"
      : format === "nginx-fullchain"
        ? "nginx"
        : "pem"
  const raw = await request<{
    download_url?: string
    url?: string
    filename?: string
    expires_at?: string
  }>(`/certs/${encodeURIComponent(id)}/download`, {
    method: "POST",
    body: JSON.stringify({ format: serverFormat }),
  })
  const url = raw.download_url ?? raw.url ?? ""
  const ext = format === "pkcs12" ? "p12" : "pem"
  const filename = raw.filename ?? `${id}.${ext}`
  return { url, filename }
}

export async function revokeCert(
  id: string,
  reason: RevokeReason,
): Promise<Cert | null> {
  try {
    const raw = await request<RawCert | undefined>(
      `/certs/${encodeURIComponent(id)}/revoke`,
      {
        method: "POST",
        body: JSON.stringify({ reason }),
      },
    )
    if (raw === undefined || raw === null) {
      return getCert(id)
    }
    return mapCert(raw)
  } catch (err) {
    if (err instanceof CertAPIError && err.httpStatus === 404) return null
    throw err
  }
}

// ── DNS credentials ─────────────────────────────────────────────────────────

export async function listDnsCredentials(): Promise<DnsCredential[]> {
  const body = await request<unknown>("/dns-credentials")
  const items = unwrapList<RawDnsCredential>(body, "dns_credentials")
  return items.map(mapDnsCredential)
}

export async function createDnsCredential(
  req: CreateDnsCredentialRequest,
): Promise<DnsCredential> {
  const payload: Record<string, unknown> = {
    provider: req.provider,
    display_name: req.displayName,
  }
  if (req.apiToken) {
    payload.secrets = { api_token: req.apiToken }
    // Some backends accept the flat field too; include it as a fallback.
    payload.api_token = req.apiToken
  } else if (req.credential) {
    // S2 multi-provider path: aliyun / dnspod / route53 / gcloud all
    // pass their fields verbatim under `secrets` per the cert-svc
    // dns_credentials API contract.
    payload.secrets = req.credential
  }
  const raw = await request<RawDnsCredential>("/dns-credentials", {
    method: "POST",
    body: JSON.stringify(payload),
  })
  return mapDnsCredential(raw)
}

export async function testDnsCredential(
  id: string,
): Promise<{ ok: boolean; message: string }> {
  try {
    const raw = await request<{
      ok?: boolean
      healthy?: boolean
      message?: string
      health?: DnsCredentialHealth
    }>(`/dns-credentials/${encodeURIComponent(id)}/health-check`, {
      method: "POST",
    })
    if (raw === undefined || raw === null) {
      return { ok: true, message: "已触发健康检查" }
    }
    const ok =
      raw.ok ??
      raw.healthy ??
      (raw.health ? raw.health === "healthy" : true)
    return { ok, message: raw.message ?? (ok ? "连接成功" : "连接失败") }
  } catch (err) {
    if (err instanceof CertAPIError) {
      return { ok: false, message: err.message || err.code }
    }
    throw err
  }
}

export async function deleteDnsCredential(id: string): Promise<void> {
  await request<void>(`/dns-credentials/${encodeURIComponent(id)}`, {
    method: "DELETE",
  })
}

// ── Quota ───────────────────────────────────────────────────────────────────

export async function getQuota(): Promise<CertQuota> {
  const raw = await request<RawQuota>("/quota")
  return mapQuota(raw)
}
