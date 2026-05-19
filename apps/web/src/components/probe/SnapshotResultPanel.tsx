"use client"

import { useEffect, useState } from "react"
import { ProbeResultPanel } from "@/components/probe/ProbeResultPanel"
import { getNodes, type Node, type ProbeResult } from "@/lib/api"
import type { SingleProbeReport } from "@/lib/diagnose-store"

// Mirrors usePollingProbeResult's cache — keeps a single /v1/nodes fetch
// across this and the live tool pages so the saved-report view doesn't
// re-stampede the endpoint when users open multiple snapshots.
let nodesCache: Node[] | null = null
let nodesPromise: Promise<Node[]> | null = null
function fetchNodesOnce(): Promise<Node[]> {
  if (nodesCache) return Promise.resolve(nodesCache)
  if (!nodesPromise) {
    nodesPromise = getNodes()
      .then((n) => { nodesCache = n; return n })
      .catch((err) => { nodesPromise = null; throw err })
  }
  return nodesPromise
}

interface SnapshotResultPanelProps {
  report: SingleProbeReport
}

// Renders a saved SingleProbeReport through ProbeResultPanel — same layout
// as the live tool pages so shared links look identical to the running tool.
// node_name isn't captured in the snapshot, so we resolve it live; if the
// node was renamed since capture this shows the *current* name, which is
// acceptable because the table also shows the raw id as the row key.
export function SnapshotResultPanel({ report }: SnapshotResultPanelProps) {
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

  const r = report.result
  const knownKeys = new Set(["node_id", "success", "duration_ms", "error"])
  const details: Record<string, unknown> = r
    ? Object.fromEntries(Object.entries(r).filter(([k]) => !knownKeys.has(k)))
    : {}
  const nodeId = r?.node_id ?? "unknown"

  const probeResult: ProbeResult = {
    task_id: report.taskId,
    status: report.status,
    results: r
      ? [{
          node_id: nodeId,
          node_name: nodeNameById[nodeId] ?? nodeId,
          success: r.success ?? false,
          latency_ms: r.duration_ms,
          error: r.error,
          details: Object.keys(details).length > 0 ? details : undefined,
        }]
      : [],
  }

  return (
    <div className="border rounded-lg bg-background overflow-hidden">
      <ProbeResultPanel
        result={probeResult}
        target={report.target}
        probeType={report.tool}
      />
    </div>
  )
}
