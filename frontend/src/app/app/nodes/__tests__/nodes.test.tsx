import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, waitFor, fireEvent } from "@testing-library/react"
import "@testing-library/jest-dom"

vi.mock("next/navigation", () => ({
  usePathname: () => "/app/nodes",
  useRouter: () => ({ replace: vi.fn() }),
}))

vi.mock("next/link", () => ({
  default: ({
    children,
    href,
    ...rest
  }: {
    children: React.ReactNode
    href: string
    [key: string]: unknown
  }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}))

vi.mock("@/lib/api", () => ({ apiRequest: vi.fn() }))

vi.mock("next-intl", () => ({
  useTranslations: (ns: string) => (key: string) => {
    const translations: Record<string, string> = {
      "nodes.myApplications.fetchFailed": "无法获取节点申请列表",
      "nodes.myApplications.submitSuccess": "申请已提交，我们将在 1-3 个工作日内完成审核",
      "nodes.myApplications.title": "我的节点申请",
      "nodes.myApplications.desc": "管理你提交的社区节点申请",
      "nodes.myApplications.applyNew": "申请新节点",
      "nodes.myApplications.empty": "暂无节点申请，点击右上角「申请新节点」开始贡献",
      "nodes.myApplications.table.ip": "节点 IP",
      "nodes.myApplications.table.country": "国家",
      "nodes.myApplications.table.status": "状态",
      "nodes.myApplications.table.submittedAt": "提交时间",
      "nodes.myApplications.statusLabel.pending": "审核中",
      "nodes.myApplications.statusLabel.probation": "试用中",
      "nodes.myApplications.statusLabel.active": "已激活",
      "nodes.myApplications.statusLabel.rejected": "已拒绝",
      "nodes.contribute.heartbeatPoints": "+1 积分",
      "nodes.contribute.activationBonus": "+200 积分",
      "nodes.contribute.desc": "贡献社区节点即可获得积分：每次心跳 +1 积分，节点激活奖励 +200 积分。",
      "nodes.contribute.hint": "审核通过后，按节点安装指南完成部署，节点上线后自动开始计入积分。",
      "nodes.deploy.title": "您有已批准的节点，请按以下步骤完成部署",
      "nodes.deploy.step1": "1. 下载并安装 Agent",
      "nodes.deploy.step2": "2. 配置环境变量",
      "nodes.deploy.step3": "3. 启动 Agent",
      "nodes.deploy.enrollTokenHint": "注册令牌需联系管理员获取。",
      "nodes.apply2.dialogTitle": "申请贡献社区节点",
      "nodes.apply2.dialogDesc": "填写节点信息后提交审核，我们将在 1-3 个工作日内完成审核。",
      "nodes.apply2.fields.ipAddress": "IP 地址",
      "nodes.apply2.fields.ipRequired": "请填写 IP 地址",
      "nodes.apply2.fields.hostname": "主机名",
      "nodes.apply2.fields.hostnameRequired": "请填写主机名",
      "nodes.apply2.fields.countryCode": "国家代码",
      "nodes.apply2.fields.countryCodeRequired": "请填写国家代码",
      "nodes.apply2.fields.countryCodePlaceholder": "CN / US / SG",
      "nodes.apply2.fields.city": "城市",
      "nodes.apply2.fields.isp": "ISP",
      "nodes.apply2.fields.bandwidth": "带宽（Mbps）",
      "nodes.apply2.fields.motivation": "申请原因",
      "nodes.apply2.fields.motivationPlaceholder": "请简要说明贡献节点的动机（可选）",
      "nodes.apply2.cancel": "取消",
      "nodes.apply2.submitting": "提交中…",
      "nodes.apply2.submit": "提交申请",
    }
    const fullKey = `${ns}.${key}`
    return translations[fullKey] ?? key
  },
}))

import { apiRequest } from "@/lib/api"
import NodesPage from "../page"

const mockApiRequest = apiRequest as ReturnType<typeof vi.fn>

const MOCK_APPLICATIONS = [
  {
    id: "app_001",
    user_id: "u_001",
    hostname: "node-sg-01.example.com",
    ip_address: "1.2.3.4",
    country: "SG",
    city: "Singapore",
    isp: "Tencent Cloud",
    status: "active" as const,
    created_at: "2024-01-15T10:00:00Z",
    updated_at: "2024-01-16T10:00:00Z",
  },
  {
    id: "app_002",
    user_id: "u_001",
    hostname: "node-us-01.example.com",
    ip_address: "5.6.7.8",
    country: "US",
    city: "Los Angeles",
    isp: "AWS",
    status: "pending" as const,
    created_at: "2024-02-10T08:00:00Z",
    updated_at: "2024-02-10T08:00:00Z",
  },
]

describe("NodesPage", () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it("加载时显示 skeleton", () => {
    // Return a never-resolving promise to keep loading state
    mockApiRequest.mockReturnValue(new Promise(() => {}))
    render(<NodesPage />)
    const skeletons = document.querySelectorAll(".animate-pulse, [data-slot='skeleton']")
    // Skeleton elements should be present while loading
    expect(skeletons.length).toBeGreaterThan(0)
  })

  it("显示节点申请列表", async () => {
    mockApiRequest.mockResolvedValue({ data: { applications: MOCK_APPLICATIONS } })
    render(<NodesPage />)
    await waitFor(() => {
      expect(screen.getByText("1.2.3.4")).toBeInTheDocument()
    })
    expect(screen.getByText("5.6.7.8")).toBeInTheDocument()
    expect(screen.getByText("SG")).toBeInTheDocument()
    expect(screen.getByText("US")).toBeInTheDocument()
  })

  it("空状态显示暂无节点申请", async () => {
    mockApiRequest.mockResolvedValue({ data: { applications: [] } })
    render(<NodesPage />)
    await waitFor(() => {
      expect(
        screen.getByText(/暂无节点申请/),
      ).toBeInTheDocument()
    })
  })

  it("点击申请新节点打开 dialog", async () => {
    mockApiRequest.mockResolvedValue({ data: { applications: [] } })
    render(<NodesPage />)
    // Wait for loading to finish
    await waitFor(() => {
      expect(screen.queryByText(/暂无节点申请/)).toBeInTheDocument()
    })
    const applyButton = screen.getByRole("button", { name: /申请新节点/ })
    fireEvent.click(applyButton)
    await waitFor(() => {
      expect(screen.getByText("申请贡献社区节点")).toBeInTheDocument()
    })
  })
})
