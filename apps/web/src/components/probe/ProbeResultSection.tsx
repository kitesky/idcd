"use client"

import { ProbeResultPanel } from "@/components/probe/ProbeResultPanel"
import ShareResultButton from "@/components/probe/ShareResultButton"
import { usePollingProbeResult } from "@/hooks/usePollingProbeResult"
import type { ProbeTaskResult } from "@/lib/api"
import type { SingleProbeReport } from "@/lib/diagnose-store"

interface PollingState {
  taskResult: ProbeTaskResult | null
  isPolling: boolean
  error: string
}

interface ShareContext {
  tool: SingleProbeReport["tool"]
  target: string
  params?: Record<string, unknown>
}

interface ProbeResultSectionProps {
  polling: PollingState
  target: string
  probeType?: string
  submitError?: string
  // Truthy only after the user has submitted at least once — used to keep
  // the section hidden on first paint so the page doesn't show an empty
  // result card before the user has even pressed Go.
  taskId: string | null
  shareContext?: ShareContext
}

// Full-width wrapper that turns a useProbePolling result into a
// ProbeResultPanel render with loading skeleton, error banner, and share
// button. Use this from every single-tool client page instead of the old
// ProbeResults — the panel layout (left map + right rank + HTTP stacked bar)
// is the single source of truth for tool results.
export function ProbeResultSection({
  polling,
  target,
  probeType,
  submitError,
  taskId,
  shareContext,
}: ProbeResultSectionProps) {
  const probeResult = usePollingProbeResult(polling)
  const error = submitError || polling.error
  const isLoading = polling.isPolling

  // Hide the section entirely until the user has actually submitted —
  // taskId is null on first paint and after a reset.
  if (!taskId && !error) return null

  return (
    <div className="border rounded-lg bg-background overflow-hidden">
      {error ? (
        <div className="px-6 py-6 text-sm text-destructive">{error}</div>
      ) : probeResult ? (
        <>
          <ProbeResultPanel
            result={probeResult}
            target={target}
            probeType={probeType}
            isLoading={isLoading}
          />
          {shareContext && polling.taskResult && (
            <div className="px-6 pb-6 -mt-2 flex justify-end">
              <ShareResultButton
                tool={shareContext.tool}
                target={shareContext.target}
                params={shareContext.params}
                taskResult={polling.taskResult}
              />
            </div>
          )}
        </>
      ) : (
        <div className="px-6 py-8 space-y-3">
          {[1, 2, 3].map(i => (
            <div key={i} className="h-12 bg-muted/50 animate-pulse rounded-md" />
          ))}
          <p className="text-xs text-muted-foreground">等待节点返回结果...</p>
        </div>
      )}
    </div>
  )
}
