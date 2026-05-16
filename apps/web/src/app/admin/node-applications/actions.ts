"use server"

export async function reviewApplicationAction(
  id: string,
  action: "approve" | "reject",
  note: string,
): Promise<string | null> {
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
      return (j as { error?: { message?: string } })?.error?.message ?? "操作失败"
    }
    return null
  } catch {
    return "网络错误，请重试"
  }
}
