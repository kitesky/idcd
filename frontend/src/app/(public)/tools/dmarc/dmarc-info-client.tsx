"use client"

import { Card, CardContent, CardHeader, CardTitle, Badge } from "@/components/ui"
import { ToolQueryLayout } from "@/components/tools/ToolQueryLayout"
import { getDMARCInfo, type DMARCInfo } from "@/lib/api"

function DmarcResult({ result }: { result: DMARCInfo }) {
  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle>DMARC 记录</CardTitle>
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

export default function DmarcInfoClient() {
  return (
    <ToolQueryLayout<DMARCInfo>
      title="DMARC 记录查询"
      description="查询域名的 DMARC 策略记录，检查邮件认证配置"
      inputLabel="域名"
      inputPlaceholder="example.com"
      inputId="dmarc-query"
      onQuery={getDMARCInfo}
      renderResult={(r) => <DmarcResult result={r} />}
      tips={
        <>
          <p>• <strong>域名</strong>：输入不含 https:// 的裸域名（如 example.com）</p>
          <p>• DMARC 记录位于 _dmarc.example.com TXT 记录</p>
          <p>• 配合 SPF 和 DKIM 共同防止邮件欺骗</p>
        </>
      }
    />
  )
}
