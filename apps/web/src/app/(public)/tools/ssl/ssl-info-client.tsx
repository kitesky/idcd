"use client"

import { Card, CardContent, CardHeader, CardTitle, Badge } from "@/components/ui"
import { ToolQueryLayout } from "@/components/tools/ToolQueryLayout"
import { getSSLInfo, type SSLInfo } from "@/lib/api"

function SslResult({ result }: { result: SSLInfo }) {
  const rows: [string, string][] = [
    ["域名", result.domain],
    ["颁发机构", result.issuer],
    ["有效期起", result.valid_from],
    ["有效期至", result.valid_to],
    ["证书状态", result.is_valid ? "有效" : "无效"],
  ]

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle>证书信息</CardTitle>
          <Badge variant={result.days_remaining > 30 ? "default" : "destructive"}>
            {result.days_remaining > 0
              ? `${result.days_remaining} 天后到期`
              : "已过期"}
          </Badge>
        </div>
      </CardHeader>
      <CardContent className="space-y-3 text-sm">
        {rows.map(([k, v]) => (
          <div key={k} className="flex gap-2">
            <span className="text-muted-foreground w-24 shrink-0 font-medium">{k}</span>
            <span className="font-mono break-all">{v ?? "-"}</span>
          </div>
        ))}
      </CardContent>
    </Card>
  )
}

export default function SslInfoClient() {
  return (
    <ToolQueryLayout<SSLInfo>
      title="SSL 证书检测"
      description="检查域名的 SSL 证书有效性、颁发机构、到期日期和 SAN 域名列表"
      inputLabel="域名"
      inputPlaceholder="example.com"
      inputId="ssl-query"
      actionLabel="检测"
      loadingLabel="检测中..."
      onQuery={getSSLInfo}
      renderResult={(r) => <SslResult result={r} />}
      tips={
        <>
          <p>• <strong>域名</strong>：输入不含 https:// 的裸域名（如 example.com）</p>
          <p>• 检测 HTTPS 证书有效性和到期时间</p>
          <p>• 显示证书颁发机构和有效期信息</p>
        </>
      }
    />
  )
}
