import { describe, it, expect, vi } from "vitest"
import { render, screen } from "@testing-library/react"
import "@testing-library/jest-dom"
import ApplyPage from "../page"

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
}))

describe("NodeApplyPage", () => {
  it("渲染不崩溃", () => {
    const { container } = render(<ApplyPage />)
    expect(container.firstChild).toBeTruthy()
  })

  it("显示页面标题", () => {
    render(<ApplyPage />)
    expect(screen.getByText("贡献节点，赚取积分")).toBeInTheDocument()
  })

  it("显示步骤说明", () => {
    render(<ApplyPage />)
    expect(screen.getByTestId("steps")).toBeInTheDocument()
    expect(screen.getAllByText("提交申请").length).toBeGreaterThanOrEqual(1)
    expect(screen.getByText("14 天观察期")).toBeInTheDocument()
    expect(screen.getByText("加入全球节点池")).toBeInTheDocument()
  })

  it("渲染服务器主机名字段", () => {
    render(<ApplyPage />)
    expect(screen.getByTestId("input-hostname")).toBeInTheDocument()
  })

  it("渲染 IP 地址字段", () => {
    render(<ApplyPage />)
    expect(screen.getByTestId("input-ip-address")).toBeInTheDocument()
  })

  it("渲染国家选择字段", () => {
    render(<ApplyPage />)
    expect(screen.getByTestId("select-country")).toBeInTheDocument()
  })

  it("渲染提交按钮", () => {
    render(<ApplyPage />)
    expect(screen.getByTestId("submit-button")).toBeInTheDocument()
  })

  it("提交按钮文字正确", () => {
    render(<ApplyPage />)
    expect(screen.getByTestId("submit-button")).toHaveTextContent("提交申请")
  })
})
