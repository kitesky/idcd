// Mirror of apps/api/internal/handler/admin_status_incidents.go::adminIncident.
// Keep field names in lockstep with the JSON tags on the Go side.

export type IncidentSeverity = "degradation" | "partial_outage" | "outage" | "maintenance"

export interface AdminIncident {
  id: number
  service_key: string
  started_at: string
  ended_at?: string | null
  severity: IncidentSeverity
  title: string
  summary?: string
  related?: string[]
  created_at: string
  updated_at: string
}

export interface CreateIncidentInput {
  service_key: string
  started_at: string
  ended_at?: string | null
  severity: IncidentSeverity
  title: string
  summary?: string
  related?: string[]
}
