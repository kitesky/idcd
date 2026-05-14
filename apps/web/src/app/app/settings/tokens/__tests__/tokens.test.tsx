import { describe, it, expect, vi } from "vitest"
import { render, screen, fireEvent, waitFor } from "@testing-library/react"
import { TokensClient } from "../tokens-client"

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
}))

describe("TokensClient", () => {
  it("renders the tokens page container", () => {
    render(<TokensClient />)
    expect(screen.getByTestId("tokens-page")).toBeInTheDocument()
  })

  it("renders the tokens card with title", () => {
    render(<TokensClient />)
    expect(screen.getByTestId("tokens-card")).toBeInTheDocument()
    expect(screen.getByText("Personal Access Token")).toBeInTheDocument()
  })

  it("renders the generate new token button", () => {
    render(<TokensClient />)
    const btn = screen.getByTestId("btn-create-token")
    expect(btn).toBeInTheDocument()
    expect(btn.textContent).toContain("生成新 Token")
  })

  it("renders the tokens table with 2 mock rows", () => {
    render(<TokensClient />)
    expect(screen.getByTestId("tokens-table")).toBeInTheDocument()
    expect(screen.getByTestId("token-row-pat_001")).toBeInTheDocument()
    expect(screen.getByTestId("token-row-pat_002")).toBeInTheDocument()
  })

  it("shows token names in table rows", () => {
    render(<TokensClient />)
    expect(screen.getByText("本地 CLI")).toBeInTheDocument()
    expect(screen.getByText("MCP 集成")).toBeInTheDocument()
  })

  it("shows token prefixes in table rows", () => {
    render(<TokensClient />)
    expect(screen.getByText("idcd_pat_a1b2c3d4")).toBeInTheDocument()
    expect(screen.getByText("idcd_pat_e5f6a7b8")).toBeInTheDocument()
  })

  it("shows '永不过期' badge for token with no expires_at", () => {
    render(<TokensClient />)
    const row = screen.getByTestId("token-row-pat_001")
    expect(row.textContent).toContain("永不过期")
  })

  it("renders revoke button for each token", () => {
    render(<TokensClient />)
    expect(screen.getByTestId("btn-revoke-pat_001")).toBeInTheDocument()
    expect(screen.getByTestId("btn-revoke-pat_002")).toBeInTheDocument()
  })

  it("shows revoke confirmation on revoke click", () => {
    render(<TokensClient />)
    fireEvent.click(screen.getByTestId("btn-revoke-pat_001"))
    expect(screen.getByTestId("btn-confirm-revoke-pat_001")).toBeInTheDocument()
    expect(screen.getByTestId("btn-cancel-revoke-pat_001")).toBeInTheDocument()
  })

  it("cancels revoke and hides confirmation", () => {
    render(<TokensClient />)
    fireEvent.click(screen.getByTestId("btn-revoke-pat_001"))
    fireEvent.click(screen.getByTestId("btn-cancel-revoke-pat_001"))
    expect(
      screen.queryByTestId("btn-confirm-revoke-pat_001")
    ).not.toBeInTheDocument()
    expect(screen.getByTestId("btn-revoke-pat_001")).toBeInTheDocument()
  })

  it("removes token from list after confirming revoke", async () => {
    render(<TokensClient />)
    fireEvent.click(screen.getByTestId("btn-revoke-pat_001"))
    fireEvent.click(screen.getByTestId("btn-confirm-revoke-pat_001"))
    await waitFor(() => {
      expect(screen.queryByTestId("token-row-pat_001")).not.toBeInTheDocument()
    })
    expect(screen.getByTestId("token-row-pat_002")).toBeInTheDocument()
  })

  it("opens create dialog on button click", () => {
    render(<TokensClient />)
    fireEvent.click(screen.getByTestId("btn-create-token"))
    expect(screen.getByTestId("create-token-dialog")).toBeInTheDocument()
    expect(screen.getByTestId("input-token-name")).toBeInTheDocument()
  })

  it("create submit button is disabled when name is empty", () => {
    render(<TokensClient />)
    fireEvent.click(screen.getByTestId("btn-create-token"))
    expect(screen.getByTestId("btn-submit-create")).toBeDisabled()
  })

  it("renders scopes checkboxes in create dialog", () => {
    render(<TokensClient />)
    fireEvent.click(screen.getByTestId("btn-create-token"))
    expect(screen.getByTestId("scopes-checkboxes")).toBeInTheDocument()
    expect(
      screen.getByTestId("checkbox-scope-read:monitors")
    ).toBeInTheDocument()
    expect(
      screen.getByTestId("checkbox-scope-write:monitors")
    ).toBeInTheDocument()
  })

  it("renders expiry select in create dialog", () => {
    render(<TokensClient />)
    fireEvent.click(screen.getByTestId("btn-create-token"))
    expect(screen.getByTestId("select-expiry")).toBeInTheDocument()
  })

  it("shows new token value after creation", async () => {
    render(<TokensClient />)
    fireEvent.click(screen.getByTestId("btn-create-token"))
    const input = screen.getByTestId("input-token-name")
    fireEvent.change(input, { target: { value: "测试 Token" } })
    fireEvent.click(screen.getByTestId("btn-submit-create"))
    await waitFor(() => {
      expect(screen.getByTestId("new-token-reveal")).toBeInTheDocument()
      expect(screen.getByTestId("new-token-value")).toBeInTheDocument()
      expect(screen.getByTestId("btn-copy-token")).toBeInTheDocument()
      expect(screen.getByTestId("btn-done-create")).toBeInTheDocument()
    })
  })

  it("new token value starts with idcd_pat_ prefix", async () => {
    render(<TokensClient />)
    fireEvent.click(screen.getByTestId("btn-create-token"))
    const input = screen.getByTestId("input-token-name")
    fireEvent.change(input, { target: { value: "New Token" } })
    fireEvent.click(screen.getByTestId("btn-submit-create"))
    await waitFor(() => {
      expect(screen.getByTestId("new-token-value")).toBeInTheDocument()
    })
    const tokenValue = screen.getByTestId("new-token-value").textContent ?? ""
    expect(tokenValue).toMatch(/^idcd_pat_/)
  })

  it("closes create dialog and adds new token to table after done", async () => {
    render(<TokensClient />)
    fireEvent.click(screen.getByTestId("btn-create-token"))
    const input = screen.getByTestId("input-token-name")
    fireEvent.change(input, { target: { value: "新 Token" } })
    fireEvent.click(screen.getByTestId("btn-submit-create"))
    await waitFor(() => {
      expect(screen.getByTestId("btn-done-create")).toBeInTheDocument()
    })
    fireEvent.click(screen.getByTestId("btn-done-create"))
    expect(screen.queryByTestId("create-token-dialog")).not.toBeInTheDocument()
    expect(screen.getByText("新 Token")).toBeInTheDocument()
  })

  it("renders the security note alert", () => {
    render(<TokensClient />)
    expect(screen.getByTestId("tokens-security-note")).toBeInTheDocument()
  })
})
