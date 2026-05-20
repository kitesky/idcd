import { render, screen } from "@testing-library/react"
import { describe, it, expect } from "vitest"
import MxPage from "../page"

describe("MxPage", () => {
  it("renders heading", () => {
    render(<MxPage />)
    expect(screen.getByRole("heading", { level: 1 })).toBeTruthy()
  })
})
