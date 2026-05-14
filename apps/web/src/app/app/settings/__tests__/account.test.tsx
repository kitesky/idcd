import { describe, it, expect, vi } from "vitest"
import { render, screen, fireEvent, waitFor } from "@testing-library/react"
import { AccountClient } from "../account/account-client"

// Mock next/navigation
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
}))

describe("AccountClient", () => {
  it("renders the account page container", () => {
    render(<AccountClient />)
    expect(screen.getByTestId("account-page")).toBeInTheDocument()
  })

  it("renders the password card with all three inputs", () => {
    render(<AccountClient />)
    expect(screen.getByTestId("password-card")).toBeInTheDocument()
    expect(screen.getByTestId("input-current-password")).toBeInTheDocument()
    expect(screen.getByTestId("input-new-password")).toBeInTheDocument()
    expect(screen.getByTestId("input-confirm-password")).toBeInTheDocument()
  })

  it("renders the save password button", () => {
    render(<AccountClient />)
    expect(screen.getByTestId("btn-save-password")).toBeInTheDocument()
  })

  it("renders the 2FA card with 'secondary' status badge", () => {
    render(<AccountClient />)
    const badge = screen.getByTestId("2fa-status-badge")
    expect(badge).toBeInTheDocument()
    expect(badge.textContent).toContain("未启用")
  })

  it("renders the enable 2FA button", () => {
    render(<AccountClient />)
    expect(screen.getByTestId("btn-enable-2fa")).toBeInTheDocument()
  })

  it("shows 2FA dialog when enable button is clicked", () => {
    render(<AccountClient />)
    const btn = screen.getByTestId("btn-enable-2fa")
    fireEvent.click(btn)
    expect(screen.getByTestId("2fa-dialog")).toBeInTheDocument()
    expect(screen.getByText(/两步验证功能即将上线/)).toBeInTheDocument()
  })

  it("closes 2FA dialog when confirm button is clicked", async () => {
    render(<AccountClient />)
    fireEvent.click(screen.getByTestId("btn-enable-2fa"))
    expect(screen.getByTestId("2fa-dialog")).toBeInTheDocument()
    fireEvent.click(screen.getByTestId("btn-2fa-dialog-close"))
    await waitFor(() => {
      expect(screen.queryByTestId("2fa-dialog")).not.toBeInTheDocument()
    })
  })

  it("renders the danger zone card with destructive border", () => {
    render(<AccountClient />)
    const card = screen.getByTestId("danger-zone-card")
    expect(card).toBeInTheDocument()
    expect(card.className).toContain("border-destructive")
  })

  it("renders the delete account button in danger zone", () => {
    render(<AccountClient />)
    expect(screen.getByTestId("btn-delete-account")).toBeInTheDocument()
  })

  it("shows confirmation panel with email input on delete button click", () => {
    render(<AccountClient />)
    fireEvent.click(screen.getByTestId("btn-delete-account"))
    expect(screen.getByTestId("delete-confirm-panel")).toBeInTheDocument()
    expect(screen.getByTestId("input-delete-email")).toBeInTheDocument()
    expect(screen.getByTestId("btn-confirm-delete")).toBeInTheDocument()
    expect(screen.getByTestId("btn-cancel-delete")).toBeInTheDocument()
  })

  it("shows error when submitted with wrong email", async () => {
    render(<AccountClient />)
    fireEvent.click(screen.getByTestId("btn-delete-account"))
    const input = screen.getByTestId("input-delete-email")
    fireEvent.change(input, { target: { value: "wrong@example.com" } })
    fireEvent.click(screen.getByTestId("btn-confirm-delete"))
    await waitFor(() => {
      expect(screen.getByTestId("delete-error")).toBeInTheDocument()
      expect(screen.getByText(/邮箱地址不匹配/)).toBeInTheDocument()
    })
  })

  it("cancels delete and hides confirmation panel", () => {
    render(<AccountClient />)
    fireEvent.click(screen.getByTestId("btn-delete-account"))
    expect(screen.getByTestId("delete-confirm-panel")).toBeInTheDocument()
    fireEvent.click(screen.getByTestId("btn-cancel-delete"))
    expect(screen.queryByTestId("delete-confirm-panel")).not.toBeInTheDocument()
    expect(screen.getByTestId("btn-delete-account")).toBeInTheDocument()
  })

  it("confirm delete button is disabled when email input is empty", () => {
    render(<AccountClient />)
    fireEvent.click(screen.getByTestId("btn-delete-account"))
    const confirmBtn = screen.getByTestId("btn-confirm-delete")
    expect(confirmBtn).toBeDisabled()
  })

  it("all password inputs are type=password", () => {
    render(<AccountClient />)
    expect(screen.getByTestId("input-current-password")).toHaveAttribute(
      "type",
      "password"
    )
    expect(screen.getByTestId("input-new-password")).toHaveAttribute(
      "type",
      "password"
    )
    expect(screen.getByTestId("input-confirm-password")).toHaveAttribute(
      "type",
      "password"
    )
  })
})
