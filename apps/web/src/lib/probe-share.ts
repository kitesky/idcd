"use client"

import { apiRequest } from "./api"
import type { SingleProbeReport } from "./diagnose-store"

/**
 * Persist a single-tool probe result to the public report store (Redis 7d
 * TTL on the backend). Called from probe-tool clients when the user clicks
 * "share result". Returns the report id on success, null on failure.
 *
 * Implementation note: this is a thin wrapper around POST /v1/diagnose/reports —
 * the same endpoint used by the SSE diagnose flow on the server side. We
 * cannot reuse `saveReport` from `diagnose-store.ts` because that module
 * targets `INTERNAL_API_URL` which is only defined server-side. Browser code
 * must go through `apiRequest` which uses `NEXT_PUBLIC_API_URL`.
 */
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

/** Build the canonical share URL for a saved report. */
export function shareUrlFor(id: string): string {
  if (typeof window === "undefined") return `/r/${id}`
  return `${window.location.origin}/r/${id}`
}
