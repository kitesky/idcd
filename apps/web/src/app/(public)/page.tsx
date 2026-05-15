"use client"

import {
  Globe2,
  Activity,
  Clock,
  TrendingUp,
  Globe,
  Zap,
  Shield,
} from "lucide-react"
import { Button, Card, CardContent, CardDescription, CardHeader, CardTitle, Badge } from "@/components/ui"
import { HeroSearch } from "@/components/hero-search"

const features = [
  {
    icon: Globe,
    title: "全球节点覆盖",
    description: "100+ 节点，中国大陆/香港/欧美/东南亚全覆盖",
  },
  {
    icon: Zap,
    title: "实时多地并发",
    description: "同时从多个节点发起检测，秒级返回结果",
  },
  {
    icon: Shield,
    title: "SSL/安全检测",
    description: "证书链验证、到期提醒、安全头检测",
  },
]

const stats = [
  {
    icon: Globe2,
    label: "监测节点",
    value: "100+",
  },
  {
    icon: Clock,
    label: "平均延迟",
    value: "10ms",
  },
  {
    icon: TrendingUp,
    label: "可用率",
    value: "99.9%",
  },
  {
    icon: Activity,
    label: "工具",
    value: "50+",
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
      <section className="py-20 lg:py-32">
        <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
          <div className="grid grid-cols-1 gap-12 lg:grid-cols-3">
            {features.map((feature) => {
              const Icon = feature.icon
              return (
                <Card key={feature.title} className="text-center">
                  <CardHeader>
                    <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-lg bg-primary/10">
                      <Icon className="h-6 w-6 text-primary" />
                    </div>
                    <CardTitle className="text-xl">{feature.title}</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <CardDescription className="text-base">
                      {feature.description}
                    </CardDescription>
                  </CardContent>
                </Card>
              )
            })}
          </div>
        </div>
      </section>

      {/* Node Map Preview Section */}
      <section className="py-20 lg:py-32 bg-muted/50">
        <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
          <div className="text-center">
            <h2 className="text-3xl font-bold tracking-tight text-foreground sm:text-4xl">
              覆盖全球的节点网络
            </h2>

            {/* Stats Grid */}
            <div className="mx-auto mt-16 grid max-w-2xl grid-cols-2 gap-8 sm:grid-cols-4">
              {stats.map((stat) => {
                const Icon = stat.icon
                return (
                  <Card key={stat.label} className="p-6">
                    <div className="flex flex-col items-center">
                      <Icon className="h-8 w-8 text-primary mb-4" />
                      <div className="text-2xl font-bold text-foreground">
                        {stat.value}
                      </div>
                      <div className="text-sm text-muted-foreground">
                        {stat.label}
                      </div>
                    </div>
                  </Card>
                )
              })}
            </div>

            {/* CTA */}
            <div className="mt-10">
              <a
                href="/nodes"
                className="inline-flex items-center text-primary hover:text-primary/80 transition-colors"
              >
                查看所有节点 →
              </a>
            </div>
          </div>
        </div>
      </section>

      {/* Tools Section */}
      <section className="py-20 lg:py-32">
        <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
          <div className="text-center mb-16">
            <h2 className="text-3xl font-bold tracking-tight text-foreground sm:text-4xl">
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