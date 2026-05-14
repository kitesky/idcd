"use client"

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Badge } from "@/components/ui/badge"
import type { AdminNodeEntry } from "./mock-data"
import { computeNodeStats } from "./mock-data"
import { Server, WifiOff, Clock, Activity } from "lucide-react"

function statusBadge(status: AdminNodeEntry["status"]) {
  switch (status) {
    case "online":
      return <Badge variant="success">在线</Badge>
    case "offline":
      return <Badge variant="destructive">离线</Badge>
    case "degraded":
      return (
        <Badge variant="outline" className="text-yellow-500 border-yellow-500">
          降级
        </Badge>
      )
  }
}

function formatHeartbeat(iso: string): string {
  try {
    return new Date(iso).toLocaleString("zh-CN", {
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
    })
  } catch {
    return iso
  }
}

interface NodesClientProps {
  nodes: AdminNodeEntry[]
}

export function NodesClient({ nodes }: NodesClientProps) {
  const stats = computeNodeStats(nodes)

  return (
    <div className="space-y-6">
      {/* Stats cards */}
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground flex items-center gap-2">
              <Server className="h-4 w-4" />
              在线节点
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold text-green-500">{stats.online}</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground flex items-center gap-2">
              <WifiOff className="h-4 w-4" />
              离线节点
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold text-destructive">{stats.offline}</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground flex items-center gap-2">
              <Activity className="h-4 w-4" />
              平均延迟
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold">{stats.avgLatency} ms</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground flex items-center gap-2">
              <Clock className="h-4 w-4" />
              今日检测
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold">{stats.checksToday.toLocaleString()}</p>
          </CardContent>
        </Card>
      </div>

      {/* Nodes table */}
      <Card>
        <CardHeader>
          <CardTitle>节点列表</CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>节点 ID</TableHead>
                <TableHead>地区</TableHead>
                <TableHead>运营商 / ASN</TableHead>
                <TableHead>出口 IP</TableHead>
                <TableHead>状态</TableHead>
                <TableHead>延迟</TableHead>
                <TableHead>最后心跳</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {nodes.map((node) => (
                <TableRow key={node.id}>
                  <TableCell className="font-mono text-xs">{node.id}</TableCell>
                  <TableCell>{node.region}</TableCell>
                  <TableCell className="text-sm text-muted-foreground">{node.asn}</TableCell>
                  <TableCell className="font-mono text-xs">{node.exitIp}</TableCell>
                  <TableCell>{statusBadge(node.status)}</TableCell>
                  <TableCell>
                    {node.latencyMs > 0 ? `${node.latencyMs} ms` : "—"}
                  </TableCell>
                  <TableCell className="text-xs text-muted-foreground">
                    {formatHeartbeat(node.lastHeartbeat)}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  )
}
