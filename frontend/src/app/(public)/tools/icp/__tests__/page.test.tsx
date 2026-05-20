import { render, screen } from "@testing-library/react"
import { describe, it, expect } from "vitest"
import IcpPage from "../page"

describe("IcpPage", () => {
  it("renders heading", () => {
    render(<IcpPage />)
    expect(screen.getByRole("heading", { level: 1 })).toBeTruthy()
  })
})
