export type MonitorType =
  | "http"
  | "https"
  | "ping"
  | "tcp"
  | "dns"
  | "ssl_expiry"
  | "domain_expiry"
  | "icp_change"
  | "keyword"

export type MonitorStatus = "UP" | "DOWN" | "PAUSED" | "degraded"

export interface Monitor {
  id: string
  name: string
  type: MonitorType
  target: string
  status: MonitorStatus
  uptimePercent: number
  lastCheckedAt: string
  intervalSeconds: number
  concurrentNodes: number
  // Optional advanced config
  assertStatusCode?: number
  keywordMatch?: string
  timeoutMs?: number
  packetLossThreshold?: number
  port?: number
  expectedIp?: string
  sslExpiryDays?: number // days until SSL expires
}

export interface CheckResult {
  checkedAt: string
  nodeId: string
  nodeRegion: string
  status: MonitorStatus
  latencyMs: number
  error?: string
}

export const MOCK_MONITORS: Monitor[] = [
  {
    id: "mon-001",
    name: "idcd.com 主站",
    type: "http",
    target: "https://idcd.com",
    status: "UP",
    uptimePercent: 99.8,
    lastCheckedAt: new Date(Date.now() - 60_000).toISOString(),
    intervalSeconds: 60,
    concurrentNodes: 5,
    assertStatusCode: 200,
  },
  {
    id: "mon-002",
    name: "API 网关健康检查",
    type: "http",
    target: "https://api.idcd.com/health",
    status: "DOWN",
    uptimePercent: 94.2,
    lastCheckedAt: new Date(Date.now() - 120_000).toISOString(),
    intervalSeconds: 60,
    concurrentNodes: 3,
    assertStatusCode: 200,
  },
  {
    id: "mon-003",
    name: "香港节点 Ping",
    type: "ping",
    target: "hk-ct-pccw-01.idcd.com",
    status: "UP",
    uptimePercent: 99.5,
    lastCheckedAt: new Date(Date.now() - 30_000).toISOString(),
    intervalSeconds: 300,
    concurrentNodes: 1,
    timeoutMs: 5000,
    packetLossThreshold: 10,
  },
  {
    id: "mon-004",
    name: "日本东京 Ping",
    type: "ping",
    target: "jp-tok-ntt-01.idcd.com",
    status: "UP",
    uptimePercent: 99.9,
    lastCheckedAt: new Date(Date.now() - 45_000).toISOString(),
    intervalSeconds: 300,
    concurrentNodes: 1,
    timeoutMs: 5000,
  },
  {
    id: "mon-005",
    name: "idcd.com SSL 证书",
    type: "ssl_expiry",
    target: "idcd.com",
    status: "UP",
    uptimePercent: 100,
    lastCheckedAt: new Date(Date.now() - 3600_000).toISOString(),
    intervalSeconds: 1800,
    concurrentNodes: 1,
    sslExpiryDays: 12,
  },
  {
    id: "mon-006",
    name: "DNS 解析检查",
    type: "dns",
    target: "idcd.com",
    status: "degraded",
    uptimePercent: 97.1,
    lastCheckedAt: new Date(Date.now() - 90_000).toISOString(),
    intervalSeconds: 300,
    concurrentNodes: 3,
    expectedIp: "104.21.0.1",
  },
]

// Generate 48 mock check blocks for the trend view
export function generateMockTrendBlocks(monitorId: string): Array<{
  index: number
  status: "UP" | "DOWN"
  checkedAt: string
  latencyMs: number
}> {
  const blocks = []
  const now = Date.now()
  // mon-002 (DOWN) has some failures; mon-006 (degraded) has occasional failures
  for (let i = 47; i >= 0; i--) {
    const ts = new Date(now - i * 30 * 60_000).toISOString()
    let status: "UP" | "DOWN" = "UP"
    if (monitorId === "mon-002") {
      // mostly DOWN with some UP at start
      status = i < 10 ? "DOWN" : i < 15 ? "DOWN" : "UP"
    } else if (monitorId === "mon-006") {
      // degraded: occasional DOWN
      status = i % 8 === 0 ? "DOWN" : "UP"
    }
    blocks.push({
      index: 48 - i,
      status,
      checkedAt: ts,
      latencyMs: status === "UP" ? 80 + Math.floor(Math.random() * 120) : 0,
    })
  }
  return blocks
}

// Generate mock check results for detail table
export function generateMockCheckResults(monitorId: string): CheckResult[] {
  const nodes = [
    { id: "cn-bj-ct-01", region: "北京" },
    { id: "hk-ct-pccw-01", region: "香港" },
    { id: "jp-tok-ntt-01", region: "东京" },
    { id: "sg-sin-sgt-01", region: "新加坡" },
    { id: "us-iad-aws-01", region: "弗吉尼亚" },
  ]
  const now = Date.now()
  const results: CheckResult[] = []
  for (let i = 0; i < 10; i++) {
    const node = nodes[i % nodes.length]
    const isDown = monitorId === "mon-002" && i < 4
    results.push({
      checkedAt: new Date(now - i * 60_000).toISOString(),
      nodeId: node.id,
      nodeRegion: node.region,
      status: isDown ? "DOWN" : "UP",
      latencyMs: isDown ? 0 : 60 + Math.floor(Math.random() * 200),
      error: isDown ? "Connection refused" : undefined,
    })
  }
  return results
}

export const TYPE_LABELS: Record<MonitorType, string> = {
  http: "HTTP",
  https: "HTTPS",
  ping: "Ping",
  tcp: "TCP",
  dns: "DNS",
  ssl_expiry: "SSL到期",
  domain_expiry: "域名到期",
  icp_change: "ICP变更",
  keyword: "关键字",
}

export const MONITOR_TYPES: MonitorType[] = [
  "http",
  "https",
  "ping",
  "tcp",
  "dns",
  "ssl_expiry",
  "domain_expiry",
  "icp_change",
  "keyword",
]
