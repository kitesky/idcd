export interface AdminNodeEntry {
  id: string
  region: string
  asn: string
  exitIp: string
  status: "online" | "offline" | "degraded"
  latencyMs: number
  lastHeartbeat: string // ISO string
}

export const MOCK_ADMIN_NODES: AdminNodeEntry[] = [
  { id: "cn-bj-ct-01", region: "中国大陆 / 北京", asn: "AS4134 中国电信", exitIp: "61.135.169.121", status: "online", latencyMs: 12, lastHeartbeat: "2026-05-14T10:00:00Z" },
  { id: "cn-sh-cu-01", region: "中国大陆 / 上海", asn: "AS4837 中国联通", exitIp: "202.106.0.20", status: "online", latencyMs: 9, lastHeartbeat: "2026-05-14T10:00:05Z" },
  { id: "cn-gz-cm-01", region: "中国大陆 / 广州", asn: "AS9808 中国移动", exitIp: "211.136.112.200", status: "online", latencyMs: 14, lastHeartbeat: "2026-05-14T10:00:08Z" },
  { id: "cn-cd-ct-01", region: "中国大陆 / 成都", asn: "AS4134 中国电信", exitIp: "61.139.2.69", status: "online", latencyMs: 22, lastHeartbeat: "2026-05-14T10:00:10Z" },
  { id: "cn-wh-cu-01", region: "中国大陆 / 武汉", asn: "AS4837 中国联通", exitIp: "60.29.251.109", status: "degraded", latencyMs: 180, lastHeartbeat: "2026-05-14T09:55:00Z" },
  { id: "hk-pccw-01", region: "香港", asn: "AS9269 PCCW", exitIp: "203.160.128.1", status: "online", latencyMs: 4, lastHeartbeat: "2026-05-14T10:00:02Z" },
  { id: "hk-hkt-01", region: "香港", asn: "AS4760 HKT", exitIp: "210.0.0.1", status: "online", latencyMs: 5, lastHeartbeat: "2026-05-14T10:00:03Z" },
  { id: "hk-ckh-01", region: "香港", asn: "AS10316 HGC", exitIp: "192.168.200.1", status: "offline", latencyMs: 0, lastHeartbeat: "2026-05-14T08:00:00Z" },
  { id: "jp-tok-ntt-01", region: "日本 / 东京", asn: "AS2914 NTT", exitIp: "129.250.0.1", status: "online", latencyMs: 52, lastHeartbeat: "2026-05-14T10:00:06Z" },
  { id: "sg-sin-sgt-01", region: "新加坡", asn: "AS7473 Singtel", exitIp: "165.21.0.1", status: "online", latencyMs: 35, lastHeartbeat: "2026-05-14T10:00:04Z" },
  { id: "us-iad-aws-01", region: "美国 / 弗吉尼亚", asn: "AS16509 AWS", exitIp: "52.94.0.1", status: "online", latencyMs: 180, lastHeartbeat: "2026-05-14T10:00:07Z" },
  { id: "us-chi-l3-01", region: "美国 / 芝加哥", asn: "AS3356 Lumen", exitIp: "4.2.2.1", status: "degraded", latencyMs: 320, lastHeartbeat: "2026-05-14T09:45:00Z" },
  { id: "de-fra-dt-01", region: "德国 / 法兰克福", asn: "AS3320 Deutsche Telekom", exitIp: "80.150.0.1", status: "online", latencyMs: 210, lastHeartbeat: "2026-05-14T10:00:09Z" },
  { id: "gb-lon-bt-01", region: "英国 / 伦敦", asn: "AS2856 BT", exitIp: "109.159.0.1", status: "online", latencyMs: 195, lastHeartbeat: "2026-05-14T10:00:11Z" },
  { id: "gb-man-vm-01", region: "英国 / 曼彻斯特", asn: "AS5089 Virgin Media", exitIp: "82.25.0.1", status: "offline", latencyMs: 0, lastHeartbeat: "2026-05-14T07:30:00Z" },
]

export function computeNodeStats(nodes: AdminNodeEntry[]) {
  const online = nodes.filter((n) => n.status === "online").length
  const offline = nodes.filter((n) => n.status === "offline").length
  const online_latencies = nodes
    .filter((n) => n.status === "online" && n.latencyMs > 0)
    .map((n) => n.latencyMs)
  const avgLatency =
    online_latencies.length > 0
      ? Math.round(
          online_latencies.reduce((a, b) => a + b, 0) / online_latencies.length
        )
      : 0
  // Mock: today's check count
  const checksToday = nodes.length * 144 // every 10 minutes

  return { online, offline, avgLatency, checksToday }
}
