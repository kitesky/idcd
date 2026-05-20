import { render, screen } from "@testing-library/react"
import { describe, it, expect } from "vitest"
import BgpPage from "../page"

describe("BgpPage", () => {
  it("renders heading", () => {
    render(<BgpPage />)
    expect(screen.getByRole("heading", { level: 1 })).toBeTruthy()
  })
})
