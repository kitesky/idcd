"use client"

import { Card, CardContent, CardHeader, CardTitle, Badge } from "@/components/ui"
import { ToolQueryLayout } from "@/components/tools/ToolQueryLayout"
import { getBGPInfo, type BGPInfo } from "@/lib/api"

function BgpResult({ result }: { result: BGPInfo }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>BGP 路由信息</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4 text-sm">
        <div className="flex gap-2">
          <span className="text-muted-foreground w-16 shrink-0 font-medium">IP</span>
          <span className="font-mono break-all">{result.ip}</span>
        </div>

        {result.asns && result.asns.length > 0 && (
          <div className="flex gap-2 flex-wrap items-start">
            <span className="text-muted-foreground w-16 shrink-0 font-medium">ASN</span>
            <div className="flex gap-1 flex-wrap">
              {result.asns.map((asn) => (
                <Badge key={asn} variant="secondary" className="font-mono text-xs">
                  {asn}
                </Badge>
              ))}
            </div>
          </div>
        )}

        {result.prefixes && result.prefixes.length > 0 && (
          <div className="flex gap-2 items-start">
            <span className="text-muted-foreground w-16 shrink-0 font-medium">前缀</span>
            <div className="space-y-1">
              {result.prefixes.map((prefix) => (
                <div key={prefix} className="font-mono text-xs">{prefix}</div>
              ))}
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

export default function BgpInfoClient() {
  return (
    <ToolQueryLayout<BGPInfo>
      title="BGP 路由查询"
      description="查询 IP 地址的 BGP 路由信息，包括所属前缀和 AS 路径"
      inputLabel="IP 地址"
      inputPlaceholder="1.1.1.1"
      inputId="bgp-query"
      onQuery={getBGPInfo}
      renderResult={(r) => <BgpResult result={r} />}
      tips={
        <>
          <p>• <strong>IP 地址</strong>：输入 IPv4 或 IPv6 地址</p>
          <p>• 显示该 IP 所属的 BGP 路由前缀（CIDR）</p>
          <p>• 显示路径涉及的 AS 号列表</p>
        </>
      }
    />
  )
}
