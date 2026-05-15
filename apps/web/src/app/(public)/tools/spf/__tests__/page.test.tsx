import { render, screen } from "@testing-library/react"
import { describe, it, expect } from "vitest"
import SpfPage from "../page"

describe("SpfPage", () => {
  it("renders heading", () => {
    render(<SpfPage />)
    expect(screen.getByRole("heading", { level: 1 })).toBeTruthy()
  })
})
