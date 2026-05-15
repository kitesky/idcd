import { render, screen } from "@testing-library/react"
import { describe, it, expect } from "vitest"
import RdnsPage from "../page"

describe("RdnsPage", () => {
  it("renders heading", () => {
    render(<RdnsPage />)
    expect(screen.getByRole("heading", { level: 1 })).toBeTruthy()
  })
})
