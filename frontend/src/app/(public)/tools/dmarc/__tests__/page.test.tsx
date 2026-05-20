import { render, screen } from "@testing-library/react"
import { describe, it, expect } from "vitest"
import DmarcPage from "../page"

describe("DmarcPage", () => {
  it("renders heading", () => {
    render(<DmarcPage />)
    expect(screen.getByRole("heading", { level: 1 })).toBeTruthy()
  })
})
