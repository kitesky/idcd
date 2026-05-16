export interface UpgradeRollout {
  id: string
  version: string
  download_url: string
  checksum: string
  rollout_pct: number
  status: "active" | "paused" | "completed"
  created_at: string
  updated_at: string
}
