import { render, screen } from "@testing-library/react"
import { describe, it, expect } from "vitest"
import { OverallBanner } from "../overall-banner"

describe("OverallBanner", () => {
  it("renders title and operational status label", () => {
    render(<OverallBanner title="idcd Status" status="operational" />)

    expect(screen.getByTestId("status-title")).toHaveTextContent("idcd Status")
    // cn translation for status.page.overall.operational
    expect(screen.getByTestId("overall-status")).toHaveTextContent("全部服务正常")
  })

  it("omits title when not provided", () => {
    render(<OverallBanner status="operational" />)

    expect(screen.queryByTestId("status-title")).not.toBeInTheDocument()
    expect(screen.getByTestId("overall-status")).toBeInTheDocument()
  })

  it("renders degraded label and warning styling", () => {
    render(<OverallBanner status="degraded" />)

    const banner = screen.getByTestId("overall-status")
    expect(banner).toHaveTextContent("部分服务降级")
    expect(banner.className).toMatch(/bg-warning/)
  })

  it("renders outage label and destructive styling", () => {
    render(<OverallBanner status="outage" />)

    const banner = screen.getByTestId("overall-status")
    expect(banner).toHaveTextContent("严重服务中断")
    expect(banner.className).toMatch(/bg-destructive/)
  })

  it("renders maintenance label and info styling", () => {
    render(<OverallBanner status="maintenance" />)

    const banner = screen.getByTestId("overall-status")
    expect(banner).toHaveTextContent("计划维护中")
    expect(banner.className).toMatch(/bg-info/)
  })
})
