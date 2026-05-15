"use client"

import { Card, CardContent, CardHeader, CardTitle, Badge } from "@/components/ui"
import { ToolQueryLayout } from "@/components/tools/ToolQueryLayout"
import { getICPInfo, type ICPInfo } from "@/lib/api"

function IcpResult({ result }: { result: ICPInfo }) {
  const rows: [string, string][] = [
    ["域名", result.domain],
    ["备案号", result.icp_number],
    ["主办单位", result.company],
    ["备案类型", result.type],
    ["备案时间", result.filed_at],
  ]

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle>ICP 备案信息</CardTitle>
          {result.icp_number ? (
            <Badge variant="default">{result.icp_number}</Badge>
          ) : (
            <Badge variant="secondary">未备案</Badge>
          )}
        </div>
      </CardHeader>
      <CardContent className="space-y-3 text-sm">
        {rows.map(([k, v]) => (
          <div key={k} className="flex gap-2">
            <span className="text-muted-foreground w-24 shrink-0 font-medium">{k}</span>
            <span className="font-mono break-all">{v || "-"}</span>
          </div>
        ))}
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

export default function IcpInfoClient() {
  return (
    <ToolQueryLayout<ICPInfo>
      title="ICP 备案查询"
      description="查询域名的 ICP 备案号、主办单位、备案类型和备案时间"
      inputLabel="域名"
      inputPlaceholder="baidu.com"
      inputId="icp-query"
      onQuery={getICPInfo}
      renderResult={(r) => <IcpResult result={r} />}
      tips={
        <>
          <p>• <strong>域名</strong>：输入不含 https:// 的裸域名（如 baidu.com）</p>
          <p>• 查询工业和信息化部 ICP/IP 地址/域名信息备案管理系统数据</p>
          <p>• 在中国大陆提供互联网信息服务的网站须完成 ICP 备案</p>
        </>
      }
    />
  )
}
