// admin-cert-api.ts — typed fetch wrappers around /v1/admin/cert/*.
//
// Two execution contexts use these helpers:
//   - Server Components (RSC): pass an absolute URL via the `base` arg so
//     Node.js's fetch can reach the cert-svc admin endpoint over the
//     internal network. `cache: "no-store"` keeps SSR data fresh.
//   - Client Components: omit `base`; the request resolves relative to
//     the browser origin and rides the existing admin cookie + reverse
//     proxy. No CORS handling needed.
//
// Each call returns `null` on a non-2xx response so the caller can render
// an empty / error state without needing try/catch boilerplate. Throwing
// errors are confined to network failures, which the caller bubbles to
// the user via an Alert.

export interface AdminCertOrder {
  id: number
  account_id: number
  sans: string[]
  sans_unicode: string[]
  status: string
  tier: string
  ca: string
  challenge_type: string
  dns_credential_id?: number | null
  cert_id?: number | null
  retry_count: number
  last_error?: string | null
  created_at: string
  finalized_at?: string | null
}

export interface AdminListOrdersResponse {
  orders: AdminCertOrder[]
  limit: number
  offset: number
}

export interface AdminOrderFilter {
  status?: string
  account_id?: string
  ca?: string
  limit?: number
  offset?: number
}

export interface AdminCAQuotaRow {
  ca: string
  per_account_3h: number
  per_registered_domain: number
  switched: boolean
  err?: string
}

export interface AdminCAQuotaResponse {
  rows: AdminCAQuotaRow[]
  switch_threshold: number
}

export interface AdminDNSHealthRow {
  provider: string
  success_rate: number  // 0..1 or -1 when unknown
  samples: number
  window_hours: number
}

export interface AdminDNSHealthResponse {
  rows: AdminDNSHealthRow[]
}

// buildQuery converts a filter object into a URLSearchParams string.
// Skips empty / undefined / null values so the resulting URL stays
// short and `?` only appears when there is at least one param.
export function buildQuery(f: Record<string, string | number | undefined | null>): string {
  const params = new URLSearchParams()
  for (const [k, v] of Object.entries(f)) {
    if (v === undefined || v === null) continue
    const s = String(v).trim()
    if (s === "") continue
    params.set(k, s)
  }
  const q = params.toString()
  return q ? `?${q}` : ""
}

async function getJSON<T>(path: string, base = ""): Promise<T | null> {
  try {
    const res = await fetch(`${base}${path}`, { cache: "no-store" })
    if (!res.ok) return null
    return (await res.json()) as T
  } catch {
    return null
  }
}

async function postJSON<T>(path: string, body: unknown, base = ""): Promise<{ ok: true; data: T } | { ok: false; status: number; message: string }> {
  try {
    const res = await fetch(`${base}${path}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    })
    if (!res.ok) {
      let message = `HTTP ${res.status}`
      try {
        const errBody = await res.json()
        if (typeof errBody?.message === "string") message = errBody.message
      } catch {
        // ignore parse failures — fall back to HTTP status
      }
      return { ok: false, status: res.status, message }
    }
    const data = (await res.json()) as T
    return { ok: true, data }
  } catch (err) {
    return { ok: false, status: 0, message: err instanceof Error ? err.message : String(err) }
  }
}

export function listOrders(filter: AdminOrderFilter, base = ""): Promise<AdminListOrdersResponse | null> {
  return getJSON<AdminListOrdersResponse>(`/v1/admin/cert/orders${buildQuery({ ...filter })}`, base)
}

export function forceFail(orderId: number, reason: string, base = "") {
  return postJSON<{ order_id: number; status: string }>(
    `/v1/admin/cert/orders/${orderId}/force-fail`, { reason }, base,
  )
}

export function getCAQuota(base = ""): Promise<AdminCAQuotaResponse | null> {
  return getJSON<AdminCAQuotaResponse>(`/v1/admin/cert/ca-quota`, base)
}

export function getDNSHealth(base = ""): Promise<AdminDNSHealthResponse | null> {
  return getJSON<AdminDNSHealthResponse>(`/v1/admin/cert/dns-health`, base)
}

export function banAccount(accountId: number, reason: string, base = "") {
  return postJSON<{ account_id: number; status: string }>(
    `/v1/admin/cert/accounts/${accountId}/ban`, { reason }, base,
  )
}

export function unbanAccount(accountId: number, reason: string, base = "") {
  return postJSON<{ account_id: number; status: string }>(
    `/v1/admin/cert/accounts/${accountId}/unban`, { reason }, base,
  )
}

// formatRate renders a 0..1 success rate as a percentage with one decimal,
// returning a placeholder when the input is the "-1 = unknown" sentinel.
export function formatRate(rate: number, unknownLabel = "—"): string {
  if (rate < 0 || Number.isNaN(rate)) return unknownLabel
  return `${(rate * 100).toFixed(1)}%`
}

// formatPercent is the equivalent for 0..1 quota usage where -1 is not
// a valid value; we still guard NaN to keep the UI from rendering "NaN%".
export function formatPercent(v: number): string {
  if (Number.isNaN(v)) return "—"
  return `${(Math.max(0, Math.min(1, v)) * 100).toFixed(1)}%`
}
