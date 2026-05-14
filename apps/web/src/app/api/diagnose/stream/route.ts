import { NextRequest } from "next/server"
import { saveReport } from "@/lib/diagnose-store"
import type { CheckResult } from "@/lib/diagnose-store"

export const dynamic = "force-dynamic"

interface MockCheck {
  key: string
  label: string
  delayMs: number
  result: Record<string, unknown>
  summary: string
}

const MOCK_CHECKS: MockCheck[] = [
  {
    key: "dns",
    label: "DNS 解析",
    delayMs: 500,
    result: { records: ["104.21.45.67", "172.67.145.99"], type: "A", ttl: 300 },
    summary: "解析到 2 条 A 记录：104.21.45.67, 172.67.145.99",
  },
  {
    key: "http",
    label: "HTTP 可达性",
    delayMs: 700,
    result: { statusCode: 200, latencyMs: 142, redirects: 0, protocol: "HTTP/2" },
    summary: "HTTP 200 OK，响应时延 142ms",
  },
  {
    key: "ping",
    label: "Ping 延迟",
    delayMs: 600,
    result: { avgRtt: 23.4, minRtt: 18.1, maxRtt: 31.2, lossPercent: 0, packets: 4 },
    summary: "平均 RTT 23.4ms，丢包率 0%",
  },
  {
    key: "traceroute",
    label: "路由追踪",
    delayMs: 1400,
    result: { hops: 8, path: ["10.0.0.1", "192.168.1.1", "203.0.113.1", "..."] },
    summary: "共 8 跳路由节点",
  },
  {
    key: "ssl",
    label: "SSL 证书",
    delayMs: 700,
    result: { issuer: "Let's Encrypt", daysRemaining: 127, valid: true, version: "TLSv1.3" },
    summary: "证书有效，Let's Encrypt 颁发，剩余 127 天",
  },
  {
    key: "icp",
    label: "ICP 备案",
    delayMs: 900,
    result: { icpNumber: null, status: "未查询到备案信息", authority: "工业和信息化部" },
    summary: "未查询到 ICP 备案记录",
  },
  {
    key: "whois",
    label: "WHOIS",
    delayMs: 600,
    result: { registrar: "GoDaddy LLC", createdAt: "2015-03-12", expiresAt: "2026-03-12" },
    summary: "注册商: GoDaddy LLC，到期 2026-03-12",
  },
]

function sleep(ms: number): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, ms))
}

export async function GET(req: NextRequest) {
  const domain = req.nextUrl.searchParams.get("domain")?.trim()
  if (!domain) {
    return new Response("Missing domain parameter", { status: 400 })
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

      for (const check of MOCK_CHECKS) {
        send({ type: "check_start", key: check.key })
        await sleep(check.delayMs)
        send({
          type: "check_done",
          key: check.key,
          summary: check.summary,
          detail: check.result,
        })
        completedChecks.push({
          key: check.key,
          label: check.label,
          status: "done",
          summary: check.summary,
          detail: check.result,
        })
      }

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
      "Connection": "keep-alive",
      "X-Accel-Buffering": "no",
    },
  })
}
