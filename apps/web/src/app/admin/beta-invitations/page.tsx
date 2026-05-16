import { BetaInvitationsClient } from "./beta-invitations-client"
import type { BetaInvitation } from "./types"

const INTERNAL_API_URL = process.env.INTERNAL_API_URL ?? "http://localhost:8080"
const ADMIN_TOKEN = process.env.ADMIN_TOKEN ?? ""

async function fetchInvitations(): Promise<BetaInvitation[]> {
  try {
    const res = await fetch(`${INTERNAL_API_URL}/v1/admin/beta-invitations`, {
      headers: { "X-Admin-Token": ADMIN_TOKEN },
      cache: "no-store",
    })
    if (!res.ok) return []
    const j = await res.json()
    return j.data ?? []
  } catch {
    return []
  }
}

export default async function BetaInvitationsPage() {
  const invitations = await fetchInvitations()
  return <BetaInvitationsClient initialInvitations={invitations} />
}
