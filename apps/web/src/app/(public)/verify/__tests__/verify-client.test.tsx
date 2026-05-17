import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, fireEvent, waitFor } from "@testing-library/react"

vi.mock("@/lib/api/verdict", async (orig) => {
  const actual = await orig<typeof import("@/lib/api/verdict")>()
  return {
    ...actual,
    verifyPdf: vi.fn(),
  }
})

import { verifyPdf } from "@/lib/api/verdict"
import { VerifyClient } from "../verify-client"

const mockVerify = vi.mocked(verifyPdf)

function makePdf(name = "report.pdf", size = 1024): File {
  const bytes = new Uint8Array(size)
  return new File([bytes], name, { type: "application/pdf" })
}

describe("VerifyClient", () => {
  beforeEach(() => {
    mockVerify.mockReset()
  })

  it("renders the upload form", () => {
    render(<VerifyClient />)
    expect(screen.getByTestId("verify-form")).toBeInTheDocument()
    expect(screen.getByTestId("verify-file-input")).toBeInTheDocument()
    expect(screen.getByTestId("verify-submit-btn")).toBeDisabled()
  })

  it("shows file name after selection and enables submit", async () => {
    render(<VerifyClient />)
    const input = screen.getByTestId("verify-file-input") as HTMLInputElement
    fireEvent.change(input, { target: { files: [makePdf()] } })
    await waitFor(() => {
      expect(screen.getByTestId("verify-file-name")).toHaveTextContent("report.pdf")
    })
    expect(screen.getByTestId("verify-submit-btn")).not.toBeDisabled()
  })

  it("rejects files exceeding the upload cap", async () => {
    render(<VerifyClient />)
    const giant = makePdf("huge.pdf", 21 * 1024 * 1024)
    const input = screen.getByTestId("verify-file-input") as HTMLInputElement
    fireEvent.change(input, { target: { files: [giant] } })
    await waitFor(() => {
      expect(screen.getByTestId("verify-error")).toBeInTheDocument()
    })
  })

  it("renders the verify result and legal disclaimer on success", async () => {
    mockVerify.mockResolvedValueOnce({
      valid: true,
      signature_chain: "CN=idcd-evidence-2026Q2",
      public_key_fingerprint: "ab:cd:ef",
      signed_at: "2026-05-01T00:00:00Z",
      tsa_provider: "digicert",
      content_hash: "sha256:deadbeef",
      report_type: "observation_only",
      legal_disclaimer:
        "本报告为 idcd 提供的一手观测数据,不构成司法鉴定结论。",
    })

    render(<VerifyClient />)
    fireEvent.change(screen.getByTestId("verify-file-input"), {
      target: { files: [makePdf()] },
    })
    fireEvent.click(screen.getByTestId("verify-submit-btn"))

    await waitFor(() => {
      expect(screen.getByTestId("verify-result")).toBeInTheDocument()
    })
    expect(screen.getByTestId("verify-result-valid")).toBeInTheDocument()
    expect(screen.getByTestId("result-report-type")).toHaveTextContent("observation_only")
    // Disclaimer rendered verbatim (D-Concern1)
    expect(screen.getByTestId("verify-disclaimer")).toHaveTextContent(
      "本报告为 idcd 提供的一手观测数据,不构成司法鉴定结论。",
    )
  })

  it("renders the invalid alert when valid=false", async () => {
    mockVerify.mockResolvedValueOnce({
      valid: false,
      signature_chain: "",
      public_key_fingerprint: "",
      signed_at: "",
      tsa_provider: "",
      content_hash: "",
      report_type: "observation_only",
      legal_disclaimer: "声明",
    })

    render(<VerifyClient />)
    fireEvent.change(screen.getByTestId("verify-file-input"), {
      target: { files: [makePdf()] },
    })
    fireEvent.click(screen.getByTestId("verify-submit-btn"))

    await waitFor(() => {
      expect(screen.getByTestId("verify-result-invalid")).toBeInTheDocument()
    })
  })

  it("surfaces backend errors in an alert", async () => {
    mockVerify.mockRejectedValueOnce(new Error("invalid pdf"))

    render(<VerifyClient />)
    fireEvent.change(screen.getByTestId("verify-file-input"), {
      target: { files: [makePdf()] },
    })
    fireEvent.click(screen.getByTestId("verify-submit-btn"))

    await waitFor(() => {
      expect(screen.getByTestId("verify-error")).toBeInTheDocument()
    })
    expect(screen.getByText(/invalid pdf/)).toBeInTheDocument()
  })
})
