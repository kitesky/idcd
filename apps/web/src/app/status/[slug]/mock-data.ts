export type ServiceStatus = "operational" | "degraded" | "outage" | "maintenance"

export interface Monitor { id: string; name: string; status: ServiceStatus; uptimePercent: number }
export interface ServiceGroup { id: string; name: string; monitors: Monitor[] }
export interface StatusEvent {
  id: string; title: string; description: string
  status: "resolved" | "investigating" | "identified" | "monitoring"
  createdAt: string; resolvedAt?: string; affectedServices: string[]
}
export interface StatusPageData {
  slug: string; title: string; overallStatus: ServiceStatus
  groups: ServiceGroup[]; events: StatusEvent[]; showBranding: boolean
}

export const MOCK_STATUS_PAGES: Record<string, StatusPageData> = {
  demo: {
    slug: "demo", title: "acme.com 服务状态", overallStatus: "operational", showBranding: true,
    groups: [
      { id: "group-core", name: "核心服务", monitors: [
        { id: "mon-web",  name: "官网 (acme.com)",  status: "operational", uptimePercent: 99.98 },
        { id: "mon-api",  name: "API 服务",          status: "operational", uptimePercent: 99.95 },
        { id: "mon-auth", name: "认证服务",           status: "operational", uptimePercent: 100 },
      ]},
      { id: "group-data", name: "数据服务", monitors: [
        { id: "mon-db",    name: "数据库集群",        status: "operational", uptimePercent: 99.99 },
        { id: "mon-cache", name: "缓存服务 (Redis)",  status: "operational", uptimePercent: 99.9 },
      ]},
      { id: "group-infra", name: "基础设施", monitors: [
        { id: "mon-cdn",   name: "CDN 分发",  status: "operational", uptimePercent: 99.97 },
        { id: "mon-dns",   name: "DNS 解析",  status: "operational", uptimePercent: 100 },
        { id: "mon-email", name: "邮件通知",  status: "operational", uptimePercent: 99.8 },
      ]},
    ],
    events: [{
      id: "evt-001", title: "API 响应延迟升高",
      description: "2026-05-10 14:23 UTC，API 服务响应时间异常升高，经排查为上游数据库连接池耗尽，已扩容解决，15:47 UTC 恢复正常。",
      status: "resolved", createdAt: "2026-05-10T14:23:00Z", resolvedAt: "2026-05-10T15:47:00Z",
      affectedServices: ["API 服务", "认证服务"],
    }],
  },
}

export function generateUptimeHistory(seedPercent: number): Array<{ date: string; status: ServiceStatus; uptime: number }> {
  const operationalRate = seedPercent / 100
  const failureRate = 1 - operationalRate
  const degradedRate = failureRate * 0.85
  const result = []
  const now = new Date()
  for (let i = 89; i >= 0; i--) {
    const d = new Date(now); d.setDate(d.getDate() - i)
    const rand = Math.random()
    let status: ServiceStatus, uptime: number
    if (rand >= operationalRate + degradedRate) { status = "outage"; uptime = 60 + Math.random() * 20 }
    else if (rand >= operationalRate)            { status = "degraded"; uptime = 85 + Math.random() * 10 }
    else                                          { status = "operational"; uptime = 99 + Math.random() }
    result.push({ date: d.toISOString().slice(0, 10), status, uptime })
  }
  return result
}
