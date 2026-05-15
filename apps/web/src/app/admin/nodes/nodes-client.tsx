"use client"

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { Badge } from "@/components/ui/badge"
import { Server, WifiOff, Clock, Activity } from "lucide-react"

export interface AdminNode {
  node_id: string
  hostname: string
  arch: string
  os: string
  ip_address: string
  agent_version: string
  status: "active" | "inactive" | "degraded"
  enrolled_at: string
  last_seen_at: string
  fingerprint: string
}

export function computeNodeStats(nodes: AdminNode[]) {
  const online   = nodes.filter(n => n.status === "active").length
  const offline  = nodes.filter(n => n.status === "inactive").length
  const degraded = nodes.filter(n => n.status === "degraded").length
  return { online, offline, degraded, total: nodes.length }
}

function statusBadge(status: AdminNode["status"]) {
  switch (status) {
    case "active":   return <Badge variant="default">在线</Badge>
    case "inactive": return <Badge variant="destructive">离线</Badge>
    case "degraded": return <Badge variant="outline" className="border-yellow-500 text-yellow-500">降级</Badge>
  }
}

function formatTime(iso: string): string {
  try {
    return new Date(iso).toLocaleString("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", second: "2-digit" })
  } catch { return iso }
}

export function NodesClient({ nodes }: { nodes: AdminNode[] }) {
  const stats = computeNodeStats(nodes)

  return (
    <div className="space-y-6">
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
              <Server className="h-4 w-4" />在线节点
            </CardTitle>
          </CardHeader>
          <CardContent><p className="text-2xl font-bold text-green-500">{stats.online}</p></CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
              <WifiOff className="h-4 w-4" />离线节点
            </CardTitle>
          </CardHeader>
          <CardContent><p className="text-2xl font-bold text-destructive">{stats.offline}</p></CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
              <Activity className="h-4 w-4" />降级节点
            </CardTitle>
          </CardHeader>
          <CardContent><p className="text-2xl font-bold text-yellow-500">{stats.degraded}</p></CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
              <Clock className="h-4 w-4" />节点总数
            </CardTitle>
          </CardHeader>
          <CardContent><p className="text-2xl font-bold">{stats.total}</p></CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader><CardTitle>节点列表</CardTitle></CardHeader>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>节点 ID</TableHead>
                <TableHead>主机名</TableHead>
                <TableHead>系统 / 架构</TableHead>
                <TableHead>IP 地址</TableHead>
                <TableHead>Agent 版本</TableHead>
                <TableHead>状态</TableHead>
                <TableHead>最后在线</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {nodes.length === 0 && (
                <TableRow>
                  <TableCell colSpan={7} className="py-8 text-center text-muted-foreground">暂无节点数据</TableCell>
                </TableRow>
              )}
              {nodes.map(node => (
                <TableRow key={node.node_id}>
                  <TableCell className="font-mono text-xs">{node.node_id}</TableCell>
                  <TableCell>{node.hostname}</TableCell>
                  <TableCell className="text-sm text-muted-foreground">{node.os} / {node.arch}</TableCell>
                  <TableCell className="font-mono text-xs">{node.ip_address}</TableCell>
                  <TableCell className="font-mono text-xs text-muted-foreground">{node.agent_version}</TableCell>
                  <TableCell>{statusBadge(node.status)}</TableCell>
                  <TableCell className="text-xs text-muted-foreground">{formatTime(node.last_seen_at)}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  )
}
