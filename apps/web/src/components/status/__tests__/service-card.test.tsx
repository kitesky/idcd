import { render, screen } from "@testing-library/react"
import { describe, it, expect } from "vitest"
import { ServiceCard } from "../service-card"
import type { MonitorHistory } from "../types"

const sampleHistory: MonitorHistory[] = [
  { date: "2026-05-18", status: "operational", uptime: 100 },
  { date: "2026-05-19", status: "operational", uptime: 100 },
  { date: "2026-05-20", status: "operational", uptime: 99.9 },
]

describe("ServiceCard", () => {
  it("renders name and status dot for minimal props", () => {
    render(<ServiceCard name="API Gateway" status="operational" />)

    const card = screen.getByTestId("service-card-API Gateway")
    expect(card).toHaveTextContent("API Gateway")
    // statusLabel.operational from cn = "正常"
    expect(card.querySelector("[aria-label='正常']")).toBeInTheDocument()
  })

  it("renders uptime percent when provided", () => {
    render(
      <ServiceCard
        name="Web"
        status="operational"
        uptimePercent={99.95}
      />,
    )

    expect(screen.getByText("99.95%")).toBeInTheDocument()
  })

  it("renders description when provided", () => {
    render(
      <ServiceCard
        name="DB"
        status="degraded"
        description="PostgreSQL primary"
      />,
    )

    expect(screen.getByText("PostgreSQL primary")).toBeInTheDocument()
  })

  it("renders UptimeBar when history is provided", () => {
    render(
      <ServiceCard
        name="Probe Service"
        status="operational"
        uptimePercent={99.99}
        history={sampleHistory}
      />,
    )

    const grid = screen.getByTestId("uptime-grid")
    expect(grid).toBeInTheDocument()
    expect(grid.querySelectorAll("[role='img']").length).toBe(3)
  })

  it("does not render UptimeBar when history is missing or empty", () => {
    const { rerender } = render(
      <ServiceCard name="No-history" status="operational" />,
    )
    expect(screen.queryByTestId("uptime-grid")).not.toBeInTheDocument()

    rerender(<ServiceCard name="Empty-history" status="operational" history={[]} />)
    expect(screen.queryByTestId("uptime-grid")).not.toBeInTheDocument()
  })

  it("reflects status in the dot color class", () => {
    render(<ServiceCard name="Down" status="outage" />)
    // statusLabel.outage from cn = "中断"
    const dot = screen.getByLabelText("中断")
    expect(dot.className).toMatch(/bg-destructive/)
  })
})
