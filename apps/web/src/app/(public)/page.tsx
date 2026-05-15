"use client"

import {
  Activity,
  Globe,
  Code2,
  Monitor,
  BellRing,
  BarChart2,
  Zap,
  Webhook,
} from "lucide-react"
import { Button, Card, CardContent, CardDescription, CardHeader, CardTitle, Badge } from "@/components/ui"
import { HeroSearch } from "@/components/hero-search"

const features = [
  {
    icon: Activity,
    title: "网络质量监控",
    description: "主动探测分布在全球各地方的用户访问中心云、边缘云的网络质量，包括可达性、时延、丢包、抖动等。",
  },
  {
    icon: Globe,
    title: "DNS 监测",
    description: "主动探测 DNS 的可用性、时延，分析诊断解析记录。",
  },
  {
    icon: Code2,
    title: "API 监测",
    description: "主动探测分布在全球各地方的用户访问 HTTP 接口的响应时间、响应码、响应内容正确性。",
  },
  {
    icon: Monitor,
    title: "Web 页面监测",
    description: "从用户视角分析页面整体加载耗时，页面元素从 CDN、源站拉取的时延、可用性，洞察用户体验。",
  },
  {
    icon: BellRing,
    title: "告警",
    description: "支持固定阈值、智能基线的异常检测，发现问题第一时间通知相关人。",
  },
  {
    icon: BarChart2,
    title: "多维分析",
    description: "分析从各地区、各城市、各运营商等访问的网络质量和用户体验。",
  },
  {
    icon: Zap,
    title: "即时拨测",
    description: "立即发起一次探测，即刻查看结果，进行问题诊断和业务验证。",
  },
  {
    icon: Webhook,
    title: "开放 API",
    description: "支持 API 拉取拨测数据，用于故障转移、报告等场景。",
  },
]

const tools = [
  {
    name: "HTTP检测",
    description: "HTTP/HTTPS 响应时间和状态检查",
    href: "/tools/http",
  },
  {
    name: "Ping测试",
    description: "多地 ICMP Ping 连通性测试",
    href: "/tools/ping",
  },
  {
    name: "DNS查询",
    description: "全球 DNS 解析和污染检测",
    href: "/tools/dns",
  },
  {
    name: "SSL检查",
    description: "SSL 证书链和安全配置验证",
    href: "/tools/ssl",
  },
  {
    name: "路由追踪",
    description: "网络路径跟踪和节点分析",
    href: "/tools/traceroute",
  },
  {
    name: "一键诊断",
    description: "综合网络诊断和问题定位",
    href: "/tools/diagnose",
  },
]

export default function HomePage() {
  return (
    <main className="flex-1">
      {/* Hero Section — HeroSearch 独立组件 */}
      <HeroSearch />

      {/* Features Section */}
      <section className="py-12 md:py-16 border-b">
        <div className="mx-auto max-w-screen-xl px-4 sm:px-6 lg:px-8">
          <h2 className="text-center text-2xl font-bold text-foreground mb-8">产品功能</h2>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            {features.map((feature) => {
              const Icon = feature.icon
              return (
                <Card key={feature.title} className="p-6">
                  <div className="flex flex-col items-center text-center">
                    <div className="mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-primary/10">
                      <Icon className="h-6 w-6 text-primary" />
                    </div>
                    <h3 className="text-sm font-semibold text-foreground">{feature.title}</h3>
                  </div>
                </Card>
              )
            })}
          </div>
        </div>
      </section>

      {/* Tools Section */}
      <section className="py-12 md:py-16 bg-muted/30">
        <div className="mx-auto max-w-screen-xl px-4 sm:px-6 lg:px-8">
          <div className="text-center mb-10">
            <h2 className="text-2xl font-bold tracking-tight text-foreground sm:text-3xl">
              常用网络诊断工具
            </h2>
          </div>

          <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-3">
            {tools.map((tool) => (
              <Card key={tool.name} className="group hover:shadow-lg transition-shadow">
                <CardHeader>
                  <div className="flex items-center gap-3">
                    <Badge variant="outline">{tool.name}</Badge>
                  </div>
                  <CardTitle className="text-lg">{tool.name}</CardTitle>
                </CardHeader>
                <CardContent>
                  <CardDescription className="mb-4">
                    {tool.description}
                  </CardDescription>
                  <Button variant="outline" className="w-full">
                    <a href={tool.href} className="block w-full">使用工具</a>
                  </Button>
                </CardContent>
              </Card>
            ))}
          </div>
        </div>
      </section>
    </main>
  )
}