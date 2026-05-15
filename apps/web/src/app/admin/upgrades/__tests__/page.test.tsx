import { render, screen } from "@testing-library/react"
import { describe, it, expect } from "vitest"
import { UpgradesClient } from "../upgrades-client"
import type { UpgradeRollout } from "../page"

const mockRollout: UpgradeRollout = {
  id: "oru_test01",
  version: "v1.2.0",
  download_url: "https://releases.idcd.com/agent-v1.2.0",
  checksum: "sha256:abc123",
  rollout_pct: 10,
  status: "active",
  created_at: "2026-05-15T00:00:00Z",
  updated_at: "2026-05-15T00:00:00Z",
}

describe("UpgradesClient", () => {
  it("renders empty state when no rollouts", () => {
    render(<UpgradesClient initialRollouts={[]} />)
    expect(screen.getByText("暂无升级计划")).toBeInTheDocument()
  })

  it("renders rollout rows", () => {
    render(<UpgradesClient initialRollouts={[mockRollout]} />)
    expect(screen.getByText("v1.2.0")).toBeInTheDocument()
    expect(screen.getByText("进行中")).toBeInTheDocument()
  })

  it("renders create button", () => {
    render(<UpgradesClient initialRollouts={[]} />)
    expect(screen.getByRole("button", { name: "新建升级计划" })).toBeInTheDocument()
  })

  it("renders pause and complete buttons for active rollout", () => {
    render(<UpgradesClient initialRollouts={[mockRollout]} />)
    expect(screen.getByRole("button", { name: "暂停" })).toBeInTheDocument()
    expect(screen.getByRole("button", { name: "完成" })).toBeInTheDocument()
  })
})
