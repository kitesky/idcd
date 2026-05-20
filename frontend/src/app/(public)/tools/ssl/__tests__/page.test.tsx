import { render, screen } from "@testing-library/react"
import { describe, it, expect } from "vitest"
import SslPage from "../page"

describe("SslPage", () => {
  it("renders SSL tool heading", () => {
    render(<SslPage />)
    expect(screen.getByRole("heading", { level: 1 })).toBeTruthy()
  })
})
