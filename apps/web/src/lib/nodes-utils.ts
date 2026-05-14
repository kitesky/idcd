export interface NodeEntry {
  id: string
  asn: string
  carrier: string
  region: string
  exitIp: string
  status: "online" | "offline" | "degraded"
  country: string
}

export interface StatusInfo {
  label: string
  variant: "success" | "destructive" | "warning" | "secondary"
}

export function mapStatus(status: string): StatusInfo {
  switch (status) {
    case "online":
      return { label: "在线", variant: "success" }
    case "offline":
      return { label: "离线", variant: "destructive" }
    case "degraded":
      return { label: "降级", variant: "warning" }
    default:
      return { label: status, variant: "secondary" }
  }
}

export function formatIP(ip: string): string {
  if (!ip) return ""
  // Pure IPv6 (contains colon but not already bracketed)
  if (ip.includes(":") && !ip.startsWith("[")) {
    return `[${ip}]`
  }
  return ip
}

export interface NodeStats {
  total: number
  online: number
  countries: number
  carriers: number
}

export function aggregateStats(nodes: NodeEntry[]): NodeStats {
  return {
    total: nodes.length,
    online: nodes.filter((n) => n.status === "online").length,
    countries: new Set(nodes.map((n) => n.country)).size,
    carriers: new Set(nodes.map((n) => n.carrier)).size,
  }
}

export interface NodeFilters {
  country?: string
  carrier?: string
  status?: string
  search?: string
}

export function filterNodes(nodes: NodeEntry[], filters: NodeFilters): NodeEntry[] {
  return nodes.filter((node) => {
    if (filters.country && filters.country !== "all" && node.country !== filters.country)
      return false
    if (filters.carrier && filters.carrier !== "all" && node.carrier !== filters.carrier)
      return false
    if (filters.status && filters.status !== "all" && node.status !== filters.status)
      return false
    if (filters.search) {
      const q = filters.search.toLowerCase()
      const hit =
        node.id.toLowerCase().includes(q) ||
        node.asn.toLowerCase().includes(q) ||
        node.carrier.toLowerCase().includes(q) ||
        node.region.toLowerCase().includes(q) ||
        node.exitIp.toLowerCase().includes(q) ||
        node.country.toLowerCase().includes(q)
      if (!hit) return false
    }
    return true
  })
}
