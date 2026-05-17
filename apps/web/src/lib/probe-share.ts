"use client"

import { apiRequest } from "./api"
import type { SingleProbeReport } from "./diagnose-store"

// Browser-side counterpart of `saveReport` in diagnose-store.ts, which is server-only (INTERNAL_API_URL).
// Backend respects the client-provided id (apps/api/internal/handler/diagnose_report.go).
export async function saveProbeReport(
  input: Omit<SingleProbeReport, "id" | "createdAt">,
): Promise<string | null> {
  const id = crypto.randomUUID()
  const report: SingleProbeReport = {
    ...input,
    id,
    createdAt: new Date().toISOString(),
  }
  try {
    await apiRequest<{ id: string }>("/v1/diagnose/reports", {
      method: "POST",
      body: JSON.stringify(report),
    })
    return id
  } catch {
    return null
  }
}

export function shareUrlFor(id: string): string {
  if (typeof window === "undefined") return `/r/${id}`
  return `${window.location.origin}/r/${id}`
}
