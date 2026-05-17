import { defaultLocale, isSupported } from "@/i18n/registry"

export const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080"

/** Default request timeout (30 s). Override per-call via AbortSignal in options. */
const DEFAULT_TIMEOUT_MS = 30_000

/** Read the non-HttpOnly csrf_token cookie set by the server on GET requests. */
function getCsrfToken(): string {
  if (typeof document === "undefined") return ""
  const match = document.cookie.match(/(?:^|;\s*)csrf_token=([^;]+)/)
  return match ? decodeURIComponent(match[1]!) : ""
}

/**
 * Resolve the active locale for outbound requests, in priority order:
 *   1. `idcd_locale` cookie (new canonical name)
 *   2. `locale` cookie (legacy; honored so cross-deploy sessions don't reset)
 *   3. registry default
 *
 * Always returns a registry-supported short code (cn / en / …). On the server
 * (no `document`), we fall back to the default and rely on layout to pass
 * an `X-Locale` via the request — most browser API calls happen client-side.
 */
function getCurrentLocale(): string {
  if (typeof document === "undefined") return defaultLocale
  const cookies = document.cookie
  const freshMatch = cookies.match(/(?:^|;\s*)idcd_locale=([^;]+)/)
  const fresh = freshMatch ? decodeURIComponent(freshMatch[1]!) : ""
  if (fresh && isSupported(fresh)) return fresh
  const legacyMatch = cookies.match(/(?:^|;\s*)locale=([^;]+)/)
  const legacy = legacyMatch ? decodeURIComponent(legacyMatch[1]!) : ""
  if (legacy && isSupported(legacy)) return legacy
  return defaultLocale
}

const MUTATING = new Set(["POST", "PUT", "PATCH", "DELETE"])

export async function apiRequest<T = unknown>(path: string, options?: RequestInit): Promise<T> {
  const method = (options?.method ?? "GET").toUpperCase()

  // Do not set a default Content-Type when the body is FormData — the browser
  // needs to set it automatically (including the multipart boundary parameter).
  const defaultHeaders: HeadersInit =
    options?.body instanceof FormData ? {} : { "Content-Type": "application/json" }

  // Inject locale on every request so the backend (and any downstream service)
  // can localize error messages / templates without re-reading cookies.
  const localeHeaders: HeadersInit = { "X-Locale": getCurrentLocale() }

  // Double-submit CSRF pattern: include the cookie value in the X-CSRF-Token header
  // for all mutating requests. /v1/auth/* is exempt server-side, but sending the
  // header there is harmless.
  const csrfHeaders: HeadersInit =
    MUTATING.has(method) ? { "X-CSRF-Token": getCsrfToken() } : {}

  // Apply a default timeout unless the caller already provided a signal.
  const ownController = options?.signal ? null : new AbortController()
  const timeoutId = ownController
    ? setTimeout(() => ownController.abort(), DEFAULT_TIMEOUT_MS)
    : null

  try {
    const res = await fetch(API_BASE + path, {
      ...options,
      // credentials: "include" sends the HttpOnly access_token cookie automatically.
      credentials: "include",
      headers: { ...defaultHeaders, ...localeHeaders, ...csrfHeaders, ...options?.headers },
      signal: options?.signal ?? ownController?.signal,
    })

    if (!res.ok) {
      let errorMessage = "Request failed"
      try {
        const err = await res.json()
        errorMessage = err?.error?.message || err?.message || errorMessage
      } catch {
        errorMessage = res.statusText || errorMessage
      }
      throw new Error(errorMessage)
    }

    return res.json()
  } finally {
    if (timeoutId !== null) clearTimeout(timeoutId)
  }
}

export interface Node {
  id: string
  name: string
  country_code: string
  region?: string
  city?: string
  asn?: string
  isp?: string
  tier?: string
  status: string
  is_active: boolean
}

export interface ProbeParams {
  target: string
  node_ids?: string[]
  /** Probe-type-specific extra fields (e.g. method, count, record_type). */
  [key: string]: unknown
}

export interface ProbeResult {
  task_id: string
  status: string
  results?: Array<{
    node_id: string
    node_name: string
    success: boolean
    latency_ms?: number
    error?: string
    details?: Record<string, unknown>
  }>
}

// Response from GET /v1/probe/tasks/{taskId}
export interface ProbeTaskResult {
  task_id: string
  status: string  // queued | running | completed | failed | cancelled
  result?: {
    node_id?: string
    success?: boolean
    duration_ms?: number
    error?: string
    [key: string]: unknown  // probe-type-specific fields
  }
  created_at: string
  completed_at?: string
}

// Nodes API
export async function getNodes(): Promise<Node[]> {
  const res = await apiRequest<{ data: { nodes: Array<Omit<Node, "is_active">>; total: number } }>("/v1/nodes")
  return (res.data?.nodes ?? []).map(n => ({ ...n, is_active: n.status === "active" }))
}

// Probe API — shared POST helper
async function probePost(endpoint: string, params: ProbeParams): Promise<ProbeResult> {
  const res = await apiRequest<{ data: ProbeResult }>(`/v1/probe/${endpoint}`, {
    method: "POST",
    body: JSON.stringify(params),
  })
  return res.data
}

export const probeHttp      = (p: ProbeParams) => probePost("http",       p)
export const probePing      = (p: ProbeParams) => probePost("ping",       p)
export const probeTcp       = (p: ProbeParams) => probePost("tcping",     p)
export const probeDns       = (p: ProbeParams) => probePost("dns",        p)
export const probeTraceroute = (p: ProbeParams) => probePost("traceroute", p)
export const probeSmtp      = (p: ProbeParams) => probePost("smtp",       p)
export const probeNtp       = (p: ProbeParams) => probePost("ntp",        p)
export const probeMtr        = (p: ProbeParams) => probePost("mtr",        p)
export const probeSpeedtest  = (p: ProbeParams) => probePost("speedtest",  p)

export async function getProbeTask(taskId: string): Promise<ProbeTaskResult> {
  const res = await apiRequest<{ data: ProbeTaskResult }>(`/v1/probe/tasks/${taskId}`)
  return res.data
}

// Info API
export interface SSLInfo {
  domain: string
  issuer: string
  valid_from: string
  valid_to: string
  days_remaining: number
  is_valid: boolean
}

export interface WhoisInfo {
  domain: string
  registrar: string
  creation_date?: string
  expiry_date?: string
  expiration_date?: string
  registrant?: string
  name_servers?: string[]
  note?: string
}

export async function getSSLInfo(domain: string): Promise<SSLInfo> {
  const res = await apiRequest<{ data: SSLInfo }>(`/v1/info/ssl?q=${encodeURIComponent(domain)}`)
  return res.data
}

export async function getWhoisInfo(domain: string): Promise<WhoisInfo> {
  const res = await apiRequest<{ data: WhoisInfo }>(`/v1/info/whois?q=${encodeURIComponent(domain)}`)
  return res.data
}

// ---- Info API: additional types & functions ----

export interface RDNSInfo {
  ip: string
  hostnames: string[]
}

export interface MXInfo {
  domain: string
  records: Array<{ host: string; priority: number }>
}

export interface SPFInfo {
  domain: string
  record: string
  found: boolean
}

export interface DMARCInfo {
  domain: string
  record: string
  found: boolean
}

export interface DKIMInfo {
  domain: string
  selector: string
  record: string
  found: boolean
}

export interface ASNInfo {
  query: string
  asn: string
  isp: string
  country: string
  country_code: string
}

export interface BGPInfo {
  ip: string
  prefixes: string[]
  asns: string[]
}

export interface ICPInfo {
  domain: string
  icp_number: string
  company: string
  type: string
  filed_at: string
  note: string
}

export async function getRDNSInfo(q: string): Promise<RDNSInfo> {
  const res = await apiRequest<{ data: RDNSInfo }>(`/v1/info/rdns?q=${encodeURIComponent(q)}`)
  return res.data
}

export async function getMXInfo(q: string): Promise<MXInfo> {
  const res = await apiRequest<{ data: MXInfo }>(`/v1/info/mx?q=${encodeURIComponent(q)}`)
  return res.data
}

export async function getSPFInfo(q: string): Promise<SPFInfo> {
  const res = await apiRequest<{ data: SPFInfo }>(`/v1/info/spf?q=${encodeURIComponent(q)}`)
  return res.data
}

export async function getDMARCInfo(q: string): Promise<DMARCInfo> {
  const res = await apiRequest<{ data: DMARCInfo }>(`/v1/info/dmarc?q=${encodeURIComponent(q)}`)
  return res.data
}

export async function getDKIMInfo(q: string, selector?: string): Promise<DKIMInfo> {
  const res = await apiRequest<{ data: DKIMInfo }>(
    `/v1/info/dkim?q=${encodeURIComponent(q)}${selector ? `&selector=${encodeURIComponent(selector)}` : ""}`
  )
  return res.data
}

export async function getASNInfo(q: string): Promise<ASNInfo> {
  const res = await apiRequest<{ data: ASNInfo }>(`/v1/info/asn?q=${encodeURIComponent(q)}`)
  return res.data
}

export async function getBGPInfo(q: string): Promise<BGPInfo> {
  const res = await apiRequest<{ data: BGPInfo }>(`/v1/info/bgp?q=${encodeURIComponent(q)}`)
  return res.data
}

export async function getICPInfo(q: string): Promise<ICPInfo> {
  const res = await apiRequest<{ data: ICPInfo }>(`/v1/info/icp?q=${encodeURIComponent(q)}`)
  return res.data
}

// Billing API
export interface Subscription {
  id: string
  plan: string
  status: "pending" | "active" | "cancelled" | "past_due"
  provider: string
  ext_sub_id?: string
  current_period_start?: string
  current_period_end?: string
  cancel_at?: string
  created_at: string
}

export interface Invoice {
  id: string
  subscription_id?: string
  amount_cents: number
  currency: string
  status: "paid" | "refunded" | "refund_failed"
  provider: string
  ext_invoice_id?: string
  paid_at?: string
  created_at: string
}

export interface InvoicesResponse {
  invoices: Invoice[]
  total: number
  page: number
  page_size: number
}

export interface SubscribeResponse {
  subscription_id: string
  pay_url: string
  expires_at: string
}

export async function getSubscription(): Promise<Subscription | null> {
  try {
    const res = await apiRequest<{ data: Subscription }>("/v1/billing/subscription")
    return res.data
  } catch {
    return null
  }
}

export async function getInvoices(page = 1, pageSize = 20): Promise<InvoicesResponse> {
  const res = await apiRequest<{ data: InvoicesResponse }>(
    `/v1/billing/invoices?page=${page}&page_size=${pageSize}`
  )
  return res.data
}

export async function subscribePlan(plan: string, channel?: string): Promise<SubscribeResponse> {
  const res = await apiRequest<{ data: SubscribeResponse }>("/v1/billing/subscribe", {
    method: "POST",
    body: JSON.stringify({ plan, ...(channel ? { channel } : {}) }),
  })
  return res.data
}

export async function cancelSubscription(): Promise<void> {
  await apiRequest("/v1/billing/cancel", { method: "POST" })
}
