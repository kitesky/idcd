// Static mock data for the /leaderboard page
// All latency values are in milliseconds, sourced from realistic CDN benchmark references

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

// CDN Response Speed Data
export const CDN_DATA: CdnEntry[] = [
  {
    rank: 1,
    name: "Cloudflare CDN",
    shortName: "Cloudflare",
    globalP50: 18,
    chinaP50: 42,
    overseasP50: 12,
    trend: [22, 21, 20, 19, 18, 18, 18],
    change: -4,
  },
  {
    rank: 2,
    name: "Fastly",
    shortName: "Fastly",
    globalP50: 24,
    chinaP50: 89,
    overseasP50: 16,
    trend: [28, 27, 25, 24, 24, 25, 24],
    change: -4,
  },
  {
    rank: 3,
    name: "Akamai",
    shortName: "Akamai",
    globalP50: 31,
    chinaP50: 95,
    overseasP50: 22,
    trend: [34, 33, 32, 31, 31, 31, 31],
    change: -3,
  },
  {
    rank: 4,
    name: "AWS CloudFront",
    shortName: "CloudFront",
    globalP50: 38,
    chinaP50: 112,
    overseasP50: 28,
    trend: [40, 39, 39, 38, 37, 38, 38],
    change: -2,
  },
  {
    rank: 5,
    name: "腾讯云 CDN",
    shortName: "腾讯云",
    globalP50: 45,
    chinaP50: 28,
    overseasP50: 68,
    trend: [48, 47, 46, 45, 45, 44, 45],
    change: -3,
  },
  {
    rank: 6,
    name: "阿里云 CDN",
    shortName: "阿里云",
    globalP50: 48,
    chinaP50: 31,
    overseasP50: 72,
    trend: [50, 50, 49, 48, 48, 48, 48],
    change: -2,
  },
  {
    rank: 7,
    name: "ByteDance CDN",
    shortName: "ByteDance",
    globalP50: 52,
    chinaP50: 35,
    overseasP50: 78,
    trend: [55, 54, 53, 52, 52, 53, 52],
    change: -3,
  },
  {
    rank: 8,
    name: "七牛云",
    shortName: "七牛云",
    globalP50: 68,
    chinaP50: 40,
    overseasP50: 105,
    trend: [72, 71, 70, 69, 68, 68, 68],
    change: -4,
  },
  {
    rank: 9,
    name: "又拍云",
    shortName: "又拍云",
    globalP50: 74,
    chinaP50: 44,
    overseasP50: 118,
    trend: [74, 74, 75, 74, 74, 75, 74],
    change: 0,
  },
  {
    rank: 10,
    name: "Cloudflare R2",
    shortName: "CF R2",
    globalP50: 88,
    chinaP50: 156,
    overseasP50: 35,
    trend: [92, 91, 90, 89, 88, 88, 88],
    change: -4,
  },
]

// Global Node Latency by Region
export const REGION_LATENCY_DATA: RegionLatency[] = [
  {
    continent: "亚洲",
    continentEn: "Asia",
    countries: [
      { name: "香港", nameEn: "Hong Kong", p50: 8, p95: 22, nodeCount: 3 },
      { name: "日本", nameEn: "Japan", p50: 14, p95: 38, nodeCount: 3 },
      { name: "新加坡", nameEn: "Singapore", p50: 18, p95: 45, nodeCount: 2 },
      { name: "韩国", nameEn: "South Korea", p50: 22, p95: 55, nodeCount: 2 },
      { name: "台湾", nameEn: "Taiwan", p50: 12, p95: 32, nodeCount: 1 },
    ],
  },
  {
    continent: "欧洲",
    continentEn: "Europe",
    countries: [
      { name: "德国", nameEn: "Germany", p50: 28, p95: 68, nodeCount: 2 },
      { name: "英国", nameEn: "United Kingdom", p50: 32, p95: 75, nodeCount: 2 },
      { name: "法国", nameEn: "France", p50: 35, p95: 82, nodeCount: 1 },
      { name: "荷兰", nameEn: "Netherlands", p50: 30, p95: 72, nodeCount: 1 },
      { name: "波兰", nameEn: "Poland", p50: 42, p95: 98, nodeCount: 1 },
    ],
  },
  {
    continent: "北美",
    continentEn: "North America",
    countries: [
      { name: "美国（弗吉尼亚）", nameEn: "US East", p50: 42, p95: 98, nodeCount: 2 },
      { name: "美国（洛杉矶）", nameEn: "US West", p50: 48, p95: 110, nodeCount: 2 },
      { name: "加拿大", nameEn: "Canada", p50: 52, p95: 118, nodeCount: 1 },
      { name: "墨西哥", nameEn: "Mexico", p50: 88, p95: 195, nodeCount: 1 },
      { name: "美国（芝加哥）", nameEn: "US Central", p50: 45, p95: 105, nodeCount: 1 },
    ],
  },
  {
    continent: "大洋洲",
    continentEn: "Oceania",
    countries: [
      { name: "澳大利亚（悉尼）", nameEn: "Australia (Sydney)", p50: 95, p95: 215, nodeCount: 1 },
      { name: "澳大利亚（墨尔本）", nameEn: "Australia (Melbourne)", p50: 102, p95: 228, nodeCount: 1 },
      { name: "新西兰", nameEn: "New Zealand", p50: 128, p95: 285, nodeCount: 1 },
      { name: "斐济", nameEn: "Fiji", p50: 185, p95: 380, nodeCount: 1 },
      { name: "巴布亚新几内亚", nameEn: "Papua New Guinea", p50: 210, p95: 430, nodeCount: 1 },
    ],
  },
  {
    continent: "南美",
    continentEn: "South America",
    countries: [
      { name: "巴西（圣保罗）", nameEn: "Brazil (São Paulo)", p50: 145, p95: 320, nodeCount: 1 },
      { name: "阿根廷", nameEn: "Argentina", p50: 168, p95: 365, nodeCount: 1 },
      { name: "智利", nameEn: "Chile", p50: 175, p95: 385, nodeCount: 1 },
      { name: "哥伦比亚", nameEn: "Colombia", p50: 182, p95: 398, nodeCount: 1 },
      { name: "秘鲁", nameEn: "Peru", p50: 190, p95: 415, nodeCount: 1 },
    ],
  },
  {
    continent: "非洲",
    continentEn: "Africa",
    countries: [
      { name: "南非（约翰内斯堡）", nameEn: "South Africa", p50: 210, p95: 445, nodeCount: 1 },
      { name: "尼日利亚", nameEn: "Nigeria", p50: 248, p95: 520, nodeCount: 1 },
      { name: "肯尼亚", nameEn: "Kenya", p50: 235, p95: 495, nodeCount: 1 },
      { name: "埃及", nameEn: "Egypt", p50: 192, p95: 410, nodeCount: 1 },
      { name: "摩洛哥", nameEn: "Morocco", p50: 188, p95: 402, nodeCount: 1 },
    ],
  },
]

// ISP Availability Stats
export const ISP_AVAILABILITY_DATA: IspAvailability[] = [
  { rank: 1, isp: "Cloudflare", region: "全球", availability30d: 99.98, sla: 99.9, datacenterCount: 12 },
  { rank: 2, isp: "AWS / Amazon", region: "全球", availability30d: 99.97, sla: 99.9, datacenterCount: 8 },
  { rank: 3, isp: "Fastly", region: "全球", availability30d: 99.95, sla: 99.9, datacenterCount: 6 },
  { rank: 4, isp: "NTT Communications", region: "亚太", availability30d: 99.94, sla: 99.5, datacenterCount: 5 },
  { rank: 5, isp: "腾讯云", region: "中国大陆", availability30d: 99.93, sla: 99.5, datacenterCount: 7 },
  { rank: 6, isp: "阿里云", region: "中国大陆", availability30d: 99.92, sla: 99.5, datacenterCount: 8 },
  { rank: 7, isp: "中国电信", region: "中国大陆", availability30d: 99.88, sla: 99.0, datacenterCount: 15 },
  { rank: 8, isp: "中国联通", region: "中国大陆", availability30d: 99.85, sla: 99.0, datacenterCount: 12 },
  { rank: 9, isp: "Deutsche Telekom", region: "欧洲", availability30d: 99.84, sla: 99.0, datacenterCount: 4 },
  { rank: 10, isp: "中国移动", region: "中国大陆", availability30d: 99.82, sla: 99.0, datacenterCount: 10 },
  { rank: 11, isp: "Hetzner", region: "欧洲", availability30d: 99.78, sla: 99.0, datacenterCount: 3 },
  { rank: 12, isp: "Lumen (CenturyLink)", region: "北美", availability30d: 99.71, sla: 99.0, datacenterCount: 3 },
]

// Helper: total node count for subtitle
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
