import { render, screen } from "@testing-library/react"
import { describe, it, expect } from "vitest"
import { UptimeBar } from "../uptime-bar"
import type { MonitorHistory } from "../types"

function makeHistory(days: number): MonitorHistory[] {
  const now = new Date("2026-05-20T00:00:00Z")
  return Array.from({ length: days }, (_, i) => {
    const d = new Date(now)
    d.setUTCDate(d.getUTCDate() - (days - 1 - i))
    return {
      date: d.toISOString().slice(0, 10),
      status: "operational" as const,
      uptime: 100,
    }
  })
}

describe("UptimeBar", () => {
  it("renders one cell per day in history (30-day window)", () => {
    const history = makeHistory(30)
    render(<UptimeBar history={history} />)

    const grid = screen.getByTestId("uptime-grid")
    expect(grid).toBeInTheDocument()
    // Each day is a Radix Tooltip trigger wrapping a div — count the dated cells via aria-label
    const cells = grid.querySelectorAll("[role='img']")
    expect(cells.length).toBe(30)
  })

  it("supports arbitrary history lengths (90-day window)", () => {
    const history = makeHistory(90)
    render(<UptimeBar history={history} />)

    const grid = screen.getByTestId("uptime-grid")
    const cells = grid.querySelectorAll("[role='img']")
    expect(cells.length).toBe(90)
    expect(grid.getAttribute("style") ?? "").toMatch(/repeat\(90, 1fr\)/)
  })

  it("renders label heading when provided", () => {
    render(<UptimeBar history={makeHistory(7)} label="过去 7 天" />)

    expect(screen.getByRole("heading", { name: "过去 7 天" })).toBeInTheDocument()
  })

  it("omits legend when showLegend is false", () => {
    render(<UptimeBar history={makeHistory(7)} showLegend={false} />)

    // axisLeft only renders in the legend block
    expect(screen.queryByText("30 天前")).not.toBeInTheDocument()
  })

  it("renders legend by default", () => {
    render(<UptimeBar history={makeHistory(7)} />)

    // status.page.uptime.axisLeft / axisRight come from cn status.json
    expect(screen.getByText("30 天前")).toBeInTheDocument()
    expect(screen.getByText("今天")).toBeInTheDocument()
  })

  it("uses status-driven cell colors", () => {
    const history: MonitorHistory[] = [
      { date: "2026-05-18", status: "operational", uptime: 100 },
      { date: "2026-05-19", status: "degraded", uptime: 95 },
      { date: "2026-05-20", status: "outage", uptime: 50 },
    ]
    render(<UptimeBar history={history} showLegend={false} />)

    const cells = screen
      .getByTestId("uptime-grid")
      .querySelectorAll<HTMLElement>("[role='img']")
    expect(cells[0]?.className).toMatch(/bg-success/)
    expect(cells[1]?.className).toMatch(/bg-warning/)
    expect(cells[2]?.className).toMatch(/bg-destructive/)
  })
})
