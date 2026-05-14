import { describe, it, expect, vi } from "vitest"
import { render, screen } from "@testing-library/react"
import "@testing-library/jest-dom"

vi.mock("next/navigation", () => ({
  usePathname: () => "/app/oncall",
  useRouter: () => ({ replace: vi.fn() }),
}))

vi.mock("next/link", () => ({
  default: ({
    children,
    href,
    ...rest
  }: {
    children: React.ReactNode
    href: string
    [key: string]: unknown
  }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}))

import OncallPage from "../page"

describe("OncallPage", () => {
  it("renders the page title", () => {
    render(<OncallPage />)
    expect(screen.getByTestId("oncall-title")).toHaveTextContent("On-Call 排班")
  })

  it("shows current on-call person", () => {
    render(<OncallPage />)
    const nameEl = screen.getByTestId("current-oncall-name")
    expect(nameEl.textContent).toBeTruthy()
    const names = ["Alice Chen", "Bob Wang", "Carol Liu"]
    expect(names.some((n) => nameEl.textContent?.includes(n))).toBe(true)
  })

  it("renders 7-day preview list", () => {
    render(<OncallPage />)
    const preview = screen.getByTestId("preview-list")
    expect(preview).toBeInTheDocument()
    const days = screen.getAllByTestId(/^preview-day-\d$/)
    expect(days).toHaveLength(7)
  })

  it("shows hours until handoff", () => {
    render(<OncallPage />)
    const el = screen.getByTestId("hours-until-handoff")
    expect(el.textContent).toMatch(/距下次交班还有 \d+ 小时/)
  })

  it("renders create schedule and override buttons", () => {
    render(<OncallPage />)
    expect(screen.getByTestId("create-schedule-button")).toBeInTheDocument()
    expect(screen.getByTestId("override-button")).toBeInTheDocument()
  })

  it("current on-call card is present", () => {
    render(<OncallPage />)
    expect(screen.getByTestId("current-oncall-card")).toBeInTheDocument()
  })

  it("schedule preview card is present", () => {
    render(<OncallPage />)
    expect(screen.getByTestId("schedule-preview-card")).toBeInTheDocument()
  })
})
