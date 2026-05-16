// Type definitions and helpers for the /leaderboard page

export interface CdnEntry {
  rank: number
  name: string
  shortName: string
  globalP50: number
  chinaP50: number
  overseasP50: number
  // 7-point sparkline values (relative trend, last 7 days)
  trend: number[]
  // positive = improved (lower latency), negative = degraded
  change: number
}

export interface RegionLatency {
  continent: string
  continentEn: string
  countries: {
    name: string
    nameEn: string
    p50: number
    p95: number
    nodeCount: number
  }[]
}

export interface IspAvailability {
  rank: number
  isp: string
  region: string
  availability30d: number
  sla: number
  datacenterCount: number
}

// Stub exports kept for backward compatibility with page.tsx and tests.
// Actual data is now fetched from the API at runtime.
export const CDN_DATA: CdnEntry[] = []
export const REGION_LATENCY_DATA: RegionLatency[] = []
export const ISP_AVAILABILITY_DATA: IspAvailability[] = []
export const NODE_COUNT = 24

// Helper: get current month label
export function getCurrentMonthLabel(): string {
  const now = new Date()
  return `${now.getFullYear()} 年 ${now.getMonth() + 1} 月`
}

// Helper: get latency badge variant
export function getLatencyVariant(ms: number): "success" | "warning" | "destructive" {
  if (ms < 50) return "success"
  if (ms <= 200) return "warning"
  return "destructive"
}
