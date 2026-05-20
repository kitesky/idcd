"use client"

import { useState, useEffect } from "react"
import { getProbeTask, type ProbeTaskResult } from "@/lib/api"

interface UseProbePollingResult {
  taskResult: ProbeTaskResult | null
  isPolling: boolean
  error: string
}

// 120s window covers the worst-case "agent disconnected → reconnect → 30s
// retry ticker" path; the 30s cap was an artifact of the old buffer-only
// upload model where every result waited for the agent's 30s upload ticker.
// With live ws.Send on the agent side the typical path completes in 2-5s,
// so the larger window is the safety net, not the expected latency.
const TIMEOUT_MS = 120_000
// Two-phase cadence: hit fast (500ms × 6 = 3s) right after submit to catch
// the common "agent already finished" case, then back off to 2s to keep
// the load reasonable while waiting on slow probes (mtr, speedtest).
const INTERVAL_FAST_MS = 500
const INTERVAL_FAST_LIMIT = 6
const INTERVAL_SLOW_MS = 2_000
const TERMINAL_STATUSES = new Set(["completed", "failed", "cancelled"])

export function useProbePolling(taskId: string | null): UseProbePollingResult {
  const [taskResult, setTaskResult] = useState<ProbeTaskResult | null>(null)
  const [isPolling, setIsPolling] = useState(false)
  const [error, setError] = useState("")

  useEffect(() => {
    if (!taskId) return

    let cancelled = false
    let timer: ReturnType<typeof setTimeout> | null = null
    let attempts = 0
    const startedAt = Date.now()

    const stop = () => {
      if (timer) {
        clearTimeout(timer)
        timer = null
      }
      if (!cancelled) setIsPolling(false)
    }

    const poll = async () => {
      if (cancelled) return
      if (Date.now() - startedAt > TIMEOUT_MS) {
        stop()
        if (!cancelled) {
          setError(`拨测超时（${Math.round(TIMEOUT_MS / 1000)}s），请重试`)
        }
        return
      }
      try {
        const data = await getProbeTask(taskId)
        if (cancelled) return
        setTaskResult(data)
        if (TERMINAL_STATUSES.has(data.status)) {
          stop()
          return
        }
      } catch (e) {
        if (cancelled) return
        stop()
        setError(e instanceof Error ? e.message : "查询结果失败")
        return
      }
      attempts++
      const next = attempts < INTERVAL_FAST_LIMIT ? INTERVAL_FAST_MS : INTERVAL_SLOW_MS
      timer = setTimeout(poll, next)
    }

    // eslint-disable-next-line react-hooks/set-state-in-effect -- 新 taskId 触发：重置状态后启动轮询
    setTaskResult(null)
    setError("")
    setIsPolling(true)
    void poll()

    return () => {
      cancelled = true
      if (timer) clearTimeout(timer)
    }
  }, [taskId])

  return { taskResult, isPolling, error }
}
