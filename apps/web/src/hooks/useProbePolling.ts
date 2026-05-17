"use client"

import { useState, useEffect } from "react"
import { getProbeTask, type ProbeTaskResult } from "@/lib/api"

interface UseProbePollingResult {
  taskResult: ProbeTaskResult | null
  isPolling: boolean
  error: string
}

const TIMEOUT_MS = 30_000
const INTERVAL_MS = 2_000
const TERMINAL_STATUSES = new Set(["completed", "failed", "cancelled"])

export function useProbePolling(taskId: string | null): UseProbePollingResult {
  const [taskResult, setTaskResult] = useState<ProbeTaskResult | null>(null)
  const [isPolling, setIsPolling] = useState(false)
  const [error, setError] = useState("")

  useEffect(() => {
    if (!taskId) return

    let cancelled = false
    let timer: ReturnType<typeof setTimeout> | null = null
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
        if (!cancelled) setError("拨测超时（30s），请重试")
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
      timer = setTimeout(poll, INTERVAL_MS)
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
