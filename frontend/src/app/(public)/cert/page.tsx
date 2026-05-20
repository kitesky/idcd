import Link from "next/link"
import {
  ShieldCheck,
  Globe,
  RefreshCw,
  KeyRound,
  ClipboardCheck,
  Server,
  Lock,
  Languages,
  AlertTriangle,
  CheckCircle2,
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
  title: "免费 SSL 证书在线申请 | idcd",
  description:
    "Let's Encrypt / ZeroSSL / Buypass / Google Trust Services 四大 ACME 免费 CA，一处登录管全部域名。支持通配符、多 SAN、自动续期，中文界面、内网可达。",
}

// ── 1. Hero ──────────────────────────────────────────────────────────────────

function Hero() {
  return (
    <section className="pt-20 pb-16 text-center px-4">
      <div className="flex flex-wrap justify-center gap-2 mb-6">
        <Badge variant="secondary">免费 DV 证书</Badge>
        <Badge variant="secondary">通配符 + 多 SAN</Badge>
        <Badge variant="secondary">自动续期</Badge>
        <Badge variant="secondary">中文界面</Badge>
      </div>
      <div className="flex flex-wrap justify-center gap-8 mb-8 text-center">
        <div>
          <div className="text-3xl font-bold">¥0</div>
          <div className="text-sm text-muted-foreground">完全免费</div>
        </div>
        <div>
          <div className="text-3xl font-bold">4</div>
          <div className="text-sm text-muted-foreground">免费 CA 可选</div>
        </div>
        <div>
          <div className="text-3xl font-bold">10</div>
          <div className="text-sm text-muted-foreground">SAN / 单证书</div>
        </div>
        <div>
          <div className="text-3xl font-bold">90s</div>
          <div className="text-sm text-muted-foreground">P95 签发耗时</div>
        </div>
      </div>
      <h1 className="text-4xl sm:text-5xl font-bold tracking-tight text-foreground mb-4">
        免费 SSL 证书<br className="hidden sm:inline" />
        申请 / 续期 / 撤销 一个面板搞定
      </h1>
      <p className="max-w-2xl mx-auto text-lg text-muted-foreground mb-8">
        覆盖 Let&apos;s Encrypt / ZeroSSL / Buypass / Google Trust Services 四大 ACME 免费 CA，
        DNS-01 自动验证，到期自动续期，私钥本地 KMS 加密托管
      </p>
      <div className="flex flex-wrap justify-center gap-3">
        <Button asChild>
          <Link href="/app/cert/new">立即免费申请</Link>
        </Button>
        <Button variant="outline" asChild>
          <Link href="/app/cert">控制台</Link>
        </Button>
      </div>
      <p className="mt-4 text-xs text-muted-foreground">
        登录即可申请，无需信用卡；支持单域名 / 多 SAN / 通配符
      </p>
    </section>
  )
}

// ── 2. 应用场景 ──────────────────────────────────────────────────────────────

const useCases = [
  {
    Icon: Globe,
    title: "个人站长 / 副业项目",
    desc: "多个小站点、副业项目、博客自架，自己跑 acme.sh / certbot 容易掉链；一处登录管全部域名，到期自动提醒和续期。",
    example: "「博客 + 工具站 + 项目 demo 共 8 个域名，一次授权 Cloudflare API，剩下 idcd 全自动」",
  },
  {
    Icon: Server,
    title: "中小企业运维",
    desc: "域名分散在阿里云 / DNSPod / Cloudflare 多家 DNS 商，手动加 TXT 太烦；一次授权 DNS API，所有 CA 自动续期。",
    example: "「30+ 内部子域名，全部走通配符 + 自动续期，再也不被深夜过期告警惊醒」",
  },
  {
    Icon: Languages,
    title: "国内开发者",
    desc: "阿里云 / 腾讯云免费证书每年 20 张限额、续期繁琐，英文 CA 界面又难懂；idcd 中文界面、内网可达、不卡限额。",
    example: "「之前每年要在阿里云 / 腾讯云之间来回跳；现在 idcd 一站搞定，国内访问也快」",
  },
] as const

function UseCases() {
  return (
    <section className="py-12 px-4 bg-muted/20">
      <div className="max-w-screen-xl mx-auto">
        <h2 className="text-2xl font-bold text-center mb-2">这些场景都能用</h2>
        <p className="text-center text-muted-foreground mb-8">从一个小站到几十个内部子域名，都帮你管到位</p>
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

// ── 3. 支持的 CA ─────────────────────────────────────────────────────────────

const cas = [
  { name: "Let's Encrypt", tag: "免费 / 默认", desc: "全球使用最广的免费 CA，ACMEv2 免 EAB，签发 < 60s" },
  { name: "ZeroSSL", tag: "免费 / 后备", desc: "EAB 自动配置，LE 配额吃紧时自动切换" },
  { name: "Buypass", tag: "免费 / 兜底", desc: "挪威 CA，根证书广泛信任，作为第二后备" },
  { name: "Google Trust Services", tag: "规划中", desc: "GCP 生态原生支持，需要 GCP 账号 + EAB" },
] as const

function CaList() {
  return (
    <section className="py-12 px-4">
      <div className="max-w-screen-xl mx-auto">
        <div className="text-center mb-8">
          <h2 className="text-2xl font-bold mb-2">4 个免费 CA，自动路由</h2>
          <p className="text-muted-foreground">默认 Let&apos;s Encrypt，配额吃紧自动切换备选 CA，永不卡住</p>
        </div>
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          {cas.map((c) => (
            <Card key={c.name}>
              <CardContent className="pt-4">
                <div className="flex items-center justify-between gap-2 mb-2">
                  <span className="text-sm font-semibold">{c.name}</span>
                  <Badge variant="outline" className="text-[10px]">{c.tag}</Badge>
                </div>
                <p className="text-xs text-muted-foreground leading-relaxed">{c.desc}</p>
              </CardContent>
            </Card>
          ))}
        </div>
      </div>
    </section>
  )
}

// ── 4. 工作流程 ──────────────────────────────────────────────────────────────

const steps = [
  {
    n: "1",
    title: "登录 + 输入域名",
    desc: "支持单域名 / 多 SAN（≤ 10）/ 通配符 *.example.com，提交前自动预检 CAA",
  },
  {
    n: "2",
    title: "DNS-01 验证",
    desc: "授权 Cloudflare / 阿里云 DNS / DNSPod / Route53 等，自动加 TXT 记录；也支持手动模式",
  },
  {
    n: "3",
    title: "CA 签发",
    desc: "CA 验证通过后立即签发；默认 LE，配额吃紧自动切 ZeroSSL / Buypass",
  },
  {
    n: "4",
    title: "下载 + 自动续期",
    desc: "PEM 包下载链接 5 分钟过期；到期前 30 天自动触发续期，邮件 + 站内消息",
  },
] as const

function HowItWorks() {
  return (
    <section className="py-12 px-4 bg-muted/30">
      <div className="max-w-screen-xl mx-auto">
        <h2 className="text-2xl font-bold text-center mb-2">从申请到续期，4 步走完</h2>
        <p className="text-center text-muted-foreground mb-8">登录后 3 分钟可完成首张证书，剩下的全部交给 idcd</p>
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
    Icon: ShieldCheck,
    title: "私钥 KMS 加密托管",
    desc: "ECDSA P-256 本地生成，AES-GCM + KMS 加密落库；下载链接 5 分钟过期",
  },
  {
    Icon: RefreshCw,
    title: "到期 30 天自动续期",
    desc: "Cron 调度 + retry queue 多次重试；DNS-01 走原 provider 全自动，邮件 + 站内消息双通知",
  },
  {
    Icon: KeyRound,
    title: "多 CA 自动路由",
    desc: "默认 Let&apos;s Encrypt，配额吃紧自动切 ZeroSSL / Buypass；CA 失败也有兜底",
  },
  {
    Icon: ClipboardCheck,
    title: "CAA 预检",
    desc: "申请前先检查 CAA 记录，给出明确「为何会被 CA 拒绝」的诊断，不让你在 60s 倒计时里乱猜",
  },
  {
    Icon: Lock,
    title: "撤销 + 反滥用",
    desc: "误签 / 私钥泄露一键 Revoke，CA + 本地状态同步；短时多根域名签发风控拦截",
  },
] as const

function Highlights() {
  return (
    <section className="py-12 px-4">
      <div className="max-w-screen-xl mx-auto">
        <h2 className="text-2xl font-bold text-center mb-8">为什么不直接用 acme.sh / certbot？</h2>
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

// ── 6. FAQ ──────────────────────────────────────────────────────────────────

const faqs = [
  {
    q: "证书真的免费吗？",
    a: "是。idcd 不自建 CA，我们是 Let's Encrypt / ZeroSSL / Buypass 这些公益免费 CA 的客户端封装，签发本身一直是免费的。当前 idcd 也不对申请张数收费，登录即可使用。",
  },
  {
    q: "支持通配符 / 多域名（SAN）吗？",
    a: "支持。单证书最多 10 个域名 / 含通配符 *.example.com。CA 上游对单证书 SAN 数有上限（LE 100 个），idcd 在 S1 阶段统一限到 10 个以控制风控风险。",
  },
  {
    q: "签发要多久？",
    a: "DNS-01 自动模式（Cloudflare / 阿里云 DNS 等）P95 在 90 秒内可下载；手动 DNS 模式取决于你加 TXT 记录的速度。",
  },
  {
    q: "私钥怎么存储？我担心安全",
    a: "私钥用 ECDSA P-256 本地生成（不上游 CA），落库前 AES-GCM + KMS keyring 加密；下载链接每次都临时生成，5 分钟过期。idcd 内部审计不可见明文。",
  },
  {
    q: "和阿里云 / 腾讯云的免费证书有什么区别？",
    a: "1) 不限张数（受 CA 自身 rate limit 制约，正常使用极少触达）；2) 中文界面 + 国内可达；3) 多 CA 自动路由 + 自动续期；4) 无需信用卡、登录即可。",
  },
] as const

function Faq() {
  return (
    <section className="py-12 px-4 bg-muted/20">
      <div className="max-w-3xl mx-auto">
        <h2 className="text-2xl font-bold text-center mb-2">常见问题</h2>
        <p className="text-center text-muted-foreground mb-8">还有疑问？直接{" "}
          <a href="mailto:hi@idcd.com" className="text-primary underline underline-offset-4">hi@idcd.com</a>
          {" "}问我们
        </p>
        <div className="space-y-4">
          {faqs.map((f) => (
            <Card key={f.q}>
              <CardHeader className="pb-2">
                <CardTitle className="text-base flex items-start gap-2">
                  <CheckCircle2 className="h-4 w-4 text-primary mt-0.5 shrink-0" />
                  <span>{f.q}</span>
                </CardTitle>
              </CardHeader>
              <CardContent>
                <p className="text-sm text-muted-foreground leading-relaxed">{f.a}</p>
              </CardContent>
            </Card>
          ))}
        </div>
      </div>
    </section>
  )
}

// ── 7. CTA 底部横幅 ──────────────────────────────────────────────────────────

function CtaBanner() {
  return (
    <section className="py-16 px-4 bg-primary/5 border-y">
      <div className="max-w-2xl mx-auto text-center">
        <div className="inline-flex items-center gap-2 mb-3 text-sm text-muted-foreground">
          <AlertTriangle className="h-4 w-4" />
          <span>HTTPS 已是 Chrome 默认要求，HTTP 站点会被标灰</span>
        </div>
        <p className="text-lg font-medium text-foreground mb-2">
          90 秒搞定一张免费证书，比泡一杯咖啡还快
        </p>
        <p className="text-muted-foreground mb-6">
          完全免费，无需信用卡；登录即可申请，通配符 / 多 SAN / 多 CA 任选
        </p>
        <Button size="lg" asChild>
          <Link href="/app/cert/new">免费申请第一张证书</Link>
        </Button>
      </div>
    </section>
  )
}

// ── Page ─────────────────────────────────────────────────────────────────────

export default function CertLandingPage() {
  return (
    <main>
      <Hero />
      <UseCases />
      <CaList />
      <HowItWorks />
      <Highlights />
      <Faq />
      <CtaBanner />
    </main>
  )
}
