import { render, screen } from "@testing-library/react"
import { describe, it, expect } from "vitest"
import DkimPage from "../page"

describe("DkimPage", () => {
  it("renders heading", () => {
    render(<DkimPage />)
    expect(screen.getByRole("heading", { level: 1 })).toBeTruthy()
  })
})
