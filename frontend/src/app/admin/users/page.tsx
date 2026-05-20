import { UsersClient } from "./users-client"

interface User {
  id: string; email: string; status: string; plan: string
  monitor_count: number; created_at: string
}

interface UsersResp { users: User[]; total: number; page: number; per_page: number }

const INTERNAL_API_URL = process.env.INTERNAL_API_URL ?? "http://localhost:8080"
const ADMIN_TOKEN = process.env.ADMIN_TOKEN ?? ""

async function fetchUsers(page: number, q: string): Promise<UsersResp> {
  try {
    const params = new URLSearchParams({ page: String(page), per_page: "20" })
    if (q) params.set("q", q)
    const res = await fetch(`${INTERNAL_API_URL}/internal/admin/users?${params}`, {
      headers: { "X-Admin-Token": ADMIN_TOKEN },
      cache: "no-store",
    })
    if (!res.ok) return { users: [], total: 0, page, per_page: 20 }
    const j = await res.json()
    return j.data ?? { users: [], total: 0, page, per_page: 20 }
  } catch {
    return { users: [], total: 0, page, per_page: 20 }
  }
}

export default async function UsersPage({
  searchParams,
}: {
  searchParams: Promise<{ page?: string; q?: string }>
}) {
  const { page: pageParam, q: qParam } = await searchParams
  const page = Math.max(1, Number(pageParam ?? "1") || 1)
  const q = qParam ?? ""
  const data = await fetchUsers(page, q)
  return <UsersClient initialData={data} initialPage={page} initialQ={q} />
}
