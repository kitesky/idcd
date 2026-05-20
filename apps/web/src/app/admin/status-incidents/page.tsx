import { StatusIncidentsClient } from "./status-incidents-client"
import type { AdminIncident } from "./types"

const INTERNAL_API_URL = process.env.INTERNAL_API_URL ?? "http://localhost:8080"
const ADMIN_TOKEN = process.env.ADMIN_TOKEN ?? ""

async function fetchIncidents(): Promise<AdminIncident[]> {
  try {
    const res = await fetch(`${INTERNAL_API_URL}/v1/admin/status-incidents`, {
      headers: { Authorization: `Bearer ${ADMIN_TOKEN}` },
      cache: "no-store",
    })
    if (!res.ok) return []
    const j = await res.json()
    return j.incidents ?? []
  } catch {
    return []
  }
}

export default async function StatusIncidentsPage() {
  const incidents = await fetchIncidents()
  return <StatusIncidentsClient initialIncidents={incidents} />
}
