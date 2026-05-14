import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"
import { render, screen, fireEvent, waitFor, act } from "@testing-library/react"
import { SecurityClient } from "../security/security-client"

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
}))

// Default fetch mock: status=disabled, setup returns secret, verify returns backup codes, disable succeeds
// passkeys list returns one mock passkey by default
function mockFetch(overrides?: Record<string, { ok: boolean; body: unknown }>) {
  const defaults: Record<string, { ok: boolean; body: unknown }> = {
    "/v1/account/2fa/status": { ok: true, body: { enabled: false } },
    "/v1/account/2fa/setup": { ok: true, body: { data: { secret: "TESTSECRET", otpauth_uri: "otpauth://totp/test?secret=TESTSECRET" } } },
    "/v1/account/2fa/verify": { ok: true, body: { data: { backup_codes: ["CODE1234", "CODE5678", "CODE9012", "CODE3456", "CODE7890", "CODE1111", "CODE2222", "CODE3333"] } } },
    "/v1/account/2fa/disable": { ok: true, body: {} },
    "/v1/account/passkeys": {
      ok: true,
      body: {
        data: {
          passkeys: [
            {
              id: "pk_Test1",
              device_name: "MacBook Pro (Touch ID)",
              created_at: new Date("2026-04-10T08:00:00Z").toISOString(),
              last_used_at: new Date("2026-05-14T06:00:00Z").toISOString(),
            },
          ],
        },
      },
    },
  }
  const map = { ...defaults, ...overrides }

  return vi.fn().mockImplementation((url: string) => {
    // Strip API_BASE prefix if present
    const path = url.replace(/^http:\/\/localhost:8080/, "")
    // Exact match first
    const entry = map[path]
    if (entry) {
      return Promise.resolve({
        ok: entry.ok,
        json: () => Promise.resolve(entry.body),
        statusText: entry.ok ? "OK" : "Error",
      })
    }
    // Prefix match for dynamic paths (e.g. DELETE /v1/account/passkeys/:id)
    const prefixEntry = Object.entries(map).find(([k]) => k !== path && path.startsWith(k + "/"))
    if (prefixEntry) {
      const [, e] = prefixEntry
      return Promise.resolve({
        ok: e.ok,
        json: () => Promise.resolve(e.body),
        statusText: e.ok ? "OK" : "Error",
      })
    }
    return Promise.resolve({
      ok: false,
      json: () => Promise.resolve({ message: "Not found" }),
      statusText: "Not Found",
    })
  })
}

describe("SecurityClient", () => {
  let fetchMock: ReturnType<typeof mockFetch>

  beforeEach(() => {
    fetchMock = mockFetch()
    vi.stubGlobal("fetch", fetchMock)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it("renders the security page container", async () => {
    await act(async () => { render(<SecurityClient />) })
    expect(screen.getByTestId("security-page")).toBeInTheDocument()
  })

  it("renders the 2FA card with disabled status badge", async () => {
    await act(async () => { render(<SecurityClient />) })
    const badge = screen.getByTestId("2fa-status-badge")
    expect(badge).toBeInTheDocument()
    expect(badge.textContent).toContain("未启用")
  })

  it("shows enabled badge when status API returns enabled=true", async () => {
    fetchMock = mockFetch({ "/v1/account/2fa/status": { ok: true, body: { enabled: true } } })
    vi.stubGlobal("fetch", fetchMock)
    await act(async () => { render(<SecurityClient />) })
    await waitFor(() => {
      expect(screen.getByTestId("2fa-status-badge").textContent).toContain("已启用")
    })
  })

  it("renders the enable 2FA button when disabled", async () => {
    await act(async () => { render(<SecurityClient />) })
    expect(screen.getByTestId("btn-enable-2fa")).toBeInTheDocument()
  })

  it("opens setup dialog step 1 when enable button clicked", async () => {
    await act(async () => { render(<SecurityClient />) })
    await act(async () => { fireEvent.click(screen.getByTestId("btn-enable-2fa")) })
    await waitFor(() => {
      expect(screen.getByTestId("2fa-setup-dialog")).toBeInTheDocument()
      expect(screen.getByTestId("2fa-qr-image")).toBeInTheDocument()
      expect(screen.getByTestId("2fa-secret")).toBeInTheDocument()
    })
    expect(screen.getByTestId("2fa-secret").textContent).toBe("TESTSECRET")
  })

  it("advances to step 2 when I scanned button clicked", async () => {
    await act(async () => { render(<SecurityClient />) })
    await act(async () => { fireEvent.click(screen.getByTestId("btn-enable-2fa")) })
    await waitFor(() => expect(screen.getByTestId("2fa-qr-image")).toBeInTheDocument())
    await act(async () => { fireEvent.click(screen.getByTestId("btn-scanned")) })
    expect(screen.getByTestId("input-totp-code")).toBeInTheDocument()
  })

  it("shows error when code is not 6 digits on verify", async () => {
    await act(async () => { render(<SecurityClient />) })
    await act(async () => { fireEvent.click(screen.getByTestId("btn-enable-2fa")) })
    await waitFor(() => expect(screen.getByTestId("btn-scanned")).toBeInTheDocument())
    await act(async () => { fireEvent.click(screen.getByTestId("btn-scanned")) })
    fireEvent.change(screen.getByTestId("input-totp-code"), { target: { value: "123" } })
    await act(async () => { fireEvent.click(screen.getByTestId("btn-verify-code")) })
    expect(screen.getByTestId("2fa-code-error")).toBeInTheDocument()
  })

  it("advances to step 3 with backup codes when valid 6-digit code entered", async () => {
    await act(async () => { render(<SecurityClient />) })
    await act(async () => { fireEvent.click(screen.getByTestId("btn-enable-2fa")) })
    await waitFor(() => expect(screen.getByTestId("btn-scanned")).toBeInTheDocument())
    await act(async () => { fireEvent.click(screen.getByTestId("btn-scanned")) })
    fireEvent.change(screen.getByTestId("input-totp-code"), { target: { value: "123456" } })
    await act(async () => { fireEvent.click(screen.getByTestId("btn-verify-code")) })
    await waitFor(() => expect(screen.getByTestId("backup-codes-grid")).toBeInTheDocument())
  })

  it("shows enabled badge after completing setup flow", async () => {
    await act(async () => { render(<SecurityClient />) })
    await act(async () => { fireEvent.click(screen.getByTestId("btn-enable-2fa")) })
    await waitFor(() => expect(screen.getByTestId("btn-scanned")).toBeInTheDocument())
    await act(async () => { fireEvent.click(screen.getByTestId("btn-scanned")) })
    fireEvent.change(screen.getByTestId("input-totp-code"), { target: { value: "654321" } })
    await act(async () => { fireEvent.click(screen.getByTestId("btn-verify-code")) })
    await waitFor(() => expect(screen.getByTestId("btn-finish-2fa")).toBeInTheDocument())
    await act(async () => { fireEvent.click(screen.getByTestId("btn-finish-2fa")) })
    await waitFor(() => {
      const badge = screen.getByTestId("2fa-status-badge")
      expect(badge.textContent).toContain("已启用")
    })
  })

  it("shows disable dialog when disable button clicked", async () => {
    await act(async () => { render(<SecurityClient />) })
    await act(async () => { fireEvent.click(screen.getByTestId("btn-enable-2fa")) })
    await waitFor(() => expect(screen.getByTestId("btn-scanned")).toBeInTheDocument())
    await act(async () => { fireEvent.click(screen.getByTestId("btn-scanned")) })
    fireEvent.change(screen.getByTestId("input-totp-code"), { target: { value: "654321" } })
    await act(async () => { fireEvent.click(screen.getByTestId("btn-verify-code")) })
    await waitFor(() => expect(screen.getByTestId("btn-finish-2fa")).toBeInTheDocument())
    await act(async () => { fireEvent.click(screen.getByTestId("btn-finish-2fa")) })
    await waitFor(() => expect(screen.getByTestId("btn-disable-2fa")).toBeInTheDocument())
    await act(async () => { fireEvent.click(screen.getByTestId("btn-disable-2fa")) })
    expect(screen.getByTestId("2fa-disable-dialog")).toBeInTheDocument()
  })

  it("shows error in disable dialog for non-6-digit code", async () => {
    await act(async () => { render(<SecurityClient />) })
    await act(async () => { fireEvent.click(screen.getByTestId("btn-enable-2fa")) })
    await waitFor(() => expect(screen.getByTestId("btn-scanned")).toBeInTheDocument())
    await act(async () => { fireEvent.click(screen.getByTestId("btn-scanned")) })
    fireEvent.change(screen.getByTestId("input-totp-code"), { target: { value: "654321" } })
    await act(async () => { fireEvent.click(screen.getByTestId("btn-verify-code")) })
    await waitFor(() => expect(screen.getByTestId("btn-finish-2fa")).toBeInTheDocument())
    await act(async () => { fireEvent.click(screen.getByTestId("btn-finish-2fa")) })
    await waitFor(() => expect(screen.getByTestId("btn-disable-2fa")).toBeInTheDocument())
    await act(async () => { fireEvent.click(screen.getByTestId("btn-disable-2fa")) })
    fireEvent.change(screen.getByTestId("input-disable-code"), { target: { value: "abc" } })
    await act(async () => { fireEvent.click(screen.getByTestId("btn-confirm-disable")) })
    expect(screen.getByTestId("2fa-disable-error")).toBeInTheDocument()
  })

  it("shows API error in verify step when server rejects code", async () => {
    fetchMock = mockFetch({ "/v1/account/2fa/verify": { ok: false, body: { message: "验证码错误" } } })
    vi.stubGlobal("fetch", fetchMock)
    await act(async () => { render(<SecurityClient />) })
    await act(async () => { fireEvent.click(screen.getByTestId("btn-enable-2fa")) })
    await waitFor(() => expect(screen.getByTestId("btn-scanned")).toBeInTheDocument())
    await act(async () => { fireEvent.click(screen.getByTestId("btn-scanned")) })
    fireEvent.change(screen.getByTestId("input-totp-code"), { target: { value: "123456" } })
    await act(async () => { fireEvent.click(screen.getByTestId("btn-verify-code")) })
    await waitFor(() => expect(screen.getByTestId("2fa-code-error")).toBeInTheDocument())
  })

  it("shows API error in disable dialog when server rejects", async () => {
    // First enable 2FA successfully
    await act(async () => { render(<SecurityClient />) })
    await act(async () => { fireEvent.click(screen.getByTestId("btn-enable-2fa")) })
    await waitFor(() => expect(screen.getByTestId("btn-scanned")).toBeInTheDocument())
    await act(async () => { fireEvent.click(screen.getByTestId("btn-scanned")) })
    fireEvent.change(screen.getByTestId("input-totp-code"), { target: { value: "654321" } })
    await act(async () => { fireEvent.click(screen.getByTestId("btn-verify-code")) })
    await waitFor(() => expect(screen.getByTestId("btn-finish-2fa")).toBeInTheDocument())
    await act(async () => { fireEvent.click(screen.getByTestId("btn-finish-2fa")) })
    await waitFor(() => expect(screen.getByTestId("btn-disable-2fa")).toBeInTheDocument())

    // Now mock disable to fail
    vi.stubGlobal("fetch", mockFetch({ "/v1/account/2fa/disable": { ok: false, body: { message: "验证码错误" } } }))

    await act(async () => { fireEvent.click(screen.getByTestId("btn-disable-2fa")) })
    fireEvent.change(screen.getByTestId("input-disable-code"), { target: { value: "999999" } })
    await act(async () => { fireEvent.click(screen.getByTestId("btn-confirm-disable")) })
    await waitFor(() => expect(screen.getByTestId("2fa-disable-error")).toBeInTheDocument())
  })

  it("renders the Passkey card", async () => {
    await act(async () => { render(<SecurityClient />) })
    expect(screen.getByTestId("passkey-card")).toBeInTheDocument()
  })

  it("renders the 添加 Passkey button", async () => {
    await act(async () => { render(<SecurityClient />) })
    expect(screen.getByTestId("btn-add-passkey")).toBeInTheDocument()
  })

  it("shows loading skeleton while fetching passkeys", async () => {
    // Delay the passkeys response so loading state is observable
    const slowFetch = vi.fn().mockImplementation((url: string) => {
      const path = url.replace(/^http:\/\/localhost:8080/, "")
      if (path === "/v1/account/passkeys") {
        return new Promise((resolve) =>
          setTimeout(() =>
            resolve({
              ok: true,
              json: () => Promise.resolve({ data: { passkeys: [] } }),
              statusText: "OK",
            }),
            100
          )
        )
      }
      return fetchMock(url)
    })
    vi.stubGlobal("fetch", slowFetch)
    render(<SecurityClient />)
    expect(screen.getByTestId("passkey-loading")).toBeInTheDocument()
    await waitFor(() => expect(screen.queryByTestId("passkey-loading")).not.toBeInTheDocument())
  })

  it("renders passkey list items loaded from API", async () => {
    await act(async () => { render(<SecurityClient />) })
    await waitFor(() => {
      expect(screen.getByTestId("passkey-list")).toBeInTheDocument()
      expect(screen.getByText("MacBook Pro (Touch ID)")).toBeInTheDocument()
    })
  })

  it("removes a passkey from list after successful DELETE API call", async () => {
    await act(async () => { render(<SecurityClient />) })
    await waitFor(() => expect(screen.getByTestId("btn-delete-passkey-pk_Test1")).toBeInTheDocument())
    await act(async () => { fireEvent.click(screen.getByTestId("btn-delete-passkey-pk_Test1")) })
    await waitFor(() => {
      expect(screen.queryByText("MacBook Pro (Touch ID)")).not.toBeInTheDocument()
    })
  })

  it("shows error and keeps passkey in list when DELETE API fails", async () => {
    fetchMock = mockFetch({ "/v1/account/passkeys": {
      ok: true,
      body: {
        data: {
          passkeys: [
            {
              id: "pk_Test1",
              device_name: "MacBook Pro (Touch ID)",
              created_at: new Date("2026-04-10T08:00:00Z").toISOString(),
              last_used_at: null,
            },
          ],
        },
      },
    },
    // Override prefix: DELETE /v1/account/passkeys/pk_Test1 → fail
    // We add a more-specific key so prefix match hits it
    "/v1/account/passkeys/pk_Test1": { ok: false, body: { message: "删除失败" } },
    })
    vi.stubGlobal("fetch", fetchMock)
    await act(async () => { render(<SecurityClient />) })
    await waitFor(() => expect(screen.getByTestId("btn-delete-passkey-pk_Test1")).toBeInTheDocument())
    await act(async () => { fireEvent.click(screen.getByTestId("btn-delete-passkey-pk_Test1")) })
    await waitFor(() => {
      expect(screen.getByTestId("passkey-error")).toBeInTheDocument()
      expect(screen.getByText("MacBook Pro (Touch ID)")).toBeInTheDocument()
    })
  })
})
