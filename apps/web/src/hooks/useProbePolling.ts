"use client"

import { useState, useEffect, useCallback, useRef } from "react"
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
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const startedAtRef = useRef<number>(0)

  const stopPolling = useCallback(() => {
    if (timerRef.current) {
      clearTimeout(timerRef.current)
      timerRef.current = null
    }
    setIsPolling(false)
  }, [])

  const poll = useCallback(async (id: string) => {
    if (Date.now() - startedAtRef.current > TIMEOUT_MS) {
      stopPolling()
      setError("拨测超时（30s），请重试")
      return
    }
    try {
      const data = await getProbeTask(id)
      setTaskResult(data)
      if (TERMINAL_STATUSES.has(data.status)) {
        stopPolling()
        return
      }
    } catch (e) {
      stopPolling()
      setError(e instanceof Error ? e.message : "查询结果失败")
      return
    }
    timerRef.current = setTimeout(() => poll(id), INTERVAL_MS)
  }, [stopPolling])

  useEffect(() => {
    if (!taskId) return
    setTaskResult(null)
    setError("")
    setIsPolling(true)
    startedAtRef.current = Date.now()
    poll(taskId)
    return () => {
      if (timerRef.current) clearTimeout(timerRef.current)
    }
  }, [taskId, poll])

  return { taskResult, isPolling, error }
}
