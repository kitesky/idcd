import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, fireEvent, waitFor } from "@testing-library/react"
import "@testing-library/jest-dom"
import ShareResultButton from "../ShareResultButton"
import type { ProbeTaskResult } from "@/lib/api"

const mockSave = vi.fn()
vi.mock("@/lib/probe-share", () => ({
  saveProbeReport: (...args: unknown[]) => mockSave(...args),
  shareUrlFor: (id: string) => `https://example.test/r/${id}`,
}))

function makeTask(status: ProbeTaskResult["status"]): ProbeTaskResult {
  return {
    task_id: "task_abc",
    status,
    result: { node_id: "node1", success: true, duration_ms: 12 },
    created_at: "2026-05-16T10:00:00Z",
  }
}

describe("ShareResultButton", () => {
  beforeEach(() => {
    mockSave.mockReset()
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText: vi.fn().mockResolvedValue(undefined) },
      configurable: true,
    })
  })

  it("is disabled until the task reaches a terminal state", () => {
    render(
      <ShareResultButton
        tool="ping"
        target="example.com"
        taskResult={makeTask("running")}
      />,
    )
    expect(screen.getByTestId("share-result-button")).toBeDisabled()
  })

  it("saves the snapshot and copies the share URL on first click", async () => {
    mockSave.mockResolvedValue("rpt_xyz")
    render(
      <ShareResultButton
        tool="http"
        target="https://idcd.com"
        params={{ method: "GET" }}
        taskResult={makeTask("completed")}
      />,
    )
    fireEvent.click(screen.getByTestId("share-result-button"))

    await waitFor(() => {
      expect(mockSave).toHaveBeenCalledTimes(1)
    })
    const arg = mockSave.mock.calls[0]![0] as Record<string, unknown>
    expect(arg.type).toBe("single")
    expect(arg.tool).toBe("http")
    expect(arg.target).toBe("https://idcd.com")
    expect(arg.taskId).toBe("task_abc")

    await waitFor(() => {
      expect(navigator.clipboard.writeText).toHaveBeenCalledWith(
        "https://example.test/r/rpt_xyz",
      )
    })
    expect(screen.getByText("链接已复制")).toBeInTheDocument()
  })

  it("re-copies without re-saving on subsequent clicks", async () => {
    mockSave.mockResolvedValue("rpt_xyz")
    render(
      <ShareResultButton
        tool="dns"
        target="idcd.com"
        taskResult={makeTask("completed")}
      />,
    )
    const btn = screen.getByTestId("share-result-button")
    fireEvent.click(btn)
    await waitFor(() => expect(mockSave).toHaveBeenCalledTimes(1))

    // wait for the "copied" badge to fade so the button is interactive again
    await waitFor(
      () => expect(screen.queryByText("链接已复制")).not.toBeInTheDocument(),
      { timeout: 2500 },
    )

    fireEvent.click(btn)
    await waitFor(() =>
      expect(navigator.clipboard.writeText).toHaveBeenCalledTimes(2),
    )
    expect(mockSave).toHaveBeenCalledTimes(1) // not re-saved
  })

  it("surfaces save failures inline", async () => {
    mockSave.mockResolvedValue(null)
    render(
      <ShareResultButton
        tool="traceroute"
        target="1.1.1.1"
        taskResult={makeTask("completed")}
      />,
    )
    fireEvent.click(screen.getByTestId("share-result-button"))
    await waitFor(() => {
      expect(screen.getByText("保存失败，请重试")).toBeInTheDocument()
    })
    expect(navigator.clipboard.writeText).not.toHaveBeenCalled()
  })
})
