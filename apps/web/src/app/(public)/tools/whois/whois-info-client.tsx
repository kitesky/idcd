"use client"

import { Card, CardContent, CardHeader, CardTitle, Badge } from "@/components/ui"
import { ToolQueryLayout } from "@/components/tools/ToolQueryLayout"
import { getWhoisInfo, type WhoisInfo } from "@/lib/api"

function WhoisResult({ result }: { result: WhoisInfo }) {
  const rows: [string, string | undefined][] = [
    ["域名", result.domain],
    ["注册商", result.registrar],
    ["注册日期", result.creation_date],
    ["到期日期", result.expiry_date ?? result.expiration_date],
  ]

  return (
    <Card>
      <CardHeader>
        <CardTitle>WHOIS 信息</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3 text-sm">
        {rows.map(([k, v]) => (
          <div key={k} className="flex gap-2">
            <span className="text-muted-foreground w-24 shrink-0 font-medium">{k}</span>
            <span className="font-mono break-all">{v || "-"}</span>
          </div>
        ))}

        {result.name_servers && result.name_servers.length > 0 && (
          <div className="flex gap-2 flex-wrap items-start">
            <span className="text-muted-foreground w-24 shrink-0 font-medium">名称服务器</span>
            <div className="flex gap-1 flex-wrap">
              {result.name_servers.map((ns) => (
                <Badge key={ns} variant="secondary" className="font-mono text-xs">
                  {ns}
                </Badge>
              ))}
            </div>
          </div>
        )}

        {result.note && (
          <div className="flex gap-2">
            <span className="text-muted-foreground w-24 shrink-0 font-medium">备注</span>
            <span className="text-muted-foreground break-all">{result.note}</span>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

export default function WhoisInfoClient() {
  return (
    <ToolQueryLayout<WhoisInfo>
      title="WHOIS 查询"
      description="查询域名的注册信息、注册商、注册日期、到期日期和名称服务器"
      inputLabel="域名"
      inputPlaceholder="example.com"
      inputId="whois-query"
      onQuery={getWhoisInfo}
      renderResult={(r) => <WhoisResult result={r} />}
      tips={
        <>
          <p>• <strong>域名</strong>：输入不含 https:// 的裸域名（如 example.com）</p>
          <p>• 显示注册商、注册日期、到期日期和 NS 记录</p>
          <p>• 部分域名后缀的 WHOIS 信息可能受限</p>
        </>
      }
    />
  )
}
