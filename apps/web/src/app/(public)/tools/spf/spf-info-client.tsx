"use client"

import { Card, CardContent, CardHeader, CardTitle, Badge } from "@/components/ui"
import { ToolQueryLayout } from "@/components/tools/ToolQueryLayout"
import { getSPFInfo, type SPFInfo } from "@/lib/api"

function SpfResult({ result }: { result: SPFInfo }) {
  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle>SPF 记录</CardTitle>
          <Badge variant={result.found ? "default" : "secondary"}>
            {result.found ? "Found" : "Not Found"}
          </Badge>
        </div>
      </CardHeader>
      <CardContent className="space-y-3 text-sm">
        <div className="flex gap-2">
          <span className="text-muted-foreground w-16 shrink-0 font-medium">域名</span>
          <span className="font-mono break-all">{result.domain}</span>
        </div>
        {result.found && result.record && (
          <div className="space-y-1">
            <span className="text-muted-foreground font-medium">记录内容</span>
            <pre className="bg-muted rounded p-3 text-xs font-mono break-all whitespace-pre-wrap overflow-x-auto">
              {result.record}
            </pre>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

export default function SpfInfoClient() {
  return (
    <ToolQueryLayout<SPFInfo>
      title="SPF 记录查询"
      description="查询域名的 SPF（发件人策略框架）记录，验证邮件发送授权"
      inputLabel="域名"
      inputPlaceholder="example.com"
      inputId="spf-query"
      onQuery={getSPFInfo}
      renderResult={(r) => <SpfResult result={r} />}
      tips={
        <>
          <p>• <strong>域名</strong>：输入不含 https:// 的裸域名（如 example.com）</p>
          <p>• SPF 记录定义哪些服务器有权代表该域名发送邮件</p>
          <p>• 配置正确的 SPF 可减少邮件被判定为垃圾邮件</p>
        </>
      }
    />
  )
}
