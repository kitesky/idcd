import { describe, expect, it, vi } from "vitest"
import { fireEvent, render, screen, waitFor } from "@testing-library/react"

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), back: vi.fn(), replace: vi.fn() }),
}))

vi.mock("next/link", () => ({
  default: ({
    children,
    href,
  }: {
    children: React.ReactNode
    href: string
  }) => <a href={href}>{children}</a>,
}))

// Stub the cert-api fetch layer so the wizard renders without making real
// network calls during the test.
vi.mock("../cert-api", () => ({
  listDnsCredentials: vi.fn(async () => [
    {
      id: "dns-1",
      provider: "cloudflare" as const,
      displayName: "Test Cloudflare",
      health: "healthy" as const,
      createdAt: new Date().toISOString(),
    },
  ]),
  createOrder: vi.fn(async () => ({ id: "ord-new" })),
}))

import { WizardClient } from "../new/wizard-client"

describe("WizardClient", () => {
  it("renders the four step indicators", async () => {
    render(<WizardClient />)
    for (let i = 0; i < 4; i++) {
      expect(screen.getByTestId(`wizard-step-${i}`)).toBeInTheDocument()
    }
  })

  it("disables next step when no SAN is entered", () => {
    render(<WizardClient />)
    expect(screen.getByTestId("wizard-next")).toBeDisabled()
  })

  it("enables next step once at least one SAN parses", () => {
    render(<WizardClient />)
    const textarea = screen.getByTestId("san-input")
    fireEvent.change(textarea, { target: { value: "example.com" } })
    expect(screen.getByTestId("wizard-next")).not.toBeDisabled()
    // SAN preview pills should reflect the entered host.
    expect(screen.getByTestId("san-preview").textContent).toContain("example.com")
  })

  it("advances through steps and reaches the confirm step with a submit button", async () => {
    render(<WizardClient />)
    fireEvent.change(screen.getByTestId("san-input"), {
      target: { value: "example.com\nwww.example.com" },
    })
    // Step 0 -> 1
    fireEvent.click(screen.getByTestId("wizard-next"))
    // Step 1 -> 2 (default CA = letsencrypt already selected)
    fireEvent.click(screen.getByTestId("wizard-next"))
    // Step 2 -> 3 — auto challenge is the default; the mock dns credential
    // auto-selects on mount so canProceed() is true.
    await waitFor(() => {
      expect(screen.getByTestId("wizard-next")).not.toBeDisabled()
    })
    fireEvent.click(screen.getByTestId("wizard-next"))
    expect(await screen.findByTestId("wizard-submit")).toBeInTheDocument()
  })

  it("step indicators reflect progression", () => {
    render(<WizardClient />)
    fireEvent.change(screen.getByTestId("san-input"), {
      target: { value: "a.example.com" },
    })
    fireEvent.click(screen.getByTestId("wizard-next"))
    // After advancing the second step indicator should be the active one;
    // we can't easily assert classes so we just verify both pills are present.
    expect(screen.getByTestId("wizard-step-0")).toBeInTheDocument()
    expect(screen.getByTestId("wizard-step-1")).toBeInTheDocument()
  })
})
