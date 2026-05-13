import type { Metadata } from "next"
import Link from "next/link"
import { Button, Card, CardContent, CardHeader, CardTitle } from "@idcd/ui"
import { ArrowLeft, Globe, Zap, Shield, Github, Mail } from "lucide-react"

export const metadata: Metadata = {
  title: "关于 idcd - 全球网络诊断平台",
  description: "idcd 是专业的多节点网络诊断平台，提供全球拨测、DNS/SSL/IP 查询、一键诊断等功能",
}

export default function AboutPage() {
  return (
    <main className="flex-1">
      <div className="mx-auto max-w-4xl px-4 py-12 sm:px-6 lg:px-8">
        {/* 返回按钮 */}
        <div className="mb-8">
          <Link href="/">
            <Button variant="ghost" size="sm" className="gap-2">
              <ArrowLeft className="h-4 w-4" />
              返回首页
            </Button>
          </Link>
        </div>

        {/* 标题 */}
        <div className="mb-12 text-center">
          <h1 className="text-4xl font-bold tracking-tight mb-4">关于 idcd</h1>
          <p className="text-xl text-muted-foreground max-w-2xl mx-auto">
            专业的多节点网络诊断平台，让网络质量可见、可测、可信
          </p>
        </div>

        {/* 核心特性 */}
        <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mb-12">
          <Card>
            <CardHeader>
              <Globe className="h-8 w-8 text-primary mb-2" />
              <CardTitle className="text-lg">全球节点覆盖</CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-sm text-muted-foreground">
                分布于全球多个地区的拨测节点，提供真实的用户视角网络质量数据
              </p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <Zap className="h-8 w-8 text-primary mb-2" />
              <CardTitle className="text-lg">实时诊断</CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-sm text-muted-foreground">
                支持 Ping、HTTP、TCP、Traceroute、DNS、SSL 证书检查等多种诊断工具
              </p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <Shield className="h-8 w-8 text-primary mb-2" />
              <CardTitle className="text-lg">Evidence 证据服务</CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-sm text-muted-foreground">
                基于 KMS 签名的 Verdict 报告，提供可验证的网络质量证据用于合规审计
              </p>
            </CardContent>
          </Card>
        </div>

        {/* 产品简介 */}
        <Card className="mb-8">
          <CardHeader>
            <CardTitle>产品简介</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4 text-muted-foreground">
            <p>
              idcd 是一个专业的网络诊断和监控平台，为开发者、运维团队和企业提供全球多节点拨测服务。
            </p>
            <p>
              我们的核心功能包括：
            </p>
            <ul className="list-disc list-inside space-y-2 ml-4">
              <li><strong>多节点拨测</strong>：从全球多个地理位置同时发起网络诊断，获得全面的网络质量视图</li>
              <li><strong>丰富的诊断工具</strong>：Ping、HTTP 探测、TCP 连接测试、Traceroute 路由跟踪、DNS 查询、SSL 证书检查等</li>
              <li><strong>实时监控</strong>：创建定时监控任务，自动检测服务可用性和性能指标</li>
              <li><strong>一键诊断</strong>：综合多种诊断工具，一次执行获取完整的网络健康报告</li>
              <li><strong>Evidence 证据服务</strong>：通过 KMS 签名的 Verdict 报告，提供可公开验证的网络质量证据</li>
              <li><strong>实用工具集</strong>：JSON 格式化、正则测试、Base64 编解码、时间戳转换、CIDR 计算器等开发者工具</li>
            </ul>
            <p>
              无论您是需要监控全球 CDN 性能、验证 SLA 达成情况、诊断网络故障，还是需要获取可信的网络质量证据，idcd 都能满足您的需求。
            </p>
          </CardContent>
        </Card>

        {/* 技术说明 */}
        <Card className="mb-8">
          <CardHeader>
            <CardTitle>技术架构</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4 text-muted-foreground">
            <p>
              idcd 采用现代化的技术栈构建，确保高性能、高可用性和良好的用户体验：
            </p>
            <ul className="list-disc list-inside space-y-2 ml-4">
              <li><strong>后端</strong>：Go 语言开发，提供高性能的 API 服务和拨测引擎</li>
              <li><strong>前端</strong>：Next.js 16 (App Router) + TypeScript + shadcn/ui，提供流畅的用户界面</li>
              <li><strong>数据库</strong>：PostgreSQL 16，支持多 schema 数据隔离</li>
              <li><strong>缓存</strong>：Redis 7，用于会话管理和高频查询优化</li>
              <li><strong>安全</strong>：KMS 签名系统、TLS 1.3 加密传输、多层安全防护</li>
              <li><strong>节点管理</strong>：全球分布式拨测节点，支持水平扩展</li>
            </ul>
            <p className="mt-4">
              整个系统设计遵循微服务架构理念，各模块职责清晰，便于维护和扩展。
            </p>
          </CardContent>
        </Card>

        {/* 联系方式 */}
        <Card>
          <CardHeader>
            <CardTitle>联系我们</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              <div className="flex items-center gap-3 text-muted-foreground">
                <Mail className="h-5 w-5 text-primary" />
                <div>
                  <p className="text-sm font-medium text-foreground">邮箱</p>
                  <a
                    href="mailto:kite365@gmail.com"
                    className="text-sm text-primary hover:underline"
                  >
                    kite365@gmail.com
                  </a>
                </div>
              </div>

              <div className="flex items-center gap-3 text-muted-foreground">
                <Globe className="h-5 w-5 text-primary" />
                <div>
                  <p className="text-sm font-medium text-foreground">网站</p>
                  <a
                    href="https://idcd.com"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-sm text-primary hover:underline"
                  >
                    https://idcd.com
                  </a>
                </div>
              </div>

              <div className="flex items-center gap-3 text-muted-foreground">
                <Github className="h-5 w-5 text-primary" />
                <div>
                  <p className="text-sm font-medium text-foreground">GitHub</p>
                  <a
                    href="https://github.com/kite365/idcd"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-sm text-primary hover:underline"
                  >
                    https://github.com/kite365/idcd
                  </a>
                </div>
              </div>
            </div>

            <div className="mt-6 pt-6 border-t">
              <p className="text-sm text-muted-foreground">
                欢迎通过邮件或 GitHub Issues 联系我们，无论是产品建议、功能需求还是技术支持，我们都期待听到您的反馈。
              </p>
            </div>
          </CardContent>
        </Card>
      </div>
    </main>
  )
}
