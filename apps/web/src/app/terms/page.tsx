import type { Metadata } from "next"
import Link from "next/link"
import { Button } from "@idcd/ui"
import { ArrowLeft } from "lucide-react"

export const metadata: Metadata = {
  title: "服务条款 - idcd",
  description: "idcd 服务条款",
}

export default function TermsPage() {
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
          <h1 className="text-4xl font-bold tracking-tight">服务条款</h1>
          <p className="mt-4 text-sm text-muted-foreground">
            最后更新：2026-05-13
          </p>
        </div>

        {/* 内容 */}
        <div className="prose prose-neutral dark:prose-invert max-w-none">
          <section className="mb-8">
            <h2 className="text-2xl font-semibold mb-4">1. 服务说明</h2>
            <p className="text-muted-foreground leading-relaxed mb-4">
              idcd 是一个专业的网络诊断和监控平台，提供多地拨测、实时监控、Evidence 证据服务等功能。本服务条款规范用户使用 idcd 服务的权利和义务。
            </p>
            <p className="text-muted-foreground italic text-sm">
              待法务审核后填充完整条款内容
            </p>
          </section>

          <section className="mb-8">
            <h2 className="text-2xl font-semibold mb-4">2. 用户责任</h2>
            <p className="text-muted-foreground leading-relaxed mb-4">
              用户在使用 idcd 服务时，应遵守相关法律法规，合法使用平台提供的网络诊断工具和监控功能。用户对其账户的使用行为负全部责任。
            </p>
            <p className="text-muted-foreground italic text-sm">
              待法务审核后填充完整条款内容
            </p>
          </section>

          <section className="mb-8">
            <h2 className="text-2xl font-semibold mb-4">3. 禁止行为</h2>
            <p className="text-muted-foreground leading-relaxed mb-4">
              用户不得利用 idcd 服务从事任何非法或违规活动，包括但不限于：未经授权的网络扫描、攻击行为、滥用平台资源等。详细禁止行为清单请参见《可接受使用政策》(AUP)。
            </p>
            <p className="text-muted-foreground italic text-sm">
              待法务审核后填充完整条款内容
            </p>
          </section>

          <section className="mb-8">
            <h2 className="text-2xl font-semibold mb-4">4. 免责声明</h2>
            <p className="text-muted-foreground leading-relaxed mb-4">
              idcd 提供的网络诊断数据仅供参考，不构成对网络状态的绝对保证。用户应根据实际情况综合判断，idcd 对因使用本服务而产生的任何直接或间接损失不承担责任。
            </p>
            <p className="text-muted-foreground italic text-sm">
              待法务审核后填充完整条款内容
            </p>
          </section>

          <section className="mb-8">
            <h2 className="text-2xl font-semibold mb-4">5. 联系方式</h2>
            <p className="text-muted-foreground leading-relaxed mb-4">
              如对本服务条款有任何疑问，请通过以下方式联系我们：
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
