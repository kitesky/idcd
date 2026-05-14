export type AlertStatus = "firing" | "resolved" | "acknowledged"
export type ChannelType = "email" | "webhook" | "wecom" | "dingtalk" | "feishu"

export interface AlertEvent {
  id: string
  monitorName: string
  status: AlertStatus
  startedAt: string
  resolvedAt?: string
  acknowledgedAt?: string
}

export interface AlertChannel {
  id: string
  name: string
  type: ChannelType
  config: string // email address or webhook URL
  verified: boolean
}

export interface AlertPolicy {
  id: string
  name: string
  monitorName: string
  channelIds: string[]
  delayMinutes: number
  muteFrom?: string // HH:mm
  muteTo?: string   // HH:mm
  enabled: boolean
}

export interface AlertNotification {
  id: string
  alert_event_id: string
  status: "sent" | "failed" | "pending"
  sent_at: string | null
  error: string | null
}

export type SilenceStatus = "active" | "upcoming" | "expired"

export interface AlertSilence {
  id: string
  monitorId?: string
  monitorName?: string
  reason: string
  startsAt: string
  endsAt: string
  status: SilenceStatus
}

export const CHANNEL_TYPE_LABELS: Record<ChannelType, string> = {
  email: "邮件",
  webhook: "Webhook",
  wecom: "企业微信",
  dingtalk: "钉钉",
  feishu: "飞书",
}

export const CHANNEL_TYPES: ChannelType[] = [
  "email",
  "webhook",
  "wecom",
  "dingtalk",
  "feishu",
]

/** Format duration from start time to now (or to end time) */
export function formatDuration(startedAt: string, endedAt?: string): string {
  const start = new Date(startedAt).getTime()
  const end = endedAt ? new Date(endedAt).getTime() : Date.now()
  const diffMs = end - start
  const mins = Math.floor(diffMs / 60_000)
  if (mins < 60) return `${mins} 分钟`
  const hours = Math.floor(mins / 60)
  const remainMins = mins % 60
  if (remainMins === 0) return `${hours} 小时`
  return `${hours} 小时 ${remainMins} 分钟`
}

/** Truncate a URL/config string for display */
export function truncateConfig(config: string, maxLen = 32): string {
  if (config.length <= maxLen) return config
  return config.slice(0, maxLen) + "…"
}

const BASE = Date.now()

export const MOCK_ALERT_EVENTS: AlertEvent[] = [
  {
    id: "evt-001",
    monitorName: "API 网关健康检查",
    status: "firing",
    startedAt: new Date(BASE - 15 * 60_000).toISOString(),
  },
  {
    id: "evt-002",
    monitorName: "idcd.com SSL 证书",
    status: "firing",
    startedAt: new Date(BASE - 3 * 60_000).toISOString(),
  },
  {
    id: "evt-003",
    monitorName: "DNS 解析检查",
    status: "acknowledged",
    startedAt: new Date(BASE - 2 * 3600_000).toISOString(),
    acknowledgedAt: new Date(BASE - 90 * 60_000).toISOString(),
  },
  {
    id: "evt-004",
    monitorName: "香港节点 Ping",
    status: "resolved",
    startedAt: new Date(BASE - 5 * 3600_000).toISOString(),
    resolvedAt: new Date(BASE - 4 * 3600_000).toISOString(),
  },
  {
    id: "evt-005",
    monitorName: "idcd.com 主站",
    status: "resolved",
    startedAt: new Date(BASE - 25 * 3600_000).toISOString(),
    resolvedAt: new Date(BASE - 24 * 3600_000).toISOString(),
  },
]

export const MOCK_ALERT_CHANNELS: AlertChannel[] = [
  {
    id: "ch-001",
    name: "运维邮件组",
    type: "email",
    config: "ops@idcd.com",
    verified: true,
  },
  {
    id: "ch-002",
    name: "运维企业微信群",
    type: "wecom",
    config: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=abc…",
    verified: true,
  },
  {
    id: "ch-003",
    name: "研发钉钉告警群",
    type: "dingtalk",
    config: "https://oapi.dingtalk.com/robot/send?access_token=xyz…",
    verified: false,
  },
]

export const MOCK_ALERT_POLICIES: AlertPolicy[] = [
  {
    id: "pol-001",
    name: "关键服务告警策略",
    monitorName: "API 网关健康检查",
    channelIds: ["ch-001", "ch-002"],
    delayMinutes: 5,
    muteFrom: "00:00",
    muteTo: "08:00",
    enabled: true,
  },
  {
    id: "pol-002",
    name: "SSL 到期提醒",
    monitorName: "idcd.com SSL 证书",
    channelIds: ["ch-001"],
    delayMinutes: 0,
    enabled: false,
  },
]

export const MOCK_NOTIFICATIONS: Record<string, AlertNotification[]> = {
  "ch-001": [
    { id: "n-001", alert_event_id: "ae-001", status: "sent", sent_at: new Date(BASE - 15 * 60_000).toISOString(), error: null },
    { id: "n-002", alert_event_id: "ae-002", status: "failed", sent_at: new Date(BASE - 3 * 3600_000).toISOString(), error: "connection timeout" },
    { id: "n-003", alert_event_id: "ae-003", status: "sent", sent_at: new Date(BASE - 5 * 3600_000).toISOString(), error: null },
  ],
  "ch-002": [
    { id: "n-004", alert_event_id: "ae-001", status: "sent", sent_at: new Date(BASE - 15 * 60_000).toISOString(), error: null },
    { id: "n-005", alert_event_id: "ae-002", status: "pending", sent_at: null, error: null },
  ],
  "ch-003": [],
}

export const MOCK_MONITOR_NAMES = [
  "idcd.com 主站",
  "API 网关健康检查",
  "香港节点 Ping",
  "日本东京 Ping",
  "idcd.com SSL 证书",
  "DNS 解析检查",
]

const NOW = Date.now()

export const MOCK_ALERT_SILENCES: AlertSilence[] = [
  {
    id: "sil-001",
    monitorId: "mon-001",
    monitorName: "API 网关健康检查",
    reason: "计划维护窗口",
    startsAt: new Date(NOW - 10 * 60_000).toISOString(),
    endsAt: new Date(NOW + 50 * 60_000).toISOString(),
    status: "active",
  },
  {
    id: "sil-002",
    reason: "全局升级维护",
    startsAt: new Date(NOW + 2 * 3600_000).toISOString(),
    endsAt: new Date(NOW + 4 * 3600_000).toISOString(),
    status: "upcoming",
  },
]
