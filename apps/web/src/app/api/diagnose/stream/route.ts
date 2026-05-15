import { NextRequest } from "next/server"
import { saveReport } from "@/lib/diagnose-store"
import type { CheckResult } from "@/lib/diagnose-store"

export const dynamic = "force-dynamic"

const BASE_URL = process.env.INTERNAL_API_URL ?? "http://localhost:8080"

// Allow tests to override via env so polling doesn't slow down the test suite.
const POLL_INTERVAL_MS = Number(process.env.PROBE_POLL_INTERVAL_MS ?? 2_000)
const POLL_TIMEOUT_MS = Number(process.env.PROBE_POLL_TIMEOUT_MS ?? 15_000)
// Hard cap on total SSE stream lifetime (60 s); prevents dangling connections.
const STREAM_TIMEOUT_MS = Number(process.env.DIAGNOSE_STREAM_TIMEOUT_MS ?? 60_000)

interface TaskResult {
  task_id: string
  status: string
  result?: {
    success?: boolean
    duration_ms?: number
    error?: string
    [key: string]: unknown
  }
}

async function pollTaskResult(taskId: string): Promise<TaskResult | null> {
  const deadline = Date.now() + POLL_TIMEOUT_MS
  while (Date.now() < deadline) {
    try {
      const res = await fetch(`${BASE_URL}/v1/probe/tasks/${taskId}`)
      if (res.ok) {
        const data = (await res.json()) as TaskResult
        if (
          data.status === "completed" ||
          data.status === "failed" ||
          data.status === "cancelled"
        ) {
          return data
        }
      }
    } catch {
      // continue polling on network error
    }
    if (POLL_INTERVAL_MS > 0) {
      await new Promise((resolve) => setTimeout(resolve, POLL_INTERVAL_MS))
    }
  }
  return null
}

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
      const d = data as { task_id?: string; _result?: TaskResult }
      if (d._result?.result) {
        const r = d._result.result
        const ms = r.duration_ms != null ? `${r.duration_ms} ms` : "-"
        return r.success ? `可达，响应时间 ${ms}` : `不可达${r.error ? `：${r.error}` : ""}`
      }
      return `任务已提交 (${d.task_id ?? "-"})`
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
      const d = data as { task_id?: string; _result?: TaskResult }
      if (d._result?.result) {
        const r = d._result.result
        const ms = r.duration_ms != null ? `${r.duration_ms} ms` : "-"
        return r.success ? `延迟 ${ms}` : `Ping 失败${r.error ? `：${r.error}` : ""}`
      }
      return `任务已提交 (${d.task_id ?? "-"})`
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
      const d = data as { task_id?: string; _result?: TaskResult }
      if (d._result?.result) {
        const r = d._result.result
        return r.success ? `路由追踪完成` : `追踪失败${r.error ? `：${r.error}` : ""}`
      }
      return `任务已提交 (${d.task_id ?? "-"})`
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
  if (blocklist.includes(labels[labels.length - 1]!.toLowerCase())) return false
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

  // Race all checks against a hard stream timeout so the connection is always closed.
  const streamController = new AbortController()
  const streamTimeoutId = STREAM_TIMEOUT_MS > 0
    ? setTimeout(() => streamController.abort(), STREAM_TIMEOUT_MS)
    : null

  const stream = new ReadableStream({
    async start(controller) {
      const send = (data: object) => {
        try {
          controller.enqueue(encoder.encode(`data: ${JSON.stringify(data)}\n\n`))
        } catch {
          // Controller already closed (client disconnected); safe to ignore.
        }
      }

      const completedChecks: CheckResult[] = []
      let errorCount = 0

      try {
        const checkPromises = CHECKS.map((check) => {
          send({ type: "check_start", key: check.key })

          return check
            .fetch(domain)
            .then(async (res) => {
              if (!res.ok) {
                const text = await res.text().catch(() => res.statusText)
                throw new Error(`HTTP ${res.status}: ${text}`)
              }
              let detail = (await res.json()) as Record<string, unknown>

              // For probe tasks, poll until we have a real result.
              if (typeof detail.task_id === "string" && detail.status === "queued") {
                const taskResult = await pollTaskResult(detail.task_id)
                if (taskResult) {
                  detail = { ...detail, _result: taskResult }
                }
              }

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

        // Race against stream timeout abort signal.
        await Promise.race([
          Promise.allSettled(checkPromises),
          new Promise<void>((_, reject) =>
            streamController.signal.addEventListener("abort", () =>
              reject(new Error("stream timeout"))
            )
          ),
        ])

        await saveReport({
          id: reportId,
          domain,
          createdAt: new Date().toISOString(),
          checks: completedChecks,
          doneCount: completedChecks.length - errorCount,
          errorCount,
        })

        send({ type: "complete", reportId })
      } catch (err: unknown) {
        const message = err instanceof Error ? err.message : String(err)
        send({ type: "error", message })
      } finally {
        if (streamTimeoutId !== null) clearTimeout(streamTimeoutId)
        controller.close()
      }
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
