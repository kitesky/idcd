"use client"

import { useState } from 'react'
import {
  Card, CardContent, CardHeader, CardTitle,
  Input, Button, Badge, Label, Separator,
} from '@/components/ui'
import { getToolBySlug } from '@/app/(public)/tools/tools-config'

const PROBE_INPUTS: Record<string, { label: string; placeholder: string; extra?: string }> = {
  ssl:   { label: '域名', placeholder: 'example.com' },
  whois: { label: '域名', placeholder: 'example.com' },
  icp:   { label: '域名', placeholder: 'example.com.cn' },
  ip:    { label: 'IP 地址', placeholder: '1.1.1.1 或 2606:4700:4700::1111' },
  tcp:   { label: '主机:端口', placeholder: 'example.com:443', extra: '格式: host:port' },
  mtr:   { label: '目标地址', placeholder: 'example.com 或 1.1.1.1' },
  smtp:  { label: 'SMTP 服务器', placeholder: 'smtp.example.com', extra: '默认端口 25/465/587' },
  rdns:  { label: 'IP 地址', placeholder: '8.8.8.8' },
  asn:   { label: 'ASN 号码', placeholder: 'AS13335 或 13335' },
  mx:    { label: '域名', placeholder: 'example.com' },
  spf:   { label: '域名', placeholder: 'example.com' },
  dmarc: { label: '域名', placeholder: 'example.com' },
  ntp:   { label: 'NTP 服务器', placeholder: 'pool.ntp.org 或 time.cloudflare.com' },
  dkim:  { label: '域名', placeholder: 'example.com', extra: 'DKIM 选择器（如 default、google）' },
  bgp:   { label: 'IP 或前缀', placeholder: '1.1.1.0/24 或 1.1.1.1' },
}

const PROBE_FIELDS_EXTRA: Record<string, React.FC<{ extra: string; setExtra: (v: string) => void }>> = {
  tcp: ({ extra, setExtra }) => (
    <div className="space-y-1">
      <Label>端口（可选，已含在地址中则忽略）</Label>
      <Input value={extra} onChange={e => setExtra(e.target.value)} placeholder="443" />
    </div>
  ),
  dkim: ({ extra, setExtra }) => (
    <div className="space-y-1">
      <Label>DKIM 选择器</Label>
      <Input value={extra} onChange={e => setExtra(e.target.value)} placeholder="default" />
    </div>
  ),
}

interface ProbeClientProps {
  slug: string
}

export default function ProbeToolClient({ slug }: ProbeClientProps) {
  const tool = getToolBySlug(slug)
  const config = PROBE_INPUTS[slug] ?? { label: '目标', placeholder: 'example.com' }

  const [target, setTarget] = useState('')
  const [extra, setExtra] = useState('')
  const [submitted, setSubmitted] = useState(false)
  const [loading, setLoading] = useState(false)

  const ExtraField = PROBE_FIELDS_EXTRA[slug]

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!target.trim()) return
    setLoading(true)
    setTimeout(() => {
      setLoading(false)
      setSubmitted(true)
    }, 800)
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">{tool?.name ?? slug}</h1>
        <p className="text-muted-foreground mt-2">{tool?.description}</p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>查询参数</CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-1">
              <Label htmlFor="target">{config.label}</Label>
              <Input
                id="target"
                value={target}
                onChange={e => setTarget(e.target.value)}
                placeholder={config.placeholder}
                required
              />
              {config.extra && (
                <p className="text-xs text-muted-foreground">{config.extra}</p>
              )}
            </div>

            {ExtraField && <ExtraField extra={extra} setExtra={setExtra} />}

            <Button type="submit" disabled={loading || !target.trim()}>
              {loading ? '查询中…' : '开始查询'}
            </Button>
          </form>
        </CardContent>
      </Card>

      {submitted && (
        <Card>
          <CardHeader>
            <div className="flex items-center gap-2">
              <CardTitle>查询结果</CardTitle>
              <Badge variant="secondary">示例数据</Badge>
            </div>
          </CardHeader>
          <CardContent className="space-y-4">
            <ProbeResultPlaceholder slug={slug} target={target} />
          </CardContent>
        </Card>
      )}

      <ProbeHelpCard slug={slug} />
    </div>
  )
}

function ProbeResultPlaceholder({ slug, target }: { slug: string; target: string }) {
  const rows = PLACEHOLDER_DATA[slug]?.(target) ?? [
    ['状态', '查询完成'],
    ['目标', target],
    ['说明', '实际数据将由后端拨测节点返回'],
  ]

  return (
    <div className="space-y-2">
      {rows.map(([k, v], i) => (
        <div key={i} className="flex gap-2 text-sm">
          <span className="text-muted-foreground w-32 shrink-0 font-medium">{k}</span>
          <span className="font-mono break-all">{v}</span>
        </div>
      ))}
    </div>
  )
}

const PLACEHOLDER_DATA: Record<string, (target: string) => string[][]> = {
  ssl: t => [
    ['域名', t],
    ['证书状态', '有效'],
    ['颁发机构', "Let's Encrypt"],
    ['有效期至', new Date(Date.now() + 86400000 * 90).toLocaleDateString('zh-CN')],
    ['SANs', t],
    ['协议', 'TLS 1.3'],
  ],
  whois: t => [
    ['域名', t],
    ['注册商', 'Cloudflare, Inc.'],
    ['注册日期', '2010-01-01'],
    ['到期日期', '2026-01-01'],
    ['名称服务器', 'ns1.cloudflare.com'],
    ['状态', 'clientTransferProhibited'],
  ],
  icp: t => [
    ['域名', t],
    ['备案号', '京ICP备XXXXXXXX号'],
    ['备案类型', '企业'],
    ['主办单位', '示例科技有限公司'],
    ['备案状态', '正常'],
  ],
  ip: t => [
    ['IP', t],
    ['国家/地区', '美国'],
    ['城市', 'San Francisco'],
    ['ISP', 'Cloudflare, Inc.'],
    ['ASN', 'AS13335'],
    ['经纬度', '37.7510° N, 97.8220° W'],
  ],
  tcp: t => [
    ['目标', t],
    ['状态', '端口开放'],
    ['响应时间', '12ms'],
    ['协议', 'TCP'],
  ],
  mtr: t => [
    ['目标', t],
    ['跳数', '10'],
    ['总延迟', '25ms'],
    ['丢包率', '0%'],
    ['路由路径', '本机 → 运营商 → … → 目标'],
  ],
  smtp: t => [
    ['服务器', t],
    ['端口 25', '开放'],
    ['端口 587', '开放'],
    ['STARTTLS', '支持'],
    ['Banner', '220 mail.example.com ESMTP'],
  ],
  rdns: t => [
    ['IP', t],
    ['PTR 记录', `reverse.${t.split('.').reverse().join('.')}.in-addr.arpa`],
    ['解析结果', 'host.example.com'],
  ],
  asn: t => [
    ['ASN', t.toUpperCase().startsWith('AS') ? t : `AS${t}`],
    ['名称', 'CLOUDFLARE, US'],
    ['国家', '美国'],
    ['IP 前缀数', '2,500+'],
    ['注册机构', 'ARIN'],
  ],
  mx: t => [
    ['域名', t],
    ['MX 记录 1', `10 mx1.${t}`],
    ['MX 记录 2', `20 mx2.${t}`],
    ['TTL', '300s'],
  ],
  spf: t => [
    ['域名', t],
    ['SPF 记录', `v=spf1 include:_spf.${t} ~all`],
    ['验证状态', '记录存在'],
  ],
  dmarc: t => [
    ['域名', `_dmarc.${t}`],
    ['DMARC 策略', 'v=DMARC1; p=reject; rua=mailto:dmarc@' + t],
    ['策略级别', 'reject（最严格）'],
  ],
  ntp: t => [
    ['服务器', t],
    ['状态', '可达'],
    ['时间偏差', '+0.012s'],
    ['版本', 'NTPv4'],
    ['层级', 'Stratum 1'],
  ],
  dkim: t => [
    ['域名', t],
    ['选择器', 'default'],
    ['公钥算法', 'RSA-2048'],
    ['状态', '记录存在'],
  ],
  bgp: t => [
    ['前缀', t],
    ['起源 AS', 'AS13335'],
    ['AS 路径', 'AS13335'],
    ['可见性', '99.9%'],
    ['最后更新', new Date().toLocaleDateString('zh-CN')],
  ],
}

function ProbeHelpCard({ slug }: { slug: string }) {
  const helpText: Record<string, string[]> = {
    ssl:   ['检查 HTTPS 证书的有效期和安全配置', '支持 IPv4 和 IPv6 双栈', '显示证书链完整性'],
    whois: ['查询域名注册商和持有者信息', '显示注册和到期日期', '数据来源 ICANN Whois'],
    icp:   ['查询网站 ICP 备案信息', '支持.cn/.com/.net 等域名', '数据来自工信部'],
    ip:    ['输入 IPv4 或 IPv6 地址', '显示地理位置和运营商', '支持私有 IP 检测'],
    tcp:   ['格式：host:port，如 example.com:443', '测试端口是否开放和响应时间', '不发送应用层数据'],
    mtr:   ['综合 ping 和 traceroute 功能', '显示每跳延迟和丢包率', '诊断网络路径故障'],
    smtp:  ['测试 SMTP 服务器连通性', '检测 TLS/STARTTLS 支持', '不实际发送邮件'],
    rdns:  ['通过 IP 地址查找对应域名', '查询 DNS PTR 记录', '反向解析结果'],
    asn:   ['输入 AS 号，如 AS13335 或 13335', '显示 ASN 注册信息和前缀', '数据来源 RIR'],
    mx:    ['查询域名的邮件交换服务器', '显示 MX 记录优先级', '验证邮件配置'],
    spf:   ['查询 SPF TXT 记录', '验证邮件发送策略', '防止邮件欺骗'],
    dmarc: ['查询 _dmarc 子域 TXT 记录', '显示 DMARC 策略级别', '分析报告地址配置'],
    ntp:   ['测试 NTP 服务器可达性', '显示时间偏差（offset）', '验证时间同步服务'],
    dkim:  ['需要提供 DKIM 选择器', '查询公钥记录', '验证邮件签名配置'],
    bgp:   ['输入 IP 地址或 CIDR 前缀', '查看 BGP 路由宣告情况', '数据来源 RouteViews/RIPE RIS'],
  }

  const tips = helpText[slug] ?? ['拨测请求通过全球分布式节点执行', '结果仅供参考']

  return (
    <Card>
      <CardHeader>
        <CardTitle>使用说明</CardTitle>
      </CardHeader>
      <CardContent className="text-sm text-muted-foreground space-y-1">
        {tips.map((tip, i) => (
          <p key={i}>• {tip}</p>
        ))}
      </CardContent>
    </Card>
  )
}
