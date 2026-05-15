"use client"

import { Card, CardContent, CardHeader, CardTitle, Badge } from "@/components/ui"
import { ToolQueryLayout } from "@/components/tools/ToolQueryLayout"
import { apiRequest } from "@/lib/api"

interface IPInfo {
  ip: string
  country: string
  city: string
  asn: string
  isp: string
  is_datacenter: boolean
  is_proxy: boolean
}

function getIPInfo(q: string): Promise<IPInfo> {
  return apiRequest<IPInfo>(`/v1/info/ip?q=${encodeURIComponent(q)}`)
}

function IpResult({ result }: { result: IPInfo }) {
  const rows: [string, string][] = [
    ["IP", result.ip],
    ["国家/地区", result.country],
    ["城市", result.city],
    ["ASN", result.asn],
    ["ISP", result.isp],
  ]

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle>查询结果</CardTitle>
          <div className="flex gap-2">
            {result.is_datacenter && <Badge variant="secondary">数据中心</Badge>}
            {result.is_proxy && <Badge variant="destructive">代理/VPN</Badge>}
          </div>
        </div>
      </CardHeader>
      <CardContent className="space-y-3 text-sm">
        {rows.map(([k, v]) => (
          <div key={k} className="flex gap-2">
            <span className="text-muted-foreground w-24 shrink-0 font-medium">{k}</span>
            <span className="font-mono break-all">{v || "-"}</span>
          </div>
        ))}
      </CardContent>
    </Card>
  )
}

export default function IpInfoClient() {
  return (
    <ToolQueryLayout<IPInfo>
      title="IP 地址查询"
      description="查询 IP 地址的地理位置、归属 ASN 和 ISP 信息"
      inputLabel="IP 地址或域名"
      inputPlaceholder="1.1.1.1 或 example.com"
      inputId="ip-query"
      onQuery={getIPInfo}
      renderResult={(r) => <IpResult result={r} />}
      tips={
        <>
          <p>• <strong>支持格式</strong>：IPv4（1.1.1.1）、IPv6（2606::1）、域名（example.com）</p>
          <p>• 显示地理位置、归属 ASN 和运营商信息</p>
          <p>• 标识数据中心 IP 和代理/VPN 出口</p>
        </>
      }
    />
  )
}
