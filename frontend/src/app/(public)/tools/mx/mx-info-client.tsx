"use client"

import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  Badge,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui"
import { ToolQueryLayout } from "@/components/tools/ToolQueryLayout"
import { getMXInfo, type MXInfo } from "@/lib/api"

function MxResult({ result }: { result: MXInfo }) {
  const sortedRecords = [...result.records].sort((a, b) => a.priority - b.priority)

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle>MX 记录</CardTitle>
          <Badge variant="secondary">{result.domain}</Badge>
        </div>
      </CardHeader>
      <CardContent>
        {sortedRecords.length > 0 ? (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-24">优先级</TableHead>
                <TableHead>邮件服务器</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {sortedRecords.map((rec) => (
                <TableRow key={`${rec.priority}-${rec.host}`}>
                  <TableCell className="font-mono">{rec.priority}</TableCell>
                  <TableCell className="font-mono break-all">{rec.host}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        ) : (
          <p className="text-sm text-muted-foreground">未找到 MX 记录</p>
        )}
      </CardContent>
    </Card>
  )
}

export default function MxInfoClient() {
  return (
    <ToolQueryLayout<MXInfo>
      title="MX 记录查询"
      description="查询域名的 MX 邮件交换记录，显示邮件服务器优先级和主机名"
      inputLabel="域名"
      inputPlaceholder="gmail.com"
      inputId="mx-query"
      onQuery={getMXInfo}
      renderResult={(r) => <MxResult result={r} />}
      tips={
        <>
          <p>• <strong>域名</strong>：输入不含 https:// 的裸域名（如 gmail.com）</p>
          <p>• 优先级数值越小，邮件服务器优先级越高</p>
          <p>• MX 记录决定接收邮件时连接哪个服务器</p>
        </>
      }
    />
  )
}
