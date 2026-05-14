import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, fireEvent, waitFor } from "@testing-library/react"
import { AccountClient } from "../account/account-client"

// Mock next/navigation
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
}))

// Mock the API module
vi.mock("@/lib/api", () => ({
  apiRequest: vi.fn(),
  API_BASE: "http://localhost:8080",
}))

import { apiRequest } from "@/lib/api"
const mockApiRequest = vi.mocked(apiRequest)

const REAL_EMAIL = "real-user@example.com"

function setupDefaultMocks() {
  mockApiRequest.mockImplementation(async (path: string, options?: RequestInit) => {
    const method = options?.method?.toUpperCase() ?? "GET"

    if (path === "/v1/account/profile" && method === "GET") {
      return { data: { email: REAL_EMAIL } }
    }
    if (path === "/v1/account/password" && method === "PATCH") {
      return {}
    }
    if (path === "/v1/account" && method === "DELETE") {
      return {}
    }

    throw new Error(`Unmocked API call: ${method} ${path}`)
  })
}

beforeEach(() => {
  vi.clearAllMocks()
  setupDefaultMocks()
})

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

  it("renders the 2FA button as a link to security settings", () => {
    render(<AccountClient />)
    const btn = screen.getByTestId("btn-enable-2fa")
    expect(btn).toBeInTheDocument()
    expect(btn.closest("a")).toHaveAttribute("href", "/app/settings/security")
  })

  it("2FA section shows '前往安全设置' text", () => {
    render(<AccountClient />)
    expect(screen.getByText("前往安全设置")).toBeInTheDocument()
  })

  it("renders the danger zone card with destructive border", () => {
    render(<AccountClient />)
    const card = screen.getByTestId("danger-zone-card")
    expect(card).toBeInTheDocument()
    expect(card.className).toContain("border-destructive")
  })

  it("shows skeleton while profile is loading, then shows delete button", async () => {
    render(<AccountClient />)
    // Initially shows skeleton
    expect(screen.getByTestId("delete-btn-skeleton")).toBeInTheDocument()
    // After profile loads, shows the real button
    await waitFor(() => {
      expect(screen.getByTestId("btn-delete-account")).toBeInTheDocument()
    })
  })

  it("renders the delete account button in danger zone after profile loads", async () => {
    render(<AccountClient />)
    await waitFor(() => {
      expect(screen.getByTestId("btn-delete-account")).toBeInTheDocument()
    })
  })

  it("shows confirmation panel with email input on delete button click", async () => {
    render(<AccountClient />)
    await waitFor(() => expect(screen.getByTestId("btn-delete-account")).toBeInTheDocument())
    fireEvent.click(screen.getByTestId("btn-delete-account"))
    expect(screen.getByTestId("delete-confirm-panel")).toBeInTheDocument()
    expect(screen.getByTestId("input-delete-email")).toBeInTheDocument()
    expect(screen.getByTestId("btn-confirm-delete")).toBeInTheDocument()
    expect(screen.getByTestId("btn-cancel-delete")).toBeInTheDocument()
  })

  it("shows real email address in the confirmation panel", async () => {
    render(<AccountClient />)
    await waitFor(() => expect(screen.getByTestId("btn-delete-account")).toBeInTheDocument())
    fireEvent.click(screen.getByTestId("btn-delete-account"))
    await waitFor(() => {
      expect(screen.getByTestId("delete-confirm-panel").textContent).toContain(REAL_EMAIL)
    })
  })

  it("shows error when submitted with wrong email", async () => {
    render(<AccountClient />)
    await waitFor(() => expect(screen.getByTestId("btn-delete-account")).toBeInTheDocument())
    fireEvent.click(screen.getByTestId("btn-delete-account"))
    const input = screen.getByTestId("input-delete-email")
    fireEvent.change(input, { target: { value: "wrong@example.com" } })
    fireEvent.click(screen.getByTestId("btn-confirm-delete"))
    await waitFor(() => {
      expect(screen.getByTestId("delete-error")).toBeInTheDocument()
      expect(screen.getByText(/邮箱地址不匹配/)).toBeInTheDocument()
    })
  })

  it("cancels delete and hides confirmation panel", async () => {
    render(<AccountClient />)
    await waitFor(() => expect(screen.getByTestId("btn-delete-account")).toBeInTheDocument())
    fireEvent.click(screen.getByTestId("btn-delete-account"))
    expect(screen.getByTestId("delete-confirm-panel")).toBeInTheDocument()
    fireEvent.click(screen.getByTestId("btn-cancel-delete"))
    expect(screen.queryByTestId("delete-confirm-panel")).not.toBeInTheDocument()
    expect(screen.getByTestId("btn-delete-account")).toBeInTheDocument()
  })

  it("confirm delete button is disabled when email input is empty", async () => {
    render(<AccountClient />)
    await waitFor(() => expect(screen.getByTestId("btn-delete-account")).toBeInTheDocument())
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

  it("calls DELETE /v1/account when confirmed with correct email", async () => {
    const mockPush = vi.fn()
    vi.mocked(await import("next/navigation")).useRouter = () => ({ push: mockPush } as any)
    render(<AccountClient />)
    await waitFor(() => expect(screen.getByTestId("btn-delete-account")).toBeInTheDocument())
    fireEvent.click(screen.getByTestId("btn-delete-account"))
    const input = screen.getByTestId("input-delete-email")
    fireEvent.change(input, { target: { value: REAL_EMAIL } })
    fireEvent.click(screen.getByTestId("btn-confirm-delete"))
    await waitFor(() => {
      expect(mockApiRequest).toHaveBeenCalledWith("/v1/account", { method: "DELETE" })
    })
  })
})
