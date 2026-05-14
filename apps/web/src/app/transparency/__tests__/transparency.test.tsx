import { render, screen } from "@testing-library/react"
import { describe, it, expect } from "vitest"
import TransparencyPage from "../page"

describe("TransparencyPage", () => {
  it("renders without crashing", () => {
    render(<TransparencyPage />)
    expect(screen.getByText("透明度报告")).toBeInTheDocument()
  })

  it("shows overall system status", () => {
    render(<TransparencyPage />)
    expect(screen.getByTestId("overall-status")).toBeInTheDocument()
    expect(screen.getByText("● 所有系统运行正常")).toBeInTheDocument()
  })

  it("shows KMS card", () => {
    render(<TransparencyPage />)
    expect(screen.getByTestId("kms-card")).toBeInTheDocument()
    expect(screen.getByText("KMS 信任根")).toBeInTheDocument()
  })

  it("shows TSA providers", () => {
    render(<TransparencyPage />)
    expect(screen.getByTestId("tsa-providers")).toBeInTheDocument()
    expect(screen.getByText("DigiCert")).toBeInTheDocument()
    expect(screen.getByText("GlobalSign")).toBeInTheDocument()
  })

  it("shows node stats", () => {
    render(<TransparencyPage />)
    expect(screen.getByText("节点覆盖")).toBeInTheDocument()
    expect(screen.getByText("总节点")).toBeInTheDocument()
    expect(screen.getByText("活跃节点")).toBeInTheDocument()
    expect(screen.getByText("覆盖地区")).toBeInTheDocument()
  })
})
