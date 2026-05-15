"use client"

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui"
import { ToolQueryLayout } from "@/components/tools/ToolQueryLayout"
import { getRDNSInfo, type RDNSInfo } from "@/lib/api"

function RdnsResult({ result }: { result: RDNSInfo }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>反向 DNS 结果</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3 text-sm">
        <div className="flex gap-2">
          <span className="text-muted-foreground w-24 shrink-0 font-medium">IP</span>
          <span className="font-mono break-all">{result.ip}</span>
        </div>
        <div className="flex gap-2 items-start">
          <span className="text-muted-foreground w-24 shrink-0 font-medium">主机名</span>
          {result.hostnames && result.hostnames.length > 0 ? (
            <div className="space-y-1">
              {result.hostnames.map((h) => (
                <div key={h} className="font-mono break-all">{h}</div>
              ))}
            </div>
          ) : (
            <span className="text-muted-foreground">未找到 PTR 记录</span>
          )}
        </div>
      </CardContent>
    </Card>
  )
}

export default function RdnsInfoClient() {
  return (
    <ToolQueryLayout<RDNSInfo>
      title="反向 DNS 查询"
      description="查询 IP 地址对应的反向 DNS（PTR 记录）主机名"
      inputLabel="IP 地址"
      inputPlaceholder="1.1.1.1"
      inputId="rdns-query"
      onQuery={getRDNSInfo}
      renderResult={(r) => <RdnsResult result={r} />}
      tips={
        <>
          <p>• <strong>IP 地址</strong>：输入 IPv4 或 IPv6 地址</p>
          <p>• 通过 PTR 记录反向解析 IP 对应的主机名</p>
          <p>• 部分 IP 可能没有配置 PTR 记录</p>
        </>
      }
    />
  )
}
