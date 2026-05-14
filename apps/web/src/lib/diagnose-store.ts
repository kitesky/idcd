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

// Module-level store — shared within same Node.js process (dev/single-instance).
// Replace with Redis for multi-instance production.
const store = new Map<string, DiagnoseReport>()

export function saveReport(report: DiagnoseReport): void {
  store.set(report.id, report)
}

export function getReport(id: string): DiagnoseReport | undefined {
  return store.get(id)
}
