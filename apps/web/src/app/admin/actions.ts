"use server"

// Server-only helpers for admin operations. Keeping ADMIN_TOKEN out of any
// "use client" file prevents it from leaking into the client bundle.
const API_BASE = process.env.INTERNAL_API_URL ?? "http://localhost:8080"
const ADMIN_TOKEN = process.env.ADMIN_TOKEN ?? ""

export async function sendTestEmailAction(to: string): Promise<{ ok: boolean; message?: string }> {
  if (!ADMIN_TOKEN) return { ok: false, message: "ADMIN_TOKEN not configured" }
  try {
    const res = await fetch(
      `${API_BASE}/internal/admin/test-email?to=${encodeURIComponent(to)}`,
      {
        method: "POST",
        headers: { Authorization: `Bearer ${ADMIN_TOKEN}` },
        cache: "no-store",
      },
    )
    if (!res.ok) return { ok: false, message: `request failed (${res.status})` }
    const body = (await res.json().catch(() => ({}))) as { data?: { message?: string } }
    return { ok: true, message: body?.data?.message }
  } catch (err) {
    return { ok: false, message: err instanceof Error ? err.message : "network error" }
  }
}
