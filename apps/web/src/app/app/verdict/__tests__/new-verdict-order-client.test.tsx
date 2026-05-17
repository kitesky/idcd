import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, fireEvent, waitFor } from "@testing-library/react"

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), back: vi.fn(), replace: vi.fn() }),
}))

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}))

vi.mock("@/lib/api/verdict", async (orig) => {
  const actual = await orig<typeof import("@/lib/api/verdict")>()
  return {
    ...actual,
    createVerdictOrder: vi.fn(),
  }
})

import { createVerdictOrder } from "@/lib/api/verdict"
import { NewVerdictOrderClient } from "../new/new-verdict-order-client"

const mockCreate = vi.mocked(createVerdictOrder)

describe("NewVerdictOrderClient", () => {
  beforeEach(() => {
    mockCreate.mockReset()
  })

  it("renders the form fields", () => {
    render(<NewVerdictOrderClient />)
    expect(screen.getByTestId("template-select")).toBeInTheDocument()
    expect(screen.getByTestId("target-input")).toBeInTheDocument()
    expect(screen.getByTestId("start-input")).toBeInTheDocument()
    expect(screen.getByTestId("end-input")).toBeInTheDocument()
    expect(screen.getByTestId("channel-select")).toBeInTheDocument()
    expect(screen.getByTestId("submit-btn")).toBeInTheDocument()
  })

  it("requires a target on submit", async () => {
    render(<NewVerdictOrderClient />)
    fireEvent.click(screen.getByTestId("submit-btn"))
    await waitFor(() => {
      expect(mockCreate).not.toHaveBeenCalled()
    })
    expect(screen.getByText(/请填写目标/)).toBeInTheDocument()
  })

  it("calls createVerdictOrder with normalized payload on submit", async () => {
    mockCreate.mockResolvedValueOnce({
      order_id: "ord_123",
      checkout_url: "https://pay.example/x",
      status: "pending",
    })

    // jsdom's window.location.assign blows up by default; stub it.
    const origLocation = window.location
    Object.defineProperty(window, "location", {
      configurable: true,
      value: { ...origLocation, assign: vi.fn(), origin: "https://app.test" },
    })

    render(<NewVerdictOrderClient />)
    fireEvent.change(screen.getByTestId("target-input"), {
      target: { value: "example.com" },
    })
    fireEvent.click(screen.getByTestId("submit-btn"))

    await waitFor(() => {
      expect(mockCreate).toHaveBeenCalledTimes(1)
    })
    const arg = mockCreate.mock.calls[0]![0]
    expect(arg.target).toBe("example.com")
    expect(arg.channel).toBeTruthy()
    expect(arg.return_url).toContain("/app/verdict/{order_id}")
    // ISO time strings:
    expect(arg.time_window.start).toMatch(/Z$/)
    expect(arg.time_window.end).toMatch(/Z$/)

    Object.defineProperty(window, "location", {
      configurable: true,
      value: origLocation,
    })
  })

  it("surfaces backend error in an alert", async () => {
    mockCreate.mockRejectedValueOnce(new Error("quota exceeded"))

    render(<NewVerdictOrderClient />)
    fireEvent.change(screen.getByTestId("target-input"), {
      target: { value: "example.com" },
    })
    fireEvent.click(screen.getByTestId("submit-btn"))

    await waitFor(() => {
      expect(screen.getByTestId("submit-error")).toBeInTheDocument()
    })
    expect(screen.getByText(/quota exceeded/)).toBeInTheDocument()
  })
})
