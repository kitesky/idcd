import type { Metadata } from "next"
import Link from "next/link"
import { Button, Alert, AlertDescription } from "@/components/ui"
import { ArrowLeft, AlertTriangle } from "lucide-react"

export const metadata: Metadata = {
  title: "可接受使用政策 (AUP) - idcd",
  alternates: { canonical: "https://idcd.com/aup" },
  robots: { index: false, follow: false },
  description: "idcd 可接受使用政策 - 明确禁止的网络行为和使用规范",
}

export default function AUPPage() {
  return (
    <main className="flex-1">
      <div className="mx-auto max-w-3xl px-4 py-12 sm:px-6 lg:px-8">
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
        <div className="mb-8 border-b pb-8">
          <h1 className="text-4xl font-bold tracking-tight">可接受使用政策 (AUP)</h1>
          <p className="mt-4 text-sm text-muted-foreground">
            最后更新：2026-05-13
          </p>
        </div>

        {/* 警告提示 */}
        <Alert className="mb-8 border-warning">
          <AlertTriangle className="h-4 w-4 text-warning" />
          <AlertDescription className="text-warning">
            违反本政策的行为将导致服务立即暂停或终止，且可能承担法律责任。请务必遵守以下规定。
          </AlertDescription>
        </Alert>

        {/* 内容 */}
        <div className="prose prose-neutral dark:prose-invert max-w-none">
          <section className="mb-8">
            <h2 className="text-2xl font-semibold mb-4">1. 允许用途</h2>
            <p className="text-muted-foreground leading-relaxed mb-4">
              idcd 提供的网络诊断和监控服务应仅用于以下合法目的：
            </p>
            <ul className="list-disc list-inside text-muted-foreground space-y-2 mb-4">
              <li>诊断和监控您自己拥有或已获得授权的网络服务</li>
              <li>进行合法的网络性能测试和可用性检查</li>
              <li>收集网络质量数据用于合规审计或 SLA 验证</li>
              <li>教育和研究目的（需获得相关方授权）</li>
            </ul>
            <p className="text-muted-foreground italic text-sm">
              待法务审核后填充完整条款内容
            </p>
          </section>

          <section className="mb-8">
            <h2 className="text-2xl font-semibold mb-4">2. 明确禁止行为</h2>
            <p className="text-muted-foreground leading-relaxed mb-4">
              以下行为严格禁止，违反者将立即终止服务并可能追究法律责任：
            </p>
            <div className="space-y-3 mb-4">
              <div className="flex items-start gap-3 p-3 rounded-lg bg-destructive/10 border border-destructive/20">
                <span className="text-destructive text-xl font-bold">❌</span>
                <div>
                  <p className="font-semibold text-destructive mb-1">DDoS 放大攻击</p>
                  <p className="text-sm text-muted-foreground">利用 idcd 节点发起分布式拒绝服务攻击或参与任何形式的流量放大攻击</p>
                </div>
              </div>

              <div className="flex items-start gap-3 p-3 rounded-lg bg-destructive/10 border border-destructive/20">
                <span className="text-destructive text-xl font-bold">❌</span>
                <div>
                  <p className="font-semibold text-destructive mb-1">端口扫描和漏洞探测</p>
                  <p className="text-sm text-muted-foreground">对未经授权的目标进行端口扫描、漏洞扫描或安全测试</p>
                </div>
              </div>

              <div className="flex items-start gap-3 p-3 rounded-lg bg-destructive/10 border border-destructive/20">
                <span className="text-destructive text-xl font-bold">❌</span>
                <div>
                  <p className="font-semibold text-destructive mb-1">未授权访问</p>
                  <p className="text-sm text-muted-foreground">尝试访问、探测或干扰您无权访问的系统、网络或服务</p>
                </div>
              </div>

              <div className="flex items-start gap-3 p-3 rounded-lg bg-destructive/10 border border-destructive/20">
                <span className="text-destructive text-xl font-bold">❌</span>
                <div>
                  <p className="font-semibold text-destructive mb-1">滥用带宽</p>
                  <p className="text-sm text-muted-foreground">过度使用网络资源导致服务质量下降，或绕过使用配额限制</p>
                </div>
              </div>

              <div className="flex items-start gap-3 p-3 rounded-lg bg-destructive/10 border border-destructive/20">
                <span className="text-destructive text-xl font-bold">❌</span>
                <div>
                  <p className="font-semibold text-destructive mb-1">恶意内容传播</p>
                  <p className="text-sm text-muted-foreground">利用服务传播恶意软件、病毒、钓鱼内容或其他有害代码</p>
                </div>
              </div>

              <div className="flex items-start gap-3 p-3 rounded-lg bg-destructive/10 border border-destructive/20">
                <span className="text-destructive text-xl font-bold">❌</span>
                <div>
                  <p className="font-semibold text-destructive mb-1">伪造身份和数据</p>
                  <p className="text-sm text-muted-foreground">伪造或篡改诊断数据、Evidence 报告签名或其他认证信息</p>
                </div>
              </div>
            </div>
            <p className="text-muted-foreground italic text-sm">
              待法务审核后填充完整条款内容
            </p>
          </section>

          <section className="mb-8">
            <h2 className="text-2xl font-semibold mb-4">3. 违规后果</h2>
            <p className="text-muted-foreground leading-relaxed mb-4">
              一旦发现违反本政策的行为，idcd 将采取以下措施：
            </p>
            <ul className="list-disc list-inside text-muted-foreground space-y-2 mb-4">
              <li>立即暂停或终止您的账户和服务访问权限</li>
              <li>保留相关日志和证据，配合执法机关调查</li>
              <li>拒绝退还任何已支付的费用</li>
              <li>追究民事或刑事法律责任</li>
              <li>将违规行为通报相关监管机构</li>
            </ul>
            <p className="text-muted-foreground italic text-sm">
              待法务审核后填充完整条款内容
            </p>
          </section>

          <section className="mb-8">
            <h2 className="text-2xl font-semibold mb-4">4. 举报机制</h2>
            <p className="text-muted-foreground leading-relaxed mb-4">
              如果您发现任何违反本政策的行为，或您的服务受到来自 idcd 节点的滥用影响，请立即通过以下方式联系我们：
            </p>
            <ul className="list-disc list-inside text-muted-foreground space-y-2">
              <li>滥用举报邮箱：abuse@idcd.com（待设置）</li>
              <li>紧急联系：kite365@gmail.com</li>
              <li>举报时请提供：时间、IP 地址、日志记录等相关证据</li>
            </ul>
            <p className="text-muted-foreground mt-4 text-sm">
              我们承诺在收到举报后 24 小时内响应，并在确认违规行为后立即采取行动。
            </p>
          </section>
        </div>
      </div>
    </main>
  )
}
