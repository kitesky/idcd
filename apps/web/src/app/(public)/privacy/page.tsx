import type { Metadata } from "next"
import Link from "next/link"
import { Button } from "@/components/ui"
import { ArrowLeft } from "lucide-react"

export const metadata: Metadata = {
  title: "隐私政策 - idcd",
  alternates: { canonical: "https://idcd.com/privacy" },
  robots: { index: false, follow: false },
  description: "idcd 隐私政策",
}

export default function PrivacyPage() {
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
          <h1 className="text-4xl font-bold tracking-tight">隐私政策</h1>
          <p className="mt-4 text-sm text-muted-foreground">
            最后更新：2026-05-13
          </p>
        </div>

        {/* 内容 */}
        <div className="prose prose-neutral dark:prose-invert max-w-none">
          <section className="mb-8">
            <h2 className="text-2xl font-semibold mb-4">1. 信息收集</h2>
            <p className="text-muted-foreground leading-relaxed mb-4">
              idcd 在提供服务过程中会收集必要的用户信息，包括但不限于：账户信息、使用记录、诊断配置、API 调用数据等。我们承诺仅收集服务所必需的最少信息。
            </p>
            <p className="text-muted-foreground italic text-sm">
              待法务审核后填充完整条款内容
            </p>
          </section>

          <section className="mb-8">
            <h2 className="text-2xl font-semibold mb-4">2. 信息使用</h2>
            <p className="text-muted-foreground leading-relaxed mb-4">
              我们收集的信息仅用于提供和改善服务，包括：执行网络诊断任务、生成监控报告、账单结算、技术支持等。我们不会将用户信息用于未经授权的商业目的。
            </p>
            <p className="text-muted-foreground italic text-sm">
              待法务审核后填充完整条款内容
            </p>
          </section>

          <section className="mb-8">
            <h2 className="text-2xl font-semibold mb-4">3. 数据安全</h2>
            <p className="text-muted-foreground leading-relaxed mb-4">
              idcd 采用业界标准的安全措施保护用户数据，包括：传输加密（TLS 1.3）、存储加密、访问控制、安全审计等。敏感数据（如 Evidence 报告签名）通过独立 KMS 系统管理。
            </p>
            <p className="text-muted-foreground italic text-sm">
              待法务审核后填充完整条款内容
            </p>
          </section>

          <section className="mb-8">
            <h2 className="text-2xl font-semibold mb-4">4. Cookie 政策</h2>
            <p className="text-muted-foreground leading-relaxed mb-4">
              idcd 使用 Cookie 和类似技术来提供更好的用户体验，包括：会话管理、用户偏好设置、分析服务使用情况等。您可以通过浏览器设置管理 Cookie，但这可能影响部分功能的正常使用。
            </p>
            <p className="text-muted-foreground italic text-sm">
              待法务审核后填充完整条款内容
            </p>
          </section>

          <section className="mb-8">
            <h2 className="text-2xl font-semibold mb-4">5. PIPL 合规（中国个人信息保护法）</h2>
            <p className="text-muted-foreground leading-relaxed mb-4">
              idcd 严格遵守《中华人民共和国个人信息保护法》(PIPL) 的相关规定。我们承诺：基于用户明确同意收集信息、明确告知信息使用目的、提供信息访问和删除机制、数据存储于中国境内或依法出境。
            </p>
            <p className="text-muted-foreground italic text-sm">
              待法务审核后填充完整条款内容
            </p>
          </section>

          <section className="mb-8">
            <h2 className="text-2xl font-semibold mb-4">6. 联系方式</h2>
            <p className="text-muted-foreground leading-relaxed mb-4">
              如对本隐私政策有任何疑问，或需要行使个人信息权利（访问、更正、删除等），请通过以下方式联系我们：
            </p>
            <ul className="list-disc list-inside text-muted-foreground space-y-2">
              <li>邮箱：kite365@gmail.com</li>
              <li>网站：<a href="https://idcd.com" className="text-primary hover:underline">https://idcd.com</a></li>
            </ul>
          </section>
        </div>
      </div>
    </main>
  )
}
