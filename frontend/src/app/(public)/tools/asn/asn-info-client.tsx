"use client"

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui"
import { ToolQueryLayout } from "@/components/tools/ToolQueryLayout"
import { getASNInfo, type ASNInfo } from "@/lib/api"

function AsnResult({ result }: { result: ASNInfo }) {
  const rows: [string, string][] = [
    ["查询值", result.query],
    ["ASN", result.asn],
    ["ISP", result.isp],
    ["国家/地区", result.country],
    ["国家代码", result.country_code],
  ]

  return (
    <Card>
      <CardHeader>
        <CardTitle>ASN 信息</CardTitle>
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

export default function AsnInfoClient() {
  return (
    <ToolQueryLayout<ASNInfo>
      title="ASN 查询"
      description="查询 IP 地址或 AS 号对应的自治系统、ISP 和国家/地区信息"
      inputLabel="IP 地址或 AS 号"
      inputPlaceholder="1.1.1.1 或 AS13335"
      inputId="asn-query"
      onQuery={getASNInfo}
      renderResult={(r) => <AsnResult result={r} />}
      tips={
        <>
          <p>• <strong>IP 地址</strong>：输入 IPv4（1.1.1.1）或 IPv6 地址</p>
          <p>• <strong>AS 号</strong>：输入格式为 AS13335 或 13335</p>
          <p>• 显示自治系统归属 ISP 和地理位置信息</p>
        </>
      }
    />
  )
}
