/**
 * Verdict (evidence report) API client.
 *
 * Wraps the v2 evidence + attestation endpoints described in
 * docs/prd/18-evidence-and-attestation.md §6. All authenticated calls go
 * through the shared `apiRequest` helper (CSRF + locale + HttpOnly cookie
 * auth). The public /verify endpoint accepts multipart POST without auth,
 * but we still reuse `apiRequest` so locale + CSRF headers flow through.
 */

import { apiRequest, API_BASE } from "@/lib/api"

// ── Domain types ────────────────────────────────────────────────────────────

export type VerdictTemplate = "sla" | "incident" | "compliance" | "legal"

export type VerdictOrderStatus =
  | "pending"
  | "paid"
  | "generating"
  | "delivered"
  | "failed"
  | "refunded"

export interface VerdictOrder {
  id: string
  owner_id?: string
  status: VerdictOrderStatus
  template: VerdictTemplate
  target: string
  time_window_start: string // ISO-8601
  time_window_end: string // ISO-8601
  price_cny: number
  price_paid_cny?: number
  ext_order_id?: string
  report_id?: string
  created_at?: string
  paid_at?: string
  delivered_at?: string
}

export interface VerdictReport {
  id: string
  order_id: string
  pdf_url: string
  content_hash: string
  tsa_provider: string
  tsa_time: string
  self_verify_status: "pass" | "fail" | "pending"
  archived_url?: string
}

export interface CreateVerdictOrderInput {
  template: VerdictTemplate
  target: string
  time_window_start: string // ISO-8601
  time_window_end: string // ISO-8601
  channel: string
  return_url?: string
}

export interface CreateVerdictOrderResponse {
  order_id: string
  pay_url: string
  price_cny: number
}

// ── Attestation verify types (v2 D-Concern1) ────────────────────────────────

export interface AttestVerifyResult {
  valid: boolean
  signature_chain: string
  public_key_fingerprint: string
  signed_at: string
  tsa_provider: string
  content_hash: string
  /** Default = "observation_only" — third parties must respect this label. */
  report_type: string
  /** Verbatim legal disclaimer required by D-Concern1. Always render as-is. */
  legal_disclaimer: string
}

// ── Wrapper helpers ─────────────────────────────────────────────────────────

/**
 * Some backend endpoints wrap responses in `{ data: T }`, others return T
 * directly. This helper accepts either.
 */
function unwrap<T>(res: T | { data: T }): T {
  if (res && typeof res === "object" && "data" in (res as Record<string, unknown>)) {
    return (res as { data: T }).data
  }
  return res as T
}

// ── Order CRUD ──────────────────────────────────────────────────────────────

export async function createVerdictOrder(
  input: CreateVerdictOrderInput,
): Promise<CreateVerdictOrderResponse> {
  const res = await apiRequest<CreateVerdictOrderResponse | { data: CreateVerdictOrderResponse }>(
    "/v1/verdict/orders",
    {
      method: "POST",
      body: JSON.stringify(input),
    },
  )
  return unwrap(res)
}

export async function getVerdictOrder(id: string): Promise<VerdictOrder> {
  const res = await apiRequest<VerdictOrder | { data: VerdictOrder }>(
    `/v1/verdict/orders/${encodeURIComponent(id)}`,
  )
  return unwrap(res)
}

export async function getVerdictReport(id: string): Promise<VerdictReport> {
  const res = await apiRequest<VerdictReport | { data: VerdictReport }>(
    `/v1/verdict/reports/${encodeURIComponent(id)}`,
  )
  return unwrap(res)
}

// ── Public PDF verify (no auth) ─────────────────────────────────────────────

/**
 * Upload a signed PDF for verification.
 *
 * Uses raw fetch (not apiRequest) because the public verify endpoint must
 * gracefully degrade when the user is not logged in — we don't want any
 * accidental credential cookies attached to a public read-only call.
 *
 * @throws Error with parsed `error.message` from the backend on non-2xx.
 */
export async function verifyPdf(file: File): Promise<AttestVerifyResult> {
  const form = new FormData()
  form.append("pdf", file)

  const res = await fetch(`${API_BASE}/v1/attest/verify`, {
    method: "POST",
    body: form,
  })

  if (!res.ok) {
    let message = "Verify request failed"
    try {
      const body = (await res.json()) as { error?: { message?: string }; message?: string }
      message = body?.error?.message ?? body?.message ?? message
    } catch {
      message = res.statusText || message
    }
    throw new Error(message)
  }

  const body = (await res.json()) as AttestVerifyResult | { data: AttestVerifyResult }
  return unwrap(body)
}

// ── UI helpers ──────────────────────────────────────────────────────────────

/** Badge variants per status, used by the detail page. */
export function statusBadgeVariant(
  status: VerdictOrderStatus,
): "default" | "secondary" | "destructive" | "success" | "warning" | "info" {
  switch (status) {
    case "delivered":
      return "success"
    case "paid":
    case "generating":
      return "info"
    case "pending":
      return "warning"
    case "failed":
    case "refunded":
      return "destructive"
    default:
      return "secondary"
  }
}

/** Chinese labels for status pills. */
export const VERDICT_STATUS_LABELS: Record<VerdictOrderStatus, string> = {
  pending: "待支付",
  paid: "已支付",
  generating: "生成中",
  delivered: "已交付",
  failed: "失败",
  refunded: "已退款",
}

/** Chinese labels for the four template options. */
export const VERDICT_TEMPLATE_LABELS: Record<VerdictTemplate, string> = {
  sla: "服务等级",
  incident: "故障事件",
  compliance: "合规归档",
  legal: "法律证据",
}

/**
 * Statuses that should keep polling (the order is still in flight).
 * Used by the detail page polling effect.
 */
export function isPollingStatus(status: VerdictOrderStatus): boolean {
  return status === "paid" || status === "generating"
}
