import { render, screen } from "@testing-library/react"
import { describe, it, expect } from "vitest"
import APIReferencePage from "../page"
import { API_GROUPS } from "../api-reference-data"

describe("APIReferencePage", () => {
  it("renders without crashing", () => {
    render(<APIReferencePage />)
    // The page heading should be present
    expect(screen.getByRole("heading", { name: /API 参考/ })).toBeInTheDocument()
  })

  it("renders the base URL in the description", () => {
    render(<APIReferencePage />)
    expect(screen.getByText(/api\.idcd\.com\/v1/)).toBeInTheDocument()
  })

  it("renders all group headings", () => {
    render(<APIReferencePage />)
    for (const group of API_GROUPS) {
      // Each group has an h2 heading
      expect(
        screen.getByRole("heading", { name: group.label, level: 2 })
      ).toBeInTheDocument()
    }
  })

  it("renders expected group headings: 拨测, 网络信息, 账号, 监控, 告警, 计费, 节点", () => {
    render(<APIReferencePage />)
    const expectedGroups = ["拨测", "网络信息", "账号", "监控", "告警", "计费", "节点"]
    for (const label of expectedGroups) {
      expect(screen.getByRole("heading", { name: label, level: 2 })).toBeInTheDocument()
    }
  })

  it("renders endpoint summaries for all groups", () => {
    render(<APIReferencePage />)
    // Spot-check a few endpoint summaries from each group
    expect(screen.getAllByText(/HTTP 拨测/).length).toBeGreaterThanOrEqual(1)
    expect(screen.getAllByText(/ICMP Ping/).length).toBeGreaterThanOrEqual(1)
    expect(screen.getAllByText(/IP 信息查询/).length).toBeGreaterThanOrEqual(1)
    expect(screen.getAllByText(/SSL 证书查询/).length).toBeGreaterThanOrEqual(1)
    expect(screen.getAllByText(/获取监控列表/).length).toBeGreaterThanOrEqual(1)
  })

  it("renders authentication note with bearer token info", () => {
    render(<APIReferencePage />)
    // The auth note contains "Authorization: Bearer" instruction
    expect(screen.getByText(/Authorization: Bearer/)).toBeInTheDocument()
  })

  it("renders the openapi.json link", () => {
    render(<APIReferencePage />)
    const link = screen.getByRole("link", { name: /openapi\.json/ })
    expect(link).toBeInTheDocument()
    expect(link).toHaveAttribute("href", "/v1/openapi.json")
  })

  it("renders '需要鉴权' and '公开' badges", () => {
    render(<APIReferencePage />)
    // Auth-required endpoints exist (e.g., monitor-list)
    const authBadges = screen.getAllByText("需要鉴权")
    expect(authBadges.length).toBeGreaterThan(0)
    // Public endpoints exist (e.g., probe-http)
    const publicBadges = screen.getAllByText("公开")
    expect(publicBadges.length).toBeGreaterThan(0)
  })

  it("renders parameter tables for endpoints with parameters", () => {
    render(<APIReferencePage />)
    // The parameter tables should have headers
    const paramHeaders = screen.getAllByText("名称")
    expect(paramHeaders.length).toBeGreaterThan(0)
  })

  it("renders HTTP method badges", () => {
    render(<APIReferencePage />)
    // GET, POST, PATCH, DELETE badges should all be present
    expect(screen.getAllByText("GET").length).toBeGreaterThan(0)
    expect(screen.getAllByText("POST").length).toBeGreaterThan(0)
    expect(screen.getAllByText("PATCH").length).toBeGreaterThan(0)
    expect(screen.getAllByText("DELETE").length).toBeGreaterThan(0)
  })

  it("renders sidebar navigation with group links", () => {
    render(<APIReferencePage />)
    const nav = screen.getByRole("navigation", { name: /API 端点导航/ })
    expect(nav).toBeInTheDocument()
    for (const group of API_GROUPS) {
      // Nav links use group.label text
      const links = screen.getAllByRole("link", { name: group.label })
      expect(links.length).toBeGreaterThanOrEqual(1)
    }
  })
})
