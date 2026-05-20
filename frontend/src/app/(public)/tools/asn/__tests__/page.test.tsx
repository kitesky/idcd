import { render, screen } from "@testing-library/react"
import { describe, it, expect } from "vitest"
import AsnPage from "../page"

describe("AsnPage", () => {
  it("renders heading", () => {
    render(<AsnPage />)
    expect(screen.getByRole("heading", { level: 1 })).toBeTruthy()
  })
})
