export type CheckStatus = "pending" | "running" | "done" | "error"

export interface CheckResult {
  key: string
  label: string
  status: CheckStatus
  summary?: string
  detail?: Record<string, unknown>
  error?: string
}

export interface DiagnoseReport {
  id: string
  domain: string
  createdAt: string
  checks: CheckResult[]
  doneCount: number
  errorCount: number
}

const INTERNAL_API = process.env.INTERNAL_API_URL ?? "http://localhost:8080"

export async function saveReport(report: DiagnoseReport): Promise<void> {
  try {
    await fetch(`${INTERNAL_API}/v1/diagnose/reports`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(report),
    })
  } catch {
    // Best-effort: if API is unavailable, silently skip
  }
}

export async function getReport(id: string): Promise<DiagnoseReport | null> {
  try {
    const res = await fetch(`${INTERNAL_API}/v1/diagnose/reports/${id}`, {
      cache: "no-store",
    })
    if (!res.ok) return null
    return res.json() as Promise<DiagnoseReport>
  } catch {
    return null
  }
}
