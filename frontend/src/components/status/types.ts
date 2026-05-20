// Re-export shared status types from the customer-facing page so that
// internal status pages (e.g. idcd.com/status) and external customer status
// pages share a single source of truth.
export type { ServiceStatus, MonitorHistory } from "@/app/status/[slug]/types"
