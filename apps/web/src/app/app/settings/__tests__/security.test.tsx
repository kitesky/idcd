import { describe, it, expect, vi } from "vitest"
import { render, screen, fireEvent, waitFor } from "@testing-library/react"
import { SecurityClient } from "../security/security-client"

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
}))

describe("SecurityClient", () => {
  it("renders the security page container", () => {
    render(<SecurityClient />)
    expect(screen.getByTestId("security-page")).toBeInTheDocument()
  })

  it("renders the 2FA card with disabled status badge", () => {
    render(<SecurityClient />)
    const badge = screen.getByTestId("2fa-status-badge")
    expect(badge).toBeInTheDocument()
    expect(badge.textContent).toContain("未启用")
  })

  it("renders the enable 2FA button when disabled", () => {
    render(<SecurityClient />)
    expect(screen.getByTestId("btn-enable-2fa")).toBeInTheDocument()
  })

  it("opens setup dialog step 1 when enable button clicked", () => {
    render(<SecurityClient />)
    fireEvent.click(screen.getByTestId("btn-enable-2fa"))
    expect(screen.getByTestId("2fa-setup-dialog")).toBeInTheDocument()
    expect(screen.getByTestId("2fa-qr-image")).toBeInTheDocument()
    expect(screen.getByTestId("2fa-secret")).toBeInTheDocument()
  })

  it("advances to step 2 when I scanned button clicked", () => {
    render(<SecurityClient />)
    fireEvent.click(screen.getByTestId("btn-enable-2fa"))
    fireEvent.click(screen.getByTestId("btn-scanned"))
    expect(screen.getByTestId("input-totp-code")).toBeInTheDocument()
  })

  it("shows error when code is not 6 digits on verify", () => {
    render(<SecurityClient />)
    fireEvent.click(screen.getByTestId("btn-enable-2fa"))
    fireEvent.click(screen.getByTestId("btn-scanned"))
    fireEvent.change(screen.getByTestId("input-totp-code"), { target: { value: "123" } })
    fireEvent.click(screen.getByTestId("btn-verify-code"))
    expect(screen.getByTestId("2fa-code-error")).toBeInTheDocument()
  })

  it("advances to step 3 with backup codes when valid 6-digit code entered", () => {
    render(<SecurityClient />)
    fireEvent.click(screen.getByTestId("btn-enable-2fa"))
    fireEvent.click(screen.getByTestId("btn-scanned"))
    fireEvent.change(screen.getByTestId("input-totp-code"), { target: { value: "123456" } })
    fireEvent.click(screen.getByTestId("btn-verify-code"))
    expect(screen.getByTestId("backup-codes-grid")).toBeInTheDocument()
  })

  it("shows enabled badge after completing setup flow", async () => {
    render(<SecurityClient />)
    fireEvent.click(screen.getByTestId("btn-enable-2fa"))
    fireEvent.click(screen.getByTestId("btn-scanned"))
    fireEvent.change(screen.getByTestId("input-totp-code"), { target: { value: "654321" } })
    fireEvent.click(screen.getByTestId("btn-verify-code"))
    fireEvent.click(screen.getByTestId("btn-finish-2fa"))
    await waitFor(() => {
      const badge = screen.getByTestId("2fa-status-badge")
      expect(badge.textContent).toContain("已启用")
    })
  })

  it("shows disable dialog when disable button clicked", async () => {
    render(<SecurityClient />)
    fireEvent.click(screen.getByTestId("btn-enable-2fa"))
    fireEvent.click(screen.getByTestId("btn-scanned"))
    fireEvent.change(screen.getByTestId("input-totp-code"), { target: { value: "654321" } })
    fireEvent.click(screen.getByTestId("btn-verify-code"))
    fireEvent.click(screen.getByTestId("btn-finish-2fa"))
    await waitFor(() => expect(screen.getByTestId("btn-disable-2fa")).toBeInTheDocument())
    fireEvent.click(screen.getByTestId("btn-disable-2fa"))
    expect(screen.getByTestId("2fa-disable-dialog")).toBeInTheDocument()
  })

  it("shows error in disable dialog for non-6-digit code", async () => {
    render(<SecurityClient />)
    fireEvent.click(screen.getByTestId("btn-enable-2fa"))
    fireEvent.click(screen.getByTestId("btn-scanned"))
    fireEvent.change(screen.getByTestId("input-totp-code"), { target: { value: "654321" } })
    fireEvent.click(screen.getByTestId("btn-verify-code"))
    fireEvent.click(screen.getByTestId("btn-finish-2fa"))
    await waitFor(() => expect(screen.getByTestId("btn-disable-2fa")).toBeInTheDocument())
    fireEvent.click(screen.getByTestId("btn-disable-2fa"))
    fireEvent.change(screen.getByTestId("input-disable-code"), { target: { value: "abc" } })
    fireEvent.click(screen.getByTestId("btn-confirm-disable"))
    expect(screen.getByTestId("2fa-disable-error")).toBeInTheDocument()
  })
})
