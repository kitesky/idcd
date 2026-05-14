// API reference data for /docs/api page.
// All groups and endpoints are defined here as static data.

export type HttpMethod = "GET" | "POST" | "PATCH" | "PUT" | "DELETE"

export interface Parameter {
  name: string
  location: "query" | "path" | "body" | "header"
  required: boolean
  type: string
  description: string
  example?: string
}

export interface Endpoint {
  id: string
  method: HttpMethod
  path: string
  summary: string
  description: string
  auth: boolean
  parameters?: Parameter[]
  responseExample?: string
}

export interface EndpointGroup {
  id: string
  label: string
  description: string
  endpoints: Endpoint[]
}

export const API_GROUPS: EndpointGroup[] = [
  {
    id: "probe",
    label: "拨测",
    description: "多节点 HTTP / Ping / DNS / TCP / Traceroute 拨测",
    endpoints: [
      {
        id: "probe-http",
        method: "POST",
        path: "/v1/probe/http",
        summary: "HTTP 拨测",
        description: "向目标 URL 发起 HTTP/HTTPS 拨测，返回状态码、响应时间及 TLS 证书信息。",
        auth: false,
        parameters: [
          { name: "url", location: "body", required: true, type: "string", description: "目标 URL", example: "https://github.com" },
          { name: "method", location: "body", required: false, type: "string", description: "HTTP 方法，默认 GET", example: "GET" },
          { name: "timeout", location: "body", required: false, type: "integer", description: "超时秒数，最大 30", example: "10" },
          { name: "nodes", location: "body", required: false, type: "string[]", description: "指定节点 ID，留空自动选择" },
        ],
        responseExample: JSON.stringify({
          data: {
            task_id: "task_01J8XXXXX",
            status: "done",
            results: [{ node_id: "cn-bj", node_name: "北京", success: true, latency_ms: 42.5 }],
          },
          request_id: "req_01J8XXXXX",
        }, null, 2),
      },
      {
        id: "probe-ping",
        method: "POST",
        path: "/v1/probe/ping",
        summary: "ICMP Ping",
        description: "对目标主机发起 ICMP Ping，返回往返延迟和丢包率。",
        auth: false,
        parameters: [
          { name: "host", location: "body", required: true, type: "string", description: "目标主机名或 IP", example: "8.8.8.8" },
          { name: "count", location: "body", required: false, type: "integer", description: "发包数量，默认 4，最大 10" },
          { name: "timeout", location: "body", required: false, type: "integer", description: "超时秒数，默认 5" },
        ],
        responseExample: JSON.stringify({
          data: {
            task_id: "task_01J8XXXXX",
            status: "done",
            results: [{ node_id: "cn-sh", node_name: "上海", success: true, latency_ms: 28.3 }],
          },
          request_id: "req_01J8XXXXX",
        }, null, 2),
      },
      {
        id: "probe-dns",
        method: "POST",
        path: "/v1/probe/dns",
        summary: "DNS 解析拨测",
        description: "对指定域名进行 DNS 解析，返回解析结果和耗时。",
        auth: false,
        parameters: [
          { name: "domain", location: "body", required: true, type: "string", description: "目标域名", example: "github.com" },
          { name: "record_type", location: "body", required: false, type: "string", description: "记录类型 A/AAAA/MX/TXT/NS/CNAME/SOA", example: "A" },
          { name: "server", location: "body", required: false, type: "string", description: "自定义 DNS 服务器", example: "8.8.8.8" },
        ],
        responseExample: JSON.stringify({
          data: {
            task_id: "task_01J8XXXXX",
            status: "done",
            results: [{ node_id: "cn-bj", node_name: "北京", success: true, latency_ms: 15.2 }],
          },
          request_id: "req_01J8XXXXX",
        }, null, 2),
      },
      {
        id: "probe-tcp",
        method: "POST",
        path: "/v1/probe/tcp",
        summary: "TCPing 拨测",
        description: "对目标主机端口发起 TCP 连接测试，返回连接耗时。",
        auth: false,
        parameters: [
          { name: "host", location: "body", required: true, type: "string", description: "目标主机名或 IP", example: "github.com" },
          { name: "port", location: "body", required: true, type: "integer", description: "目标端口 (1-65535)", example: "443" },
          { name: "timeout", location: "body", required: false, type: "integer", description: "超时秒数，默认 5" },
        ],
        responseExample: JSON.stringify({
          data: {
            task_id: "task_01J8XXXXX",
            status: "done",
            results: [{ node_id: "cn-gz", node_name: "广州", success: true, latency_ms: 35.1 }],
          },
          request_id: "req_01J8XXXXX",
        }, null, 2),
      },
      {
        id: "probe-traceroute",
        method: "POST",
        path: "/v1/probe/traceroute",
        summary: "Traceroute 路由追踪",
        description: "追踪到目标主机的网络路径，返回每跳的 IP 和延迟。",
        auth: false,
        parameters: [
          { name: "host", location: "body", required: true, type: "string", description: "目标主机名或 IP", example: "github.com" },
          { name: "max_hops", location: "body", required: false, type: "integer", description: "最大跳数，默认 30" },
        ],
        responseExample: JSON.stringify({
          data: {
            task_id: "task_01J8XXXXX",
            status: "done",
            results: [{ node_id: "cn-bj", node_name: "北京", success: true, latency_ms: 180.0 }],
          },
          request_id: "req_01J8XXXXX",
        }, null, 2),
      },
    ],
  },
  {
    id: "info",
    label: "网络信息",
    description: "IP / WHOIS / SSL / DNS 记录 / ICP 备案查询",
    endpoints: [
      {
        id: "info-ip",
        method: "GET",
        path: "/v1/info/ip",
        summary: "IP 信息查询",
        description: "查询 IP 地址的归属地、ASN、运营商等信息。留空则查询请求方 IP。",
        auth: false,
        parameters: [
          { name: "ip", location: "query", required: false, type: "string", description: "要查询的 IP，留空则查询请求方 IP", example: "8.8.8.8" },
        ],
        responseExample: JSON.stringify({
          data: {
            ip: "8.8.8.8",
            country: "US",
            country_name: "美国",
            isp: "Google LLC",
            asn: "AS15169",
          },
          request_id: "req_01J8XXXXX",
        }, null, 2),
      },
      {
        id: "info-whois",
        method: "GET",
        path: "/v1/info/whois",
        summary: "WHOIS 查询",
        description: "查询域名或 IP 的 WHOIS 注册信息。",
        auth: false,
        parameters: [
          { name: "q", location: "query", required: true, type: "string", description: "查询目标（域名或 IP）", example: "github.com" },
        ],
        responseExample: JSON.stringify({
          data: {
            domain: "github.com",
            registrar: "MarkMonitor Inc.",
            expires_at: "2026-10-09",
            name_servers: ["ns1.github.com", "ns2.github.com"],
          },
          request_id: "req_01J8XXXXX",
        }, null, 2),
      },
      {
        id: "info-ssl",
        method: "GET",
        path: "/v1/info/ssl",
        summary: "SSL 证书查询",
        description: "查询域名的 SSL/TLS 证书信息，包括颁发机构、有效期、SANs 等。",
        auth: false,
        parameters: [
          { name: "domain", location: "query", required: true, type: "string", description: "要查询的域名", example: "github.com" },
        ],
        responseExample: JSON.stringify({
          data: {
            domain: "github.com",
            issuer: "DigiCert Inc",
            valid_to: "2026-03-14T00:00:00Z",
            days_remaining: 305,
            protocol: "TLS 1.3",
          },
          request_id: "req_01J8XXXXX",
        }, null, 2),
      },
      {
        id: "info-dns",
        method: "GET",
        path: "/v1/info/dns",
        summary: "DNS 记录查询",
        description: "查询域名的各类 DNS 记录（A/AAAA/MX/TXT/NS/CNAME/SOA）。",
        auth: false,
        parameters: [
          { name: "domain", location: "query", required: true, type: "string", description: "要查询的域名", example: "github.com" },
          { name: "type", location: "query", required: false, type: "string", description: "记录类型，留空返回所有类型", example: "A" },
        ],
        responseExample: JSON.stringify({
          data: {
            domain: "github.com",
            records: [{ type: "A", value: "140.82.114.4", ttl: 60 }],
          },
          request_id: "req_01J8XXXXX",
        }, null, 2),
      },
      {
        id: "info-icp",
        method: "GET",
        path: "/v1/info/icp",
        summary: "ICP 备案查询",
        description: "查询域名的 ICP 备案信息（中国大陆）。",
        auth: false,
        parameters: [
          { name: "domain", location: "query", required: true, type: "string", description: "要查询的域名", example: "baidu.com" },
        ],
        responseExample: JSON.stringify({
          data: {
            domain: "baidu.com",
            icp_number: "京ICP证030173号",
            company: "北京百度网讯科技有限公司",
            license_type: "经营性ICP许可证",
          },
          request_id: "req_01J8XXXXX",
        }, null, 2),
      },
    ],
  },
  {
    id: "auth",
    label: "账号",
    description: "注册 / 登录 / 个人资料 / API Key 管理",
    endpoints: [
      {
        id: "auth-register",
        method: "POST",
        path: "/v1/auth/register",
        summary: "用户注册",
        description: "注册新用户账号，注册后发送验证邮件。",
        auth: false,
        parameters: [
          { name: "email", location: "body", required: true, type: "string", description: "邮箱地址", example: "user@example.com" },
          { name: "password", location: "body", required: true, type: "string", description: "密码，最少 8 位" },
        ],
        responseExample: JSON.stringify({
          data: {
            access_token: "eyJhbGciOiJIUzI1...",
            token_type: "Bearer",
            expires_in: 900,
          },
          request_id: "req_01J8XXXXX",
        }, null, 2),
      },
      {
        id: "auth-login",
        method: "POST",
        path: "/v1/auth/login",
        summary: "用户登录",
        description: "使用邮箱和密码登录，返回 JWT access token（有效期 15 分钟）。",
        auth: false,
        parameters: [
          { name: "email", location: "body", required: true, type: "string", description: "邮箱地址", example: "user@example.com" },
          { name: "password", location: "body", required: true, type: "string", description: "密码" },
        ],
        responseExample: JSON.stringify({
          data: {
            access_token: "eyJhbGciOiJIUzI1...",
            token_type: "Bearer",
            expires_in: 900,
          },
          request_id: "req_01J8XXXXX",
        }, null, 2),
      },
      {
        id: "auth-logout",
        method: "POST",
        path: "/v1/auth/logout",
        summary: "用户登出",
        description: "吊销当前 session token。",
        auth: true,
        responseExample: JSON.stringify({ data: null, request_id: "req_01J8XXXXX" }, null, 2),
      },
      {
        id: "account-profile-get",
        method: "GET",
        path: "/v1/account/profile",
        summary: "获取用户资料",
        description: "获取当前登录用户的资料信息。",
        auth: true,
        responseExample: JSON.stringify({
          data: {
            id: "u_01J8XXXXX",
            email: "user@example.com",
            display_name: "张三",
            plan: "pro",
            email_verified: true,
          },
          request_id: "req_01J8XXXXX",
        }, null, 2),
      },
      {
        id: "account-profile-update",
        method: "PATCH",
        path: "/v1/account/profile",
        summary: "更新用户资料",
        description: "更新当前登录用户的显示名称等资料。",
        auth: true,
        parameters: [
          { name: "display_name", location: "body", required: false, type: "string", description: "显示名称" },
        ],
        responseExample: JSON.stringify({
          data: { id: "u_01J8XXXXX", display_name: "李四" },
          request_id: "req_01J8XXXXX",
        }, null, 2),
      },
      {
        id: "apikey-list",
        method: "GET",
        path: "/v1/account/api-keys",
        summary: "获取 API Key 列表",
        description: "获取当前用户的所有 API Key（不含完整密钥）。",
        auth: true,
        responseExample: JSON.stringify({
          data: {
            items: [{ id: "key_01J8XXXXX", name: "生产环境", prefix: "idc_live_xxx..." }],
          },
          request_id: "req_01J8XXXXX",
        }, null, 2),
      },
      {
        id: "apikey-create",
        method: "POST",
        path: "/v1/account/api-keys",
        summary: "创建 API Key",
        description: "创建新的 API Key。完整密钥仅在创建时返回一次，请妥善保管。",
        auth: true,
        parameters: [
          { name: "name", location: "body", required: true, type: "string", description: "API Key 名称（便于识别）", example: "生产环境" },
        ],
        responseExample: JSON.stringify({
          data: {
            id: "key_01J8XXXXX",
            name: "生产环境",
            key: "idc_live_01J8XXXXXXXXXXX",
          },
          request_id: "req_01J8XXXXX",
        }, null, 2),
      },
    ],
  },
  {
    id: "monitors",
    label: "监控",
    description: "监控项创建 / 查询 / 更新 / 删除 / 暂停 / 恢复",
    endpoints: [
      {
        id: "monitor-list",
        method: "GET",
        path: "/v1/monitors",
        summary: "获取监控列表",
        description: "获取当前用户的所有监控项，支持分页。",
        auth: true,
        parameters: [
          { name: "page", location: "query", required: false, type: "integer", description: "页码，默认 1" },
          { name: "per_page", location: "query", required: false, type: "integer", description: "每页条数，默认 20，最大 100" },
        ],
        responseExample: JSON.stringify({
          data: {
            items: [{ id: "mon_01J8XXXXX", name: "GitHub 主页", type: "http", status: "active", interval: 60 }],
            total: 1,
            page: 1,
            per_page: 20,
          },
          request_id: "req_01J8XXXXX",
        }, null, 2),
      },
      {
        id: "monitor-create",
        method: "POST",
        path: "/v1/monitors",
        summary: "创建监控",
        description: "创建新的监控项。",
        auth: true,
        parameters: [
          { name: "name", location: "body", required: true, type: "string", description: "监控名称", example: "GitHub 主页" },
          { name: "type", location: "body", required: true, type: "string", description: "监控类型 http/ping/tcp/dns/traceroute", example: "http" },
          { name: "url", location: "body", required: false, type: "string", description: "目标 URL（http 类型必填）", example: "https://github.com" },
          { name: "interval", location: "body", required: false, type: "integer", description: "拨测间隔（秒），默认 60" },
        ],
        responseExample: JSON.stringify({
          data: { id: "mon_01J8XXXXX", name: "GitHub 主页", type: "http", status: "active" },
          request_id: "req_01J8XXXXX",
        }, null, 2),
      },
      {
        id: "monitor-get",
        method: "GET",
        path: "/v1/monitors/{id}",
        summary: "获取监控详情",
        description: "获取指定监控项的详细信息。",
        auth: true,
        parameters: [
          { name: "id", location: "path", required: true, type: "string", description: "监控 ID", example: "mon_01J8XXXXX" },
        ],
        responseExample: JSON.stringify({
          data: { id: "mon_01J8XXXXX", name: "GitHub 主页", type: "http", url: "https://github.com", status: "active", interval: 60 },
          request_id: "req_01J8XXXXX",
        }, null, 2),
      },
      {
        id: "monitor-update",
        method: "PATCH",
        path: "/v1/monitors/{id}",
        summary: "更新监控",
        description: "更新指定监控项的配置（名称、间隔等）。",
        auth: true,
        parameters: [
          { name: "id", location: "path", required: true, type: "string", description: "监控 ID" },
          { name: "name", location: "body", required: false, type: "string", description: "新名称" },
          { name: "interval", location: "body", required: false, type: "integer", description: "新拨测间隔（秒）" },
        ],
        responseExample: JSON.stringify({
          data: { id: "mon_01J8XXXXX", name: "新名称", interval: 120 },
          request_id: "req_01J8XXXXX",
        }, null, 2),
      },
      {
        id: "monitor-delete",
        method: "DELETE",
        path: "/v1/monitors/{id}",
        summary: "删除监控",
        description: "删除指定监控项（不可恢复）。",
        auth: true,
        parameters: [
          { name: "id", location: "path", required: true, type: "string", description: "监控 ID" },
        ],
        responseExample: JSON.stringify({ data: null, request_id: "req_01J8XXXXX" }, null, 2),
      },
      {
        id: "monitor-pause",
        method: "POST",
        path: "/v1/monitors/{id}/pause",
        summary: "暂停监控",
        description: "暂停指定监控项的拨测任务。",
        auth: true,
        parameters: [
          { name: "id", location: "path", required: true, type: "string", description: "监控 ID" },
        ],
        responseExample: JSON.stringify({
          data: { id: "mon_01J8XXXXX", status: "paused" },
          request_id: "req_01J8XXXXX",
        }, null, 2),
      },
      {
        id: "monitor-resume",
        method: "POST",
        path: "/v1/monitors/{id}/resume",
        summary: "恢复监控",
        description: "恢复已暂停的监控项。",
        auth: true,
        parameters: [
          { name: "id", location: "path", required: true, type: "string", description: "监控 ID" },
        ],
        responseExample: JSON.stringify({
          data: { id: "mon_01J8XXXXX", status: "active" },
          request_id: "req_01J8XXXXX",
        }, null, 2),
      },
    ],
  },
  {
    id: "alerts",
    label: "告警",
    description: "告警通道 / 策略 / 事件管理",
    endpoints: [
      {
        id: "alert-channels-list",
        method: "GET",
        path: "/v1/alert-channels",
        summary: "获取告警通道列表",
        description: "获取当前用户配置的所有告警通道。",
        auth: true,
        responseExample: JSON.stringify({
          data: {
            items: [{ id: "ch_01J8XXXXX", name: "邮件通知", type: "email" }],
          },
          request_id: "req_01J8XXXXX",
        }, null, 2),
      },
      {
        id: "alert-channels-create",
        method: "POST",
        path: "/v1/alert-channels",
        summary: "创建告警通道",
        description: "创建新的告警通道（邮件/Webhook/企微/钉钉/飞书等）。",
        auth: true,
        parameters: [
          { name: "name", location: "body", required: true, type: "string", description: "通道名称" },
          { name: "type", location: "body", required: true, type: "string", description: "通道类型：email/webhook/wecom/dingtalk/feishu/sms/pagerduty/slack" },
          { name: "config", location: "body", required: false, type: "object", description: "通道配置（因类型而异）" },
        ],
        responseExample: JSON.stringify({
          data: { id: "ch_01J8XXXXX", name: "企业微信", type: "wecom" },
          request_id: "req_01J8XXXXX",
        }, null, 2),
      },
      {
        id: "alert-policies-list",
        method: "GET",
        path: "/v1/alert-policies",
        summary: "获取告警策略列表",
        description: "获取当前用户的所有告警策略。",
        auth: true,
        responseExample: JSON.stringify({
          data: {
            items: [{ id: "pol_01J8XXXXX", name: "可用性告警", monitor_id: "mon_01J8XXXXX" }],
          },
          request_id: "req_01J8XXXXX",
        }, null, 2),
      },
      {
        id: "alert-policies-create",
        method: "POST",
        path: "/v1/alert-policies",
        summary: "创建告警策略",
        description: "创建新的告警策略，绑定监控项和告警通道。",
        auth: true,
        parameters: [
          { name: "name", location: "body", required: true, type: "string", description: "策略名称" },
          { name: "monitor_id", location: "body", required: true, type: "string", description: "关联的监控项 ID" },
          { name: "channel_ids", location: "body", required: false, type: "string[]", description: "告警通道 ID 列表" },
        ],
        responseExample: JSON.stringify({
          data: { id: "pol_01J8XXXXX", name: "可用性告警", monitor_id: "mon_01J8XXXXX" },
          request_id: "req_01J8XXXXX",
        }, null, 2),
      },
      {
        id: "alert-events-list",
        method: "GET",
        path: "/v1/alert-events",
        summary: "获取告警事件列表",
        description: "获取当前用户的所有告警事件历史。",
        auth: true,
        responseExample: JSON.stringify({
          data: {
            items: [{ id: "evt_01J8XXXXX", policy_id: "pol_01J8XXXXX", type: "down", created_at: "2026-05-14T06:00:00Z" }],
          },
          request_id: "req_01J8XXXXX",
        }, null, 2),
      },
    ],
  },
  {
    id: "billing",
    label: "计费",
    description: "订阅 / 取消 / 账单查询",
    endpoints: [
      {
        id: "billing-subscribe",
        method: "POST",
        path: "/v1/billing/subscribe",
        summary: "订阅付费计划",
        description: "发起付费计划订阅，返回 Paddle 支付跳转 URL。",
        auth: true,
        parameters: [
          { name: "plan", location: "body", required: true, type: "string", description: "计划名称：pro / business", example: "pro" },
        ],
        responseExample: JSON.stringify({
          data: { checkout_url: "https://checkout.paddle.com/..." },
          request_id: "req_01J8XXXXX",
        }, null, 2),
      },
      {
        id: "billing-subscription",
        method: "GET",
        path: "/v1/billing/subscription",
        summary: "获取当前订阅信息",
        description: "获取当前登录用户的订阅状态和到期时间。",
        auth: true,
        responseExample: JSON.stringify({
          data: { plan: "pro", status: "active", current_period_end: "2026-06-14T00:00:00Z" },
          request_id: "req_01J8XXXXX",
        }, null, 2),
      },
      {
        id: "billing-invoices",
        method: "GET",
        path: "/v1/billing/invoices",
        summary: "获取账单列表",
        description: "获取当前用户的历史账单记录。",
        auth: true,
        responseExample: JSON.stringify({
          data: {
            items: [{ id: "inv_01J8XXXXX", amount: 99, currency: "CNY", status: "paid", created_at: "2026-05-14T00:00:00Z" }],
          },
          request_id: "req_01J8XXXXX",
        }, null, 2),
      },
    ],
  },
  {
    id: "nodes",
    label: "节点",
    description: "拨测节点目录",
    endpoints: [
      {
        id: "nodes-list",
        method: "GET",
        path: "/v1/nodes",
        summary: "获取节点列表",
        description: "获取所有可用的拨测节点信息，包括地区、运营商和在线状态。",
        auth: false,
        responseExample: JSON.stringify({
          data: {
            items: [
              { id: "cn-bj", name: "北京", country: "CN", region: "华北", provider: "阿里云", status: "online" },
              { id: "cn-sh", name: "上海", country: "CN", region: "华东", provider: "腾讯云", status: "online" },
              { id: "us-west", name: "美国西部", country: "US", region: "西海岸", provider: "AWS", status: "online" },
            ],
          },
          request_id: "req_01J8XXXXX",
        }, null, 2),
      },
    ],
  },
]

// Flat list of all endpoints (for search/iteration)
export const ALL_ENDPOINTS: Endpoint[] = API_GROUPS.flatMap((g) => g.endpoints)
