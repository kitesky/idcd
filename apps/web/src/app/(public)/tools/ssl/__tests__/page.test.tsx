import { render, screen } from "@testing-library/react"
import { describe, it, expect } from "vitest"
import SslPage from "../page"

describe("SslPage", () => {
  it("renders SSL tool heading", () => {
    render(<SslPage />)
    expect(screen.getByText("SSL 证书检测")).toBeTruthy()
  })
})
