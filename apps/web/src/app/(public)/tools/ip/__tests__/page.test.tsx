import { render, screen } from "@testing-library/react"
import { describe, it, expect } from "vitest"
import IpPage from "../page"

describe("IpPage", () => {
  it("renders IP tool heading", () => {
    render(<IpPage />)
    expect(screen.getByRole("heading", { level: 1 })).toBeTruthy()
  })
})
