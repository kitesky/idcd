import type { Metadata } from "next"
import { UpgradesClient } from "./upgrades-client"

export const metadata: Metadata = { title: "OTA 灰度升级 — idcd Admin" }

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

const INTERNAL_API_URL = process.env.INTERNAL_API_URL ?? "http://localhost:8080"
const ADMIN_TOKEN = process.env.ADMIN_TOKEN ?? ""

async function fetchRollouts(): Promise<UpgradeRollout[]> {
  try {
    const res = await fetch(`${INTERNAL_API_URL}/internal/admin/upgrade-rollouts`, {
      headers: { Authorization: `Bearer ${ADMIN_TOKEN}` },
      cache: "no-store",
    })
    if (!res.ok) return []
    const j = await res.json()
    return j.data ?? []
  } catch {
    return []
  }
}

export default async function UpgradesPage() {
  const rollouts = await fetchRollouts()
  return (
    <div>
      <div className="mb-6">
        <h1 className="text-2xl font-bold tracking-tight">OTA 灰度升级</h1>
        <p className="mt-1 text-sm text-muted-foreground">管理节点二进制升级计划（三级灰度：1% / 10% / 100%）</p>
      </div>
      <UpgradesClient initialRollouts={rollouts} />
    </div>
  )
}
