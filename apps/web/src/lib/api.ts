export const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080"

/** Read the non-HttpOnly csrf_token cookie set by the server on GET requests. */
function getCsrfToken(): string {
  if (typeof document === "undefined") return ""
  const match = document.cookie.match(/(?:^|;\s*)csrf_token=([^;]+)/)
  return match ? decodeURIComponent(match[1]!) : ""
}

const MUTATING = new Set(["POST", "PUT", "PATCH", "DELETE"])

export async function apiRequest<T = unknown>(path: string, options?: RequestInit): Promise<T> {
  const method = (options?.method ?? "GET").toUpperCase()

  // Do not set a default Content-Type when the body is FormData — the browser
  // needs to set it automatically (including the multipart boundary parameter).
  const defaultHeaders: HeadersInit =
    options?.body instanceof FormData ? {} : { "Content-Type": "application/json" }

  // Double-submit CSRF pattern: include the cookie value in the X-CSRF-Token header
  // for all mutating requests. /v1/auth/* is exempt server-side, but sending the
  // header there is harmless.
  const csrfHeaders: HeadersInit =
    MUTATING.has(method) ? { "X-CSRF-Token": getCsrfToken() } : {}

  const res = await fetch(API_BASE + path, {
    ...options,
    // credentials: "include" sends the HttpOnly access_token cookie automatically.
    credentials: "include",
    headers: { ...defaultHeaders, ...csrfHeaders, ...options?.headers },
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
}

export interface Node {
  id: string
  name: string
  country_code: string
  region: string
  city: string
  asn: string
  isp: string
  tier: string
  status: string
  is_active: boolean
}

export interface ProbeParams {
  target: string
  node_ids?: string[]
  [key: string]: any
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
export async function getNodes(): Promise<{ data: Node[] }> {
  return apiRequest<{ data: Node[] }>("/v1/nodes")
}

// Probe API
export async function probeHttp(params: ProbeParams): Promise<ProbeResult> {
  return apiRequest<ProbeResult>("/v1/probe/http", {
    method: "POST",
    body: JSON.stringify(params)
  })
}

export async function probePing(params: ProbeParams): Promise<ProbeResult> {
  return apiRequest<ProbeResult>("/v1/probe/ping", {
    method: "POST",
    body: JSON.stringify(params)
  })
}

export async function probeTcp(params: ProbeParams): Promise<ProbeResult> {
  return apiRequest<ProbeResult>("/v1/probe/tcping", {
    method: "POST",
    body: JSON.stringify(params)
  })
}

export async function probeDns(params: ProbeParams): Promise<ProbeResult> {
  return apiRequest<ProbeResult>("/v1/probe/dns", {
    method: "POST",
    body: JSON.stringify(params)
  })
}

export async function probeTraceroute(params: ProbeParams): Promise<ProbeResult> {
  return apiRequest<ProbeResult>("/v1/probe/traceroute", {
    method: "POST",
    body: JSON.stringify(params)
  })
}

export async function getProbeTask(taskId: string): Promise<ProbeTaskResult> {
  return apiRequest<ProbeTaskResult>(`/v1/probe/tasks/${taskId}`)
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
  expiration_date?: string
  registrant?: string
  name_servers?: string[]
}

export async function getSSLInfo(domain: string): Promise<SSLInfo> {
  return apiRequest<SSLInfo>(`/v1/info/ssl?q=${encodeURIComponent(domain)}`)
}

export async function getWhoisInfo(domain: string): Promise<WhoisInfo> {
  return apiRequest<WhoisInfo>(`/v1/info/whois?q=${encodeURIComponent(domain)}`)
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
    return await apiRequest<Subscription>("/v1/billing/subscription")
  } catch {
    return null
  }
}

export async function getInvoices(page = 1, pageSize = 20): Promise<InvoicesResponse> {
  return apiRequest<InvoicesResponse>(
    `/v1/billing/invoices?page=${page}&page_size=${pageSize}`
  )
}

export async function subscribePlan(plan: string, channel?: string): Promise<SubscribeResponse> {
  return apiRequest<SubscribeResponse>("/v1/billing/subscribe", {
    method: "POST",
    body: JSON.stringify({ plan, ...(channel ? { channel } : {}) }),
  })
}

export async function cancelSubscription(): Promise<void> {
  await apiRequest("/v1/billing/cancel", { method: "POST" })
}
