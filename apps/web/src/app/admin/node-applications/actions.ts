"use server"

// Server actions cannot call useTranslations/getTranslations safely without a
// locale context — return a stable shape and let the client translate. The
// `messageKey` is an admin namespace path (e.g. nodeApplications.errors.X);
// `message` is the raw upstream API error message (already localized by API).
export interface ReviewApplicationResult {
  ok: boolean
  message?: string
  messageKey?: string
}

export async function reviewApplicationAction(
  id: string,
  action: "approve" | "reject",
  note: string,
): Promise<ReviewApplicationResult> {
  const base = process.env.INTERNAL_API_URL ?? "http://localhost:8080"
  const token = process.env.ADMIN_TOKEN ?? ""
  try {
    const res = await fetch(`${base}/v1/admin/node-applications/${id}/review`, {
      method: "PATCH",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${token}`,
      },
      body: JSON.stringify({ action, note: note || undefined }),
    })
    if (!res.ok) {
      const j = await res.json().catch(() => ({}))
      const upstream = (j as { error?: { message?: string } })?.error?.message
      if (upstream) return { ok: false, message: upstream }
      return { ok: false, messageKey: "nodeApplications.errors.reviewFailed" }
    }
    return { ok: true }
  } catch {
    return { ok: false, messageKey: "nodeApplications.errors.networkError" }
  }
}
