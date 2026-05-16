"use client"

import { useState } from "react"
import { Share2, Check, Loader2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import { saveProbeReport, shareUrlFor } from "@/lib/probe-share"
import type { SingleProbeReport, AnyReport } from "@/lib/diagnose-store"
import type { ProbeTaskResult } from "@/lib/api"

interface ShareResultButtonProps {
  tool: SingleProbeReport["tool"]
  target: string
  params?: Record<string, unknown>
  taskResult: ProbeTaskResult | null
}

/**
 * "Share Result" button that lives next to a finished probe-task result.
 *
 * On first click: POST the task snapshot to /v1/diagnose/reports (Redis 7d
 * TTL) and copy /r/{id} to clipboard. Subsequent clicks just re-copy the
 * URL, no new save. The button is disabled until the task reaches a
 * terminal state — sharing an in-flight task would leak a half-baked URL.
 */
export default function ShareResultButton({
  tool,
  target,
  params,
  taskResult,
}: ShareResultButtonProps) {
  const [savedId, setSavedId] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [copied, setCopied] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const ready =
    taskResult !== null &&
    (taskResult.status === "completed" || taskResult.status === "failed")

  const handleClick = async () => {
    if (!ready || busy) return
    setError(null)

    let id = savedId
    if (!id) {
      setBusy(true)
      const payload: Omit<AnyReport, "id" | "createdAt"> = {
        type: "single",
        tool,
        target,
        params,
        taskId: taskResult!.task_id,
        status: taskResult!.status,
        result: taskResult!.result,
      } as Omit<SingleProbeReport, "id" | "createdAt">
      id = await saveProbeReport(payload as Omit<SingleProbeReport, "id" | "createdAt">)
      setBusy(false)
      if (!id) {
        setError("保存失败，请重试")
        return
      }
      setSavedId(id)
    }

    const url = shareUrlFor(id)
    try {
      await navigator.clipboard.writeText(url)
      setCopied(true)
      setTimeout(() => setCopied(false), 1800)
    } catch {
      setError("无法访问剪贴板，请手动复制：" + url)
    }
  }

  return (
    <div className="flex items-center gap-2">
      <Button
        type="button"
        variant="outline"
        size="sm"
        onClick={handleClick}
        disabled={!ready || busy}
        data-testid="share-result-button"
      >
        {busy ? (
          <>
            <Loader2 className="mr-1 h-4 w-4 animate-spin" />
            生成中…
          </>
        ) : copied ? (
          <>
            <Check className="mr-1 h-4 w-4" />
            链接已复制
          </>
        ) : (
          <>
            <Share2 className="mr-1 h-4 w-4" />
            {savedId ? "再次复制链接" : "分享结果"}
          </>
        )}
      </Button>
      {error && <span className="text-xs text-destructive">{error}</span>}
    </div>
  )
}
