export interface NodeLocation {
  country: string
  city: string
  asn: string
  isp: string
}

export interface LatencyDistribution {
  p50: number
  p90: number
  p95: number
  p99: number
  min: number
  max: number
}

export interface HealthTrendPoint {
  hour: string
  success_rate: number
  avg_latency: number
}

export interface NodeDiagnosticsData {
  node_id: string
  name: string
  location: NodeLocation
  status: string
  uptime_24h: number
  checks_24h: number
  latency_distribution: LatencyDistribution
  health_trend: HealthTrendPoint[]
  last_seen: string
}

function buildHealthTrend(): HealthTrendPoint[] {
  const now = new Date()
  now.setMinutes(0, 0, 0)
  return Array.from({ length: 24 }, (_, i) => {
    const h = new Date(now.getTime() - i * 3600 * 1000)
    const rate = i === 3 ? 87.5 : i === 7 ? 94.2 : 100.0
    const latency = 30 + Math.sin(i * 0.4) * 8 + (i === 3 ? 45 : 0)
    return {
      hour: h.toISOString(),
      success_rate: rate,
      avg_latency: parseFloat(latency.toFixed(1)),
    }
  })
}

const MOCK_DIAGNOSTICS_MAP: Record<string, NodeDiagnosticsData> = {
  "jp-tok-ntt-01": {
    node_id: "jp-tok-ntt-01",
    name: "Tokyo JP — NTT",
    location: { country: "JP", city: "Tokyo", asn: "AS2914", isp: "NTT" },
    status: "active",
    uptime_24h: 99.97,
    checks_24h: 1440,
    latency_distribution: { p50: 32.5, p90: 45.2, p95: 58.1, p99: 124.7, min: 18.2, max: 312.5 },
    health_trend: buildHealthTrend(),
    last_seen: new Date().toISOString(),
  },
  "cn-bj-ct-01": {
    node_id: "cn-bj-ct-01",
    name: "Beijing CN — 中国电信",
    location: { country: "CN", city: "Beijing", asn: "AS4134", isp: "China Telecom" },
    status: "active",
    uptime_24h: 99.85,
    checks_24h: 1440,
    latency_distribution: { p50: 12.3, p90: 21.5, p95: 28.0, p99: 65.4, min: 5.1, max: 198.2 },
    health_trend: buildHealthTrend(),
    last_seen: new Date().toISOString(),
  },
  "us-lax-cf-01": {
    node_id: "us-lax-cf-01",
    name: "Los Angeles US — Cloudflare",
    location: { country: "US", city: "Los Angeles", asn: "AS13335", isp: "Cloudflare" },
    status: "active",
    uptime_24h: 100.0,
    checks_24h: 1440,
    latency_distribution: { p50: 8.1, p90: 15.4, p95: 21.2, p99: 54.8, min: 3.2, max: 142.6 },
    health_trend: buildHealthTrend(),
    last_seen: new Date().toISOString(),
  },
}

function buildDefaultDiagnostics(id: string): NodeDiagnosticsData {
  return {
    node_id: id,
    name: id,
    location: { country: "Unknown", city: "Unknown", asn: "AS0", isp: "Unknown" },
    status: "unknown",
    uptime_24h: 99.9,
    checks_24h: 1440,
    latency_distribution: { p50: 35.0, p90: 55.0, p95: 72.0, p99: 140.0, min: 20.0, max: 350.0 },
    health_trend: buildHealthTrend(),
    last_seen: new Date().toISOString(),
  }
}

export function getMockDiagnostics(id: string): NodeDiagnosticsData {
  return MOCK_DIAGNOSTICS_MAP[id] ?? buildDefaultDiagnostics(id)
}
