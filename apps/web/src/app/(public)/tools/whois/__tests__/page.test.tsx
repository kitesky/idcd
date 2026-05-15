import { render, screen } from "@testing-library/react"
import { describe, it, expect } from "vitest"
import WhoisPage from "../page"

describe("WhoisPage", () => {
  it("renders heading", () => {
    render(<WhoisPage />)
    expect(screen.getByRole("heading", { level: 1 })).toBeTruthy()
  })
})
