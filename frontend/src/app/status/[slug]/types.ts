export type ServiceStatus = "operational" | "degraded" | "outage" | "maintenance"

export interface MonitorHistory {
  date: string
  status: ServiceStatus
  uptime: number
}

export interface StatusMonitor {
  id: string
  name: string
  status: ServiceStatus
  uptime_percent: number
  history: MonitorHistory[]
}

export interface StatusGroup {
  id: string
  name: string
  monitors: StatusMonitor[]
}

export interface StatusPageData {
  slug: string
  title: string
  description: string
  branding: boolean
  overall_status: ServiceStatus
  groups: StatusGroup[]
  events: unknown[]
}
