import { describe, it, expect, vi } from "vitest"
import { render, screen, fireEvent, waitFor } from "@testing-library/react"
import { APIKeysClient } from "../api-keys/api-keys-client"

// Mock next/navigation (not used directly but imported transitively)
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
}))

describe("APIKeysClient", () => {
  it("renders the api-keys page container", () => {
    render(<APIKeysClient />)
    expect(screen.getByTestId("api-keys-page")).toBeInTheDocument()
  })

  it("renders the api-keys card", () => {
    render(<APIKeysClient />)
    expect(screen.getByTestId("api-keys-card")).toBeInTheDocument()
  })

  it("renders the create API Key button", () => {
    render(<APIKeysClient />)
    expect(screen.getByTestId("btn-create-key")).toBeInTheDocument()
    expect(screen.getByTestId("btn-create-key").textContent).toContain(
      "创建 API Key"
    )
  })

  it("renders the keys table with 2 mock rows", () => {
    render(<APIKeysClient />)
    expect(screen.getByTestId("api-keys-table")).toBeInTheDocument()
    expect(screen.getByTestId("key-row-key_001")).toBeInTheDocument()
    expect(screen.getByTestId("key-row-key_002")).toBeInTheDocument()
  })

  it("shows key names in table rows", () => {
    render(<APIKeysClient />)
    expect(screen.getByText("生产环境")).toBeInTheDocument()
    expect(screen.getByText("CI/CD 流水线")).toBeInTheDocument()
  })

  it("shows key prefixes in table rows", () => {
    render(<APIKeysClient />)
    expect(screen.getByText("sk_live_abc...xyz")).toBeInTheDocument()
    expect(screen.getByText("sk_live_def...uvw")).toBeInTheDocument()
  })

  it("shows '从未使用' badge for key with no last_used_at", () => {
    render(<APIKeysClient />)
    // key_002 has null last_used_at
    const row = screen.getByTestId("key-row-key_002")
    expect(row.textContent).toContain("从未使用")
  })

  it("renders revoke button for each key", () => {
    render(<APIKeysClient />)
    expect(screen.getByTestId("btn-revoke-key_001")).toBeInTheDocument()
    expect(screen.getByTestId("btn-revoke-key_002")).toBeInTheDocument()
  })

  it("shows revoke confirmation panel on revoke button click", () => {
    render(<APIKeysClient />)
    fireEvent.click(screen.getByTestId("btn-revoke-key_001"))
    expect(screen.getByTestId("btn-confirm-revoke-key_001")).toBeInTheDocument()
    expect(screen.getByTestId("btn-cancel-revoke-key_001")).toBeInTheDocument()
  })

  it("cancels revoke and hides confirmation panel", () => {
    render(<APIKeysClient />)
    fireEvent.click(screen.getByTestId("btn-revoke-key_001"))
    fireEvent.click(screen.getByTestId("btn-cancel-revoke-key_001"))
    expect(
      screen.queryByTestId("btn-confirm-revoke-key_001")
    ).not.toBeInTheDocument()
    expect(screen.getByTestId("btn-revoke-key_001")).toBeInTheDocument()
  })

  it("removes key from list after confirming revoke", async () => {
    render(<APIKeysClient />)
    fireEvent.click(screen.getByTestId("btn-revoke-key_001"))
    fireEvent.click(screen.getByTestId("btn-confirm-revoke-key_001"))
    await waitFor(() => {
      expect(screen.queryByTestId("key-row-key_001")).not.toBeInTheDocument()
    })
    // key_002 still present
    expect(screen.getByTestId("key-row-key_002")).toBeInTheDocument()
  })

  it("opens create dialog on button click", () => {
    render(<APIKeysClient />)
    fireEvent.click(screen.getByTestId("btn-create-key"))
    expect(screen.getByTestId("create-key-dialog")).toBeInTheDocument()
    expect(screen.getByTestId("input-key-name")).toBeInTheDocument()
  })

  it("create submit button is disabled when key name is empty", () => {
    render(<APIKeysClient />)
    fireEvent.click(screen.getByTestId("btn-create-key"))
    expect(screen.getByTestId("btn-submit-create")).toBeDisabled()
  })

  it("shows new key value after creation and hides form", async () => {
    render(<APIKeysClient />)
    fireEvent.click(screen.getByTestId("btn-create-key"))
    const input = screen.getByTestId("input-key-name")
    fireEvent.change(input, { target: { value: "测试 Key" } })
    fireEvent.click(screen.getByTestId("btn-submit-create"))
    await waitFor(() => {
      expect(screen.getByTestId("new-key-reveal")).toBeInTheDocument()
      expect(screen.getByTestId("new-key-value")).toBeInTheDocument()
      expect(screen.getByTestId("btn-copy-key")).toBeInTheDocument()
      expect(screen.getByTestId("btn-done-create")).toBeInTheDocument()
    })
  })

  it("closes create dialog and adds new key to table after done", async () => {
    render(<APIKeysClient />)
    fireEvent.click(screen.getByTestId("btn-create-key"))
    const input = screen.getByTestId("input-key-name")
    fireEvent.change(input, { target: { value: "新 Key" } })
    fireEvent.click(screen.getByTestId("btn-submit-create"))
    await waitFor(() => {
      expect(screen.getByTestId("btn-done-create")).toBeInTheDocument()
    })
    fireEvent.click(screen.getByTestId("btn-done-create"))
    expect(screen.queryByTestId("create-key-dialog")).not.toBeInTheDocument()
    expect(screen.getByText("新 Key")).toBeInTheDocument()
  })

  it("renders the security note alert", () => {
    render(<APIKeysClient />)
    expect(screen.getByTestId("api-keys-security-note")).toBeInTheDocument()
  })
})
