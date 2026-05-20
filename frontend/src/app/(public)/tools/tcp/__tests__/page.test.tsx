import { render, screen } from "@testing-library/react"
import { describe, it, expect } from "vitest"
import TcpPage from "../page"

describe("TcpPage", () => {
  it("renders heading", () => {
    render(<TcpPage />)
    expect(screen.getByRole("heading", { level: 1 })).toBeTruthy()
  })
})
