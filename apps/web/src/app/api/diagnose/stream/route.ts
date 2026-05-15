import { NextRequest } from "next/server"
import { saveReport } from "@/lib/diagnose-store"
import type { CheckResult } from "@/lib/diagnose-store"

export const dynamic = "force-dynamic"

const BASE_URL = process.env.INTERNAL_API_URL ?? "http://localhost:8080"

interface CheckDef {
  key: string
  label: string
  fetch: (domain: string) => Promise<Response>
  summarize: (data: unknown) => string
}

const CHECKS: CheckDef[] = [
  {
    key: "dns",
    label: "DNS 解析",
    fetch: (domain) => fetch(`${BASE_URL}/v1/info/dns?q=${encodeURIComponent(domain)}&type=A`),
    summarize: (data) => {
      const d = data as { records?: { value: string }[]; type?: string }
      const count = d.records?.length ?? 0
      const ips = d.records?.slice(0, 2).map((r) => r.value).join(", ") ?? ""
      return count > 0 ? `解析到 ${count} 条 ${d.type ?? "A"} 记录：${ips}` : "未解析到记录"
    },
  },
  {
    key: "http",
    label: "HTTP 可达性",
    fetch: (domain) =>
      fetch(`${BASE_URL}/v1/probe/http`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ target: domain }),
      }),
    summarize: (data) => {
      const d = data as { task_id?: string; status?: string }
      return `探测任务已提交 (task_id: ${d.task_id ?? "-"}, status: ${d.status ?? "-"})`
    },
  },
  {
    key: "ping",
    label: "Ping 延迟",
    fetch: (domain) =>
      fetch(`${BASE_URL}/v1/probe/ping`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ target: domain }),
      }),
    summarize: (data) => {
      const d = data as { task_id?: string; status?: string }
      return `探测任务已提交 (task_id: ${d.task_id ?? "-"}, status: ${d.status ?? "-"})`
    },
  },
  {
    key: "traceroute",
    label: "路由追踪",
    fetch: (domain) =>
      fetch(`${BASE_URL}/v1/probe/traceroute`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ target: domain }),
      }),
    summarize: (data) => {
      const d = data as { task_id?: string; status?: string }
      return `探测任务已提交 (task_id: ${d.task_id ?? "-"}, status: ${d.status ?? "-"})`
    },
  },
  {
    key: "ssl",
    label: "SSL 证书",
    fetch: (domain) => fetch(`${BASE_URL}/v1/info/ssl?q=${encodeURIComponent(domain)}`),
    summarize: (data) => {
      const d = data as { issuer?: string; days_until_expiry?: number; subject?: string }
      if (d.days_until_expiry !== undefined) {
        return `证书有效，${d.issuer ?? "未知 CA"} 颁发，剩余 ${d.days_until_expiry} 天`
      }
      return "已获取 SSL 证书信息"
    },
  },
  {
    key: "icp",
    label: "ICP 备案",
    fetch: (domain) => fetch(`${BASE_URL}/v1/info/icp?q=${encodeURIComponent(domain)}`),
    summarize: (data) => {
      const d = data as { icp_number?: string; company?: string; note?: string }
      if (d.icp_number) {
        return `ICP 备案号：${d.icp_number}${d.company ? `，${d.company}` : ""}`
      }
      return d.note ?? "未查询到 ICP 备案记录"
    },
  },
  {
    key: "whois",
    label: "WHOIS",
    fetch: (domain) => fetch(`${BASE_URL}/v1/info/whois?q=${encodeURIComponent(domain)}`),
    summarize: (data) => {
      const d = data as { registrar?: string; expiry_date?: string; creation_date?: string }
      const parts: string[] = []
      if (d.registrar) parts.push(`注册商: ${d.registrar}`)
      if (d.expiry_date) parts.push(`到期 ${d.expiry_date}`)
      return parts.length > 0 ? parts.join("，") : "已获取 WHOIS 信息"
    },
  },
]

// Compiled once at module load — reused on every isPublicDomain call.
const IPV4_RE = /^(\d{1,3}\.){3}\d{1,3}$/
// Each DNS label: starts and ends with alnum, optionally has hyphens in the middle.
const LABEL_RE = /^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/i

// Block internal/reserved hostnames to prevent SSRF via the diagnose endpoint.
// Only allows public domain names (labels separated by dots, no IPs, no localhost).
function isPublicDomain(domain: string): boolean {
  // Reject anything that looks like an IP address (v4 or v6)
  if (IPV4_RE.test(domain)) return false
  if (domain.includes(":")) return false // IPv6 or port-qualified host
  // Must look like a valid public hostname: labels of [a-z0-9-] separated by dots,
  // at least two labels, no leading/trailing hyphens per label.
  const labels = domain.split(".")
  if (labels.length < 2) return false
  if (!labels.every((l) => l.length > 0 && LABEL_RE.test(l))) return false
  // Reject well-known internal/loopback names
  const blocklist = ["localhost", "local", "internal", "intranet", "localdomain"]
  if (blocklist.includes(labels[labels.length - 1].toLowerCase())) return false
  return true
}

export async function GET(req: NextRequest) {
  const domain = req.nextUrl.searchParams.get("domain")?.trim()
  if (!domain) {
    return new Response("Missing domain parameter", { status: 400 })
  }
  if (!isPublicDomain(domain)) {
    return new Response("Invalid domain parameter", { status: 400 })
  }

  const encoder = new TextEncoder()
  const reportId = crypto.randomUUID()

  const stream = new ReadableStream({
    async start(controller) {
      const send = (data: object) => {
        controller.enqueue(encoder.encode(`data: ${JSON.stringify(data)}\n\n`))
      }

      const completedChecks: CheckResult[] = []
      let errorCount = 0

      const checkPromises = CHECKS.map((check) => {
        send({ type: "check_start", key: check.key })

        return check
          .fetch(domain)
          .then(async (res) => {
            if (!res.ok) {
              const text = await res.text().catch(() => res.statusText)
              throw new Error(`HTTP ${res.status}: ${text}`)
            }
            const detail = (await res.json()) as unknown
            const summary = check.summarize(detail)
            send({ type: "check_done", key: check.key, summary, detail })
            completedChecks.push({
              key: check.key,
              label: check.label,
              status: "done",
              summary,
              detail: detail as Record<string, unknown>,
            })
          })
          .catch((err: unknown) => {
            const message = err instanceof Error ? err.message : String(err)
            const summary = `检测失败：${message}`
            send({ type: "check_done", key: check.key, summary, detail: { error: message } })
            completedChecks.push({
              key: check.key,
              label: check.label,
              status: "error",
              summary,
              error: message,
            })
            errorCount++
          })
      })

      await Promise.allSettled(checkPromises)

      saveReport({
        id: reportId,
        domain,
        createdAt: new Date().toISOString(),
        checks: completedChecks,
        doneCount: completedChecks.length - errorCount,
        errorCount,
      })

      send({ type: "complete", reportId })
      controller.close()
    },
  })

  return new Response(stream, {
    headers: {
      "Content-Type": "text/event-stream",
      "Cache-Control": "no-cache, no-transform",
      Connection: "keep-alive",
      "X-Accel-Buffering": "no",
    },
  })
}
