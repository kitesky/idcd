import Link from "next/link"
import {
  Activity,
  Globe,
  Lock,
  ShieldCheck,
  CalendarClock,
  FileSearch,
  Cpu,
  ServerCog,
  BellRing,
  LineChart,
  Webhook,
  MapPin,
} from "lucide-react"
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"

export const metadata = {
  title: "网站与 API 监控 | idcd",
  description:
    "30+ 全球节点同时拨测，网站 / API / DNS / SSL / 域名到期一站监控。1 分钟最快频率，异常多通道告警，Free 档即可创建。",
}

// ── 1. Hero ──────────────────────────────────────────────────────────────────

function Hero() {
  return (
    <section className="pt-20 pb-16 text-center px-4">
      <div className="flex flex-wrap justify-center gap-2 mb-6">
        <Badge variant="secondary">全球 30+ 节点同时拨测</Badge>
        <Badge variant="secondary">1 分钟最快频率</Badge>
        <Badge variant="secondary">Free 档即可创建</Badge>
      </div>
      <div className="flex flex-wrap justify-center gap-8 mb-8 text-center">
        <div>
          <div className="text-3xl font-bold">8+</div>
          <div className="text-sm text-muted-foreground">监控类型</div>
        </div>
        <div>
          <div className="text-3xl font-bold">30+</div>
          <div className="text-sm text-muted-foreground">全球探测节点</div>
        </div>
        <div>
          <div className="text-3xl font-bold">60s</div>
          <div className="text-sm text-muted-foreground">最快拨测频率</div>
        </div>
        <div>
          <div className="text-3xl font-bold">5</div>
          <div className="text-sm text-muted-foreground">告警通道</div>
        </div>
      </div>
      <h1 className="text-4xl sm:text-5xl font-bold tracking-tight text-foreground mb-4">
        网站和 API 出问题？<br className="hidden sm:inline" />
        全球节点替你 7×24 盯着
      </h1>
      <p className="max-w-2xl mx-auto text-lg text-muted-foreground mb-8">
        多地拨测同时上场，单点波动不误报；HTTP / Ping / DNS / SSL / 域名到期 一处管完，异常 60 秒内推到你手机
      </p>
      <div className="flex flex-wrap justify-center gap-3">
        <Button asChild>
          <Link href="/auth/register">免费创建监控</Link>
        </Button>
        <Button variant="outline" asChild>
          <Link href="/app/monitors">查看演示</Link>
        </Button>
      </div>
    </section>
  )
}

// ── 2. 应用场景 ──────────────────────────────────────────────────────────────

const useCases = [
  {
    Icon: Globe,
    title: "网站可用性",
    desc: "首页 / 落地页 / 关键流程多地拨测，证书到期、域名到期、关键字消失同步告警，运维和市场都安心。",
    example: "「上海 / 香港 / 法兰克福 三地同时返回 200，任意一地连续 3 次失败立刻短信」",
  },
  {
    Icon: ServerCog,
    title: "API 健康度",
    desc: "REST / GraphQL / Webhook 出口监控，状态码 + 响应体关键字 + 延迟阈值三重断言，上线前接到 CI 也行。",
    example: "「/health 返回 200 且 body 含 \"ok\"，p95 延迟 < 800ms，否则飞书 + Webhook」",
  },
  {
    Icon: MapPin,
    title: "多地 / 多运营商对比",
    desc: "电信 / 联通 / 移动 / 海外 同一目标的延迟对比，定位「我这边访问慢但客服说一切正常」那种扯皮问题。",
    example: "「北京电信 vs 上海联通 vs 香港 PCCW 同时打 idcd.com，输出热力图」",
  },
] as const

function UseCases() {
  return (
    <section className="py-12 px-4 bg-muted/20">
      <div className="max-w-screen-xl mx-auto">
        <h2 className="text-2xl font-bold text-center mb-2">这些场景都能盯</h2>
        <p className="text-center text-muted-foreground mb-8">替你 24 小时蹲在网络上等故障</p>
        <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
          {useCases.map((uc) => (
            <Card key={uc.title} className="flex flex-col">
              <CardHeader className="pb-3">
                <div className="mb-3 flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10">
                  <uc.Icon className="h-5 w-5 text-primary" />
                </div>
                <CardTitle className="text-base">{uc.title}</CardTitle>
              </CardHeader>
              <CardContent className="flex-1 flex flex-col gap-3">
                <p className="text-sm text-muted-foreground">{uc.desc}</p>
                <blockquote className="text-xs italic text-muted-foreground bg-muted rounded-md px-3 py-2 border-l-2 border-primary/40">
                  {uc.example}
                </blockquote>
              </CardContent>
            </Card>
          ))}
        </div>
      </div>
    </section>
  )
}

// ── 3. 监控类型 ──────────────────────────────────────────────────────────────

const monitorTypes = [
  { Icon: Activity, name: "HTTP / HTTPS", desc: "状态码 + body 关键字 + 延迟阈值断言，5xx / 超时 / 证书错误一网打尽" },
  { Icon: Globe, name: "Ping", desc: "ICMP 延迟、丢包率、抖动，节点连通性的最朴素判断" },
  { Icon: ServerCog, name: "TCP 端口", desc: "数据库 / Redis / 自建服务的端口存活检测，防止 80 通 3306 不通" },
  { Icon: FileSearch, name: "DNS 解析", desc: "校验 A / AAAA / CNAME 返回值，避免被劫持 / 缓存污染" },
  { Icon: Lock, name: "SSL 到期", desc: "证书剩余天数、签发者、CN 校验，提前 30 / 14 / 7 天三阶告警" },
  { Icon: CalendarClock, name: "域名到期", desc: "WHOIS 周扫一次，到期前提前续费，永不再被甩锅「忘了续费」" },
  { Icon: ShieldCheck, name: "ICP 备案变更", desc: "国内站合规必备，备案号 / 主体一变立刻报，避免被监管下架" },
  { Icon: FileSearch, name: "关键字检测", desc: "页面 HTML 里关键字消失（如「购买」按钮没了）也算异常，比 200 更严格" },
] as const

function MonitorTypes() {
  return (
    <section className="py-12 px-4">
      <div className="max-w-screen-xl mx-auto">
        <div className="text-center mb-8">
          <h2 className="text-2xl font-bold mb-2">8 类监控，覆盖全链路</h2>
          <p className="text-muted-foreground">从最底层的 ICMP 到合规层的 ICP 备案，一个面板搞定</p>
        </div>
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          {monitorTypes.map((m) => (
            <Card key={m.name}>
              <CardContent className="pt-4">
                <div className="flex items-center gap-2 mb-2">
                  <m.Icon className="h-4 w-4 text-primary shrink-0" />
                  <span className="text-sm font-semibold">{m.name}</span>
                </div>
                <p className="text-xs text-muted-foreground leading-relaxed">{m.desc}</p>
              </CardContent>
            </Card>
          ))}
        </div>
        <p className="text-center text-sm text-muted-foreground mt-6">
          还在做 AI Agent 应用？看{" "}
          <Link href="/agent" className="text-primary underline underline-offset-4">
            LLM / Tool API / RAG 三类 AI 监控 →
          </Link>
        </p>
      </div>
    </section>
  )
}

// ── 4. 工作流程 ──────────────────────────────────────────────────────────────

const steps = [
  {
    n: "1",
    title: "创建任务",
    desc: "选监控类型、填目标、勾选节点、设频率（1m / 5m / 30m），断言条件可选",
  },
  {
    n: "2",
    title: "多节点并发拨测",
    desc: "默认 3 节点同时打，避免单点抖动误报；可选最多 30+ 节点全球覆盖",
  },
  {
    n: "3",
    title: "异常告警",
    desc: "连续 N 次失败才告警（防抖），邮件 / 短信 / Webhook / 飞书 / 钉钉 任选",
  },
  {
    n: "4",
    title: "证据 + 报表",
    desc: "每次拨测留 latency / status / body 快照，可一键生成 SLA 月报 / 故障复盘 PDF",
  },
] as const

function HowItWorks() {
  return (
    <section className="py-12 px-4 bg-muted/30">
      <div className="max-w-screen-xl mx-auto">
        <h2 className="text-2xl font-bold text-center mb-2">从创建到告警，4 步走完</h2>
        <p className="text-center text-muted-foreground mb-8">没有学习成本，3 分钟把第一条监控跑起来</p>
        <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-4">
          {steps.map((s) => (
            <Card key={s.n}>
              <CardHeader className="pb-2">
                <div className="flex items-center gap-2">
                  <span className="flex h-7 w-7 items-center justify-center rounded-full bg-primary text-primary-foreground text-sm font-bold">
                    {s.n}
                  </span>
                  <CardTitle className="text-base">{s.title}</CardTitle>
                </div>
              </CardHeader>
              <CardContent>
                <p className="text-sm text-muted-foreground">{s.desc}</p>
              </CardContent>
            </Card>
          ))}
        </div>
      </div>
    </section>
  )
}

// ── 5. 能力亮点 ──────────────────────────────────────────────────────────────

const highlights = [
  {
    Icon: MapPin,
    title: "全球 30+ 探测节点",
    desc: "电信 / 联通 / 移动 / 海外覆盖，任选节点子集，避免单点误报",
  },
  {
    Icon: BellRing,
    title: "5 通道告警",
    desc: "邮件 / 短信 / Webhook / 飞书 / 钉钉，支持值班轮换和升级策略",
  },
  {
    Icon: LineChart,
    title: "可视化历史",
    desc: "全部样本可回溯，按节点 / 时段切片，p50 / p95 / p99 一目了然",
  },
  {
    Icon: Webhook,
    title: "Open API + Webhook",
    desc: "拨测结果实时推到你的系统，接 Grafana / 自建 SRE 大盘都行",
  },
  {
    Icon: Cpu,
    title: "AI 故障摘要",
    desc: "异常段自动 LLM 总结根因（DNS 污染 / 节点丢包 / 证书过期等）",
  },
  {
    Icon: ShieldCheck,
    title: "证据存证",
    desc: "每次拨测留时间戳 + 节点签名，可作为 SLA 索赔 / 故障复盘的客观证据",
  },
] as const

function Highlights() {
  return (
    <section className="py-12 px-4">
      <div className="max-w-screen-xl mx-auto">
        <h2 className="text-2xl font-bold text-center mb-8">为什么不直接 cron + curl？</h2>
        <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
          {highlights.map((h) => (
            <Card key={h.title} className="flex flex-col">
              <CardHeader className="pb-3">
                <div className="mb-3 flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10">
                  <h.Icon className="h-5 w-5 text-primary" />
                </div>
                <CardTitle className="text-base">{h.title}</CardTitle>
              </CardHeader>
              <CardContent>
                <p className="text-sm text-muted-foreground">{h.desc}</p>
              </CardContent>
            </Card>
          ))}
        </div>
      </div>
    </section>
  )
}

// ── 6. 定价提示 ──────────────────────────────────────────────────────────────

function PricingHint() {
  return (
    <section className="py-12 px-4 bg-muted/20">
      <div className="max-w-2xl mx-auto">
        <Card className="border-primary/40 shadow-lg">
          <CardHeader>
            <div className="flex items-center justify-between gap-2">
              <CardTitle className="text-xl">Free 档即可创建</CardTitle>
              <Badge variant="outline">无需信用卡</Badge>
            </div>
            <p className="text-sm text-muted-foreground mt-1">
              先用着，需要更多监控数 / 更高频率再升级
            </p>
          </CardHeader>
          <CardContent>
            <ul className="space-y-2 mb-6">
              {[
                "Free: 5 个监控任务 / 5 分钟频率 / 3 个节点 / 邮件告警",
                "Pro: 50 个监控 / 1 分钟频率 / 全部节点 / 5 通道告警",
                "Team: 不限监控 / 30 秒频率 / 多人协作 / SSO + 审计",
              ].map((f) => (
                <li key={f} className="flex items-start gap-2 text-sm">
                  <span className="text-primary mt-0.5">✓</span>
                  <span>{f}</span>
                </li>
              ))}
            </ul>
            <div className="flex flex-wrap gap-3">
              <Button asChild>
                <Link href="/auth/register">免费创建</Link>
              </Button>
              <Button variant="outline" asChild>
                <Link href="/pricing">查看完整定价</Link>
              </Button>
            </div>
          </CardContent>
        </Card>
      </div>
    </section>
  )
}

// ── 7. CTA 底部横幅 ──────────────────────────────────────────────────────────

function CtaBanner() {
  return (
    <section className="py-16 px-4 bg-primary/5 border-y">
      <div className="max-w-2xl mx-auto text-center">
        <p className="text-lg font-medium text-foreground mb-2">
          每分钟都在丢钱的故障，不该等用户来报
        </p>
        <p className="text-muted-foreground mb-6">
          注册即可创建 5 个监控任务，邮件告警立即可用，不用信用卡
        </p>
        <Button size="lg" asChild>
          <Link href="/auth/register">注册并创建第一个监控</Link>
        </Button>
      </div>
    </section>
  )
}

// ── Page ─────────────────────────────────────────────────────────────────────

export default function MonitorsLandingPage() {
  return (
    <main>
      <Hero />
      <UseCases />
      <MonitorTypes />
      <HowItWorks />
      <Highlights />
      <PricingHint />
      <CtaBanner />
    </main>
  )
}
