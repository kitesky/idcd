"use client"

import { useEffect, useMemo, useState } from "react"
import type { ProbeResult, ProbeTaskResult } from "@/lib/api"
import { fetchNodesOnce } from "@/lib/nodes-cache"

interface PollingState {
  taskResult: ProbeTaskResult | null
  isPolling: boolean
  error: string
}

// Adapts useProbePolling's single-node ProbeTaskResult into the multi-row
// ProbeResult shape ProbeResultPanel was designed for. Resolves node_id →
// node_name once on mount via the shared /v1/nodes cache so callers don't
// each refetch the same node list. Returns null until the agent reports
// back so the panel can show its loading skeleton.
export function usePollingProbeResult(
  polling: PollingState,
): ProbeResult | null {
  const [nodeNameById, setNodeNameById] = useState<Record<string, string>>({})

  useEffect(() => {
    let cancelled = false
    fetchNodesOnce()
      .then((nodes) => {
        if (cancelled) return
        const map: Record<string, string> = {}
        for (const n of nodes) map[n.id] = n.name || n.id
        setNodeNameById(map)
      })
      .catch(() => { /* fall back to raw node_id */ })
    return () => { cancelled = true }
  }, [])

  return useMemo(() => {
    const tr = polling.taskResult
    if (!tr?.result) return null
    const r = tr.result
    const knownKeys = new Set(["node_id", "success", "duration_ms", "error"])
    const details: Record<string, unknown> = Object.fromEntries(
      Object.entries(r).filter(([k]) => !knownKeys.has(k))
    )
    const nodeId = typeof r.node_id === "string" ? r.node_id : "unknown"
    const nodeName = nodeNameById[nodeId] ?? nodeId
    return {
      task_id: tr.task_id,
      status: tr.status,
      results: [{
        node_id: nodeId,
        node_name: nodeName,
        success: Boolean(r.success),
        latency_ms: typeof r.duration_ms === "number" ? r.duration_ms : undefined,
        error: typeof r.error === "string" ? r.error : undefined,
        details: Object.keys(details).length > 0 ? details : undefined,
      }],
    }
  }, [polling.taskResult, nodeNameById])
}
