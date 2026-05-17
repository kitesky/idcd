// Types and mock data for the free certificate platform.
//
// All shapes mirror the planned /api/v1/cert/* responses (see
// docs/prd/20-free-cert.md). When the backend lands in W3 we keep these
// types unchanged and replace the bodies inside cert-api.ts.

export type CaProvider = "letsencrypt" | "zerossl" | "buypass" | "gts"

export type ChallengeMode = "dns01-auto" | "dns01-manual"

export type CertTier = "free" | "ev" | "ov" | "wildcard"

export type OrderStatus =
  | "draft"
  | "validating"
  | "issuing"
  | "issued"
  | "failed"
  | "revoked"

export type CertStatus = "active" | "expired" | "revoked"

export type DnsProvider =
  | "cloudflare"
  | "manual"
  | "aliyun"
  | "dnspod"
  | "route53"
  | "gcloud"

export type DnsCredentialHealth = "healthy" | "degraded" | "revoked" | "unknown"

export type DownloadFormat = "pem" | "pkcs12" | "nginx-fullchain"

export type RevokeReason =
  | "unspecified"
  | "keyCompromise"
  | "cessationOfOperation"

export interface OrderEvent {
  at: string
  action: string
  payload?: Record<string, unknown>
}

export interface ManualChallenge {
  recordName: string
  recordValue: string
}

export interface Order {
  id: string
  san: string[]
  tier: CertTier
  ca: CaProvider
  challenge: ChallengeMode
  status: OrderStatus
  idempotencyKey: string
  createdAt: string
  updatedAt: string
  dnsCredentialId?: string
  manualChallenges?: ManualChallenge[]
  events: OrderEvent[]
  certId?: string
  errorMessage?: string
}

export interface Cert {
  id: string
  orderId: string
  san: string[]
  issuer: string
  serial: string
  fingerprintSha256: string
  notBefore: string
  notAfter: string
  status: CertStatus
}

export interface DnsCredential {
  id: string
  provider: DnsProvider
  displayName: string
  health: DnsCredentialHealth
  createdAt: string
  // The token itself is never returned by the API; we only keep a fingerprint.
  fingerprint?: string
}

export interface CreateOrderRequest {
  san: string[]
  ca: CaProvider
  challenge: ChallengeMode
  dnsCredentialId?: string
}

export interface CreateDnsCredentialRequest {
  provider: DnsProvider
  displayName: string
  apiToken?: string
  // Generic credential map for non-cloudflare providers. Keys MUST match the
  // backend credential field names (e.g. access_key_id, secret_access_key,
  // service_account_json). cert-api.ts forwards the map as `secrets`.
  credential?: Record<string, string>
}

export interface CertQuota {
  used: number
  limit: number
  expiringSoon: number
  monthlySuccessRate: number
}

// ── CA metadata ──────────────────────────────────────────────────────────────

export interface CaMetadata {
  id: CaProvider
  label: string
  validityDays: number
  rateLimit: string
  supportsWildcard: boolean
}

export const CA_OPTIONS: CaMetadata[] = [
  {
    id: "letsencrypt",
    label: "Let's Encrypt",
    validityDays: 90,
    rateLimit: "300 张 / 周",
    supportsWildcard: true,
  },
  {
    id: "zerossl",
    label: "ZeroSSL",
    validityDays: 90,
    rateLimit: "50 张 / 月（免费档）",
    supportsWildcard: true,
  },
  {
    id: "buypass",
    label: "Buypass Go",
    validityDays: 180,
    rateLimit: "20 张 / 域名 / 月",
    supportsWildcard: false,
  },
  {
    id: "gts",
    label: "Google Trust Services",
    validityDays: 90,
    rateLimit: "需要 EAB 凭据",
    supportsWildcard: true,
  },
]

export const TIER_LABELS: Record<CertTier, string> = {
  free: "免费 DV",
  ev: "EV",
  ov: "OV",
  wildcard: "通配符",
}

export const ORDER_STATUS_LABELS: Record<OrderStatus, string> = {
  draft: "草稿",
  validating: "验证中",
  issuing: "签发中",
  issued: "已签发",
  failed: "失败",
  revoked: "已撤销",
}

export const CERT_STATUS_LABELS: Record<CertStatus, string> = {
  active: "有效",
  expired: "已过期",
  revoked: "已撤销",
}

export const DNS_PROVIDER_LABELS: Record<DnsProvider, string> = {
  cloudflare: "Cloudflare",
  manual: "手动 DNS",
  aliyun: "阿里云 DNS",
  dnspod: "DNSPod / 腾讯云 DNS",
  route53: "AWS Route 53",
  gcloud: "Google Cloud DNS",
}

export const DNS_HEALTH_LABELS: Record<DnsCredentialHealth, string> = {
  healthy: "健康",
  degraded: "异常",
  revoked: "已吊销",
  unknown: "未检测",
}

export const REVOKE_REASON_LABELS: Record<RevokeReason, string> = {
  unspecified: "未指定",
  keyCompromise: "私钥泄露",
  cessationOfOperation: "停止运营",
}

// ── Mock data ────────────────────────────────────────────────────────────────

const now = Date.now()
const day = 86_400_000

export const MOCK_ORDERS: Order[] = [
  {
    id: "ord-001",
    san: ["idcd.com", "www.idcd.com"],
    tier: "free",
    ca: "letsencrypt",
    challenge: "dns01-auto",
    status: "issued",
    idempotencyKey: "idem-2025-04-10-001",
    createdAt: new Date(now - 5 * day).toISOString(),
    updatedAt: new Date(now - 5 * day + 60_000).toISOString(),
    dnsCredentialId: "dns-001",
    certId: "cert-001",
    events: [
      { at: new Date(now - 5 * day).toISOString(), action: "order.created" },
      {
        at: new Date(now - 5 * day + 30_000).toISOString(),
        action: "challenge.dns.propagated",
      },
      {
        at: new Date(now - 5 * day + 50_000).toISOString(),
        action: "ca.finalize.ok",
      },
      { at: new Date(now - 5 * day + 60_000).toISOString(), action: "issued" },
    ],
  },
  {
    id: "ord-002",
    san: ["api.idcd.com"],
    tier: "free",
    ca: "letsencrypt",
    challenge: "dns01-manual",
    status: "validating",
    idempotencyKey: "idem-2025-05-15-042",
    createdAt: new Date(now - 2 * 3600_000).toISOString(),
    updatedAt: new Date(now - 30 * 60_000).toISOString(),
    manualChallenges: [
      {
        recordName: "_acme-challenge.api.idcd.com",
        recordValue: "Xm-7vN4qP8aZyL1RtUvWxYzAbCdEfGhIjKlMnOpQrSt",
      },
    ],
    events: [
      {
        at: new Date(now - 2 * 3600_000).toISOString(),
        action: "order.created",
      },
      {
        at: new Date(now - 2 * 3600_000 + 5_000).toISOString(),
        action: "challenge.dns.pending",
      },
    ],
  },
  {
    id: "ord-003",
    san: ["*.dev.idcd.com"],
    tier: "wildcard",
    ca: "zerossl",
    challenge: "dns01-auto",
    status: "issuing",
    idempotencyKey: "idem-2025-05-16-007",
    createdAt: new Date(now - 6 * 60_000).toISOString(),
    updatedAt: new Date(now - 30_000).toISOString(),
    dnsCredentialId: "dns-001",
    events: [
      { at: new Date(now - 6 * 60_000).toISOString(), action: "order.created" },
      {
        at: new Date(now - 4 * 60_000).toISOString(),
        action: "challenge.dns.propagated",
      },
      { at: new Date(now - 30_000).toISOString(), action: "ca.finalize.pending" },
    ],
  },
  {
    id: "ord-004",
    san: ["staging.idcd.com"],
    tier: "free",
    ca: "buypass",
    challenge: "dns01-auto",
    status: "failed",
    idempotencyKey: "idem-2025-05-15-051",
    createdAt: new Date(now - 12 * 3600_000).toISOString(),
    updatedAt: new Date(now - 11 * 3600_000).toISOString(),
    dnsCredentialId: "dns-002",
    errorMessage: "CA 拒绝：CAA 记录禁止 buypass.com 签发",
    events: [
      {
        at: new Date(now - 12 * 3600_000).toISOString(),
        action: "order.created",
      },
      {
        at: new Date(now - 11 * 3600_000).toISOString(),
        action: "ca.finalize.failed",
        payload: { code: "caa-forbidden" },
      },
    ],
  },
  {
    id: "ord-005",
    san: ["old.idcd.com"],
    tier: "free",
    ca: "letsencrypt",
    challenge: "dns01-auto",
    status: "revoked",
    idempotencyKey: "idem-2025-03-01-010",
    createdAt: new Date(now - 60 * day).toISOString(),
    updatedAt: new Date(now - 7 * day).toISOString(),
    dnsCredentialId: "dns-001",
    certId: "cert-003",
    events: [
      { at: new Date(now - 60 * day).toISOString(), action: "order.created" },
      { at: new Date(now - 60 * day + 60_000).toISOString(), action: "issued" },
      {
        at: new Date(now - 7 * day).toISOString(),
        action: "cert.revoked",
        payload: { reason: "cessationOfOperation" },
      },
    ],
  },
]

export const MOCK_CERTS: Cert[] = [
  {
    id: "cert-001",
    orderId: "ord-001",
    san: ["idcd.com", "www.idcd.com"],
    issuer: "Let's Encrypt R10",
    serial: "03:a1:5f:7b:c9:11:00:00:00:00:00:00:00:00:00:01",
    fingerprintSha256:
      "AA:BB:CC:DD:EE:FF:00:11:22:33:44:55:66:77:88:99:AA:BB:CC:DD:EE:FF:00:11:22:33:44:55:66:77:88:99",
    notBefore: new Date(now - 5 * day).toISOString(),
    notAfter: new Date(now + 85 * day).toISOString(),
    status: "active",
  },
  {
    id: "cert-002",
    orderId: "ord-007",
    san: ["edge.idcd.com"],
    issuer: "Let's Encrypt R10",
    serial: "03:a1:5f:7b:c9:11:00:00:00:00:00:00:00:00:00:02",
    fingerprintSha256:
      "11:22:33:44:55:66:77:88:99:AA:BB:CC:DD:EE:FF:00:11:22:33:44:55:66:77:88:99:AA:BB:CC:DD:EE:FF:00",
    notBefore: new Date(now - 75 * day).toISOString(),
    notAfter: new Date(now + 15 * day).toISOString(),
    status: "active",
  },
  {
    id: "cert-003",
    orderId: "ord-005",
    san: ["old.idcd.com"],
    issuer: "Let's Encrypt R10",
    serial: "03:a1:5f:7b:c9:11:00:00:00:00:00:00:00:00:00:03",
    fingerprintSha256:
      "33:44:55:66:77:88:99:AA:BB:CC:DD:EE:FF:00:11:22:33:44:55:66:77:88:99:AA:BB:CC:DD:EE:FF:00:11:22",
    notBefore: new Date(now - 60 * day).toISOString(),
    notAfter: new Date(now + 30 * day).toISOString(),
    status: "revoked",
  },
  {
    id: "cert-004",
    orderId: "ord-008",
    san: ["legacy.idcd.com"],
    issuer: "Let's Encrypt R3",
    serial: "03:a1:5f:7b:c9:11:00:00:00:00:00:00:00:00:00:04",
    fingerprintSha256:
      "55:66:77:88:99:AA:BB:CC:DD:EE:FF:00:11:22:33:44:55:66:77:88:99:AA:BB:CC:DD:EE:FF:00:11:22:33:44",
    notBefore: new Date(now - 120 * day).toISOString(),
    notAfter: new Date(now - 30 * day).toISOString(),
    status: "expired",
  },
]

export const MOCK_DNS_CREDENTIALS: DnsCredential[] = [
  {
    id: "dns-001",
    provider: "cloudflare",
    displayName: "Cloudflare 主账号",
    health: "healthy",
    createdAt: new Date(now - 40 * day).toISOString(),
    fingerprint: "cf-token-…f3a7",
  },
  {
    id: "dns-002",
    provider: "cloudflare",
    displayName: "Cloudflare staging",
    health: "degraded",
    createdAt: new Date(now - 14 * day).toISOString(),
    fingerprint: "cf-token-…b911",
  },
  {
    id: "dns-003",
    provider: "manual",
    displayName: "手动验证",
    health: "unknown",
    createdAt: new Date(now - 3 * day).toISOString(),
  },
]

export const MOCK_QUOTA: CertQuota = {
  used: 3,
  limit: 5,
  expiringSoon: 1,
  monthlySuccessRate: 95,
}

// ── Domain parsing helpers ──────────────────────────────────────────────────

/**
 * Parse a free-form SAN input (textarea content) into a normalised list of
 * domain names. Splits on newlines, commas and whitespace, trims entries,
 * lowercases ASCII characters, and de-duplicates while preserving order.
 *
 * Wildcards (`*.example.com`) are preserved. Non-ASCII labels are returned
 * verbatim — callers can run them through {@link toPunycode} when display
 * requires it.
 */
export function parseSanInput(raw: string): string[] {
  if (!raw) return []
  const parts = raw
    .split(/[\s,;\n\r]+/)
    .map((s) => s.trim())
    .filter(Boolean)
  const seen = new Set<string>()
  const out: string[] = []
  for (const p of parts) {
    const lowered = p.toLowerCase()
    if (!seen.has(lowered)) {
      seen.add(lowered)
      out.push(lowered)
    }
  }
  return out
}

/**
 * Returns true when the host contains any non-ASCII codepoint and therefore
 * needs IDN/Punycode conversion before being submitted to the CA.
 */
export function isIdn(host: string): boolean {
  // Strip a leading wildcard label so `*.中文.com` is detected correctly.
  const stripped = host.startsWith("*.") ? host.slice(2) : host
  // Match anything outside printable ASCII (space through tilde).
  return /[^\x20-\x7e]/.test(stripped)
}

/**
 * Convert a hostname to its Punycode representation when the runtime exposes
 * the standard `URL` parser. We intentionally avoid pulling in a full
 * `punycode` polyfill — the wizard only needs a visual hint, the actual
 * encoding happens on the server.
 */
export function toPunycode(host: string): string {
  if (!isIdn(host)) return host
  try {
    const wildcard = host.startsWith("*.")
    const candidate = wildcard ? host.slice(2) : host
    const u = new URL(`https://${candidate}`)
    const ascii = u.hostname
    return wildcard ? `*.${ascii}` : ascii
  } catch {
    return host
  }
}

export function isWildcard(host: string): boolean {
  return host.startsWith("*.")
}

export function isExpiringSoon(notAfter: string, withinDays = 30): boolean {
  const t = new Date(notAfter).getTime()
  if (Number.isNaN(t)) return false
  return t - Date.now() < withinDays * day && t - Date.now() > 0
}
