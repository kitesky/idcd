"use client"

import { useEffect, useState } from "react"
import { Card, CardContent, CardHeader, CardTitle, Badge, cn } from "@/components/ui"
import { getNodes, type Node } from "@/lib/api"

interface NodeSelectorProps {
  selectedNodes: string[]
  onNodesChange: (nodeIds: string[]) => void
}

export default function NodeSelector({ selectedNodes, onNodesChange }: NodeSelectorProps) {
  const [nodes, setNodes] = useState<Node[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")

  useEffect(() => {
    async function loadNodes() {
      try {
        setLoading(true)
        const nodes = await getNodes()
        setNodes(nodes)
        setError("")
      } catch (err) {
        setError(err instanceof Error ? err.message : "加载节点列表失败")
        setNodes([])
      } finally {
        setLoading(false)
      }
    }
    loadNodes()
  }, [])

  const handleToggleNode = (nodeId: string) => {
    if (selectedNodes.includes(nodeId)) {
      onNodesChange(selectedNodes.filter(id => id !== nodeId))
    } else {
      onNodesChange([...selectedNodes, nodeId])
    }
  }

  const handleSelectAll = () => {
    if (selectedNodes.length === nodes.filter(n => n.is_active).length) {
      onNodesChange([])
    } else {
      onNodesChange(nodes.filter(n => n.is_active).map(n => n.id))
    }
  }

  if (loading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>选择拨测节点</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-muted-foreground text-sm">加载节点列表中...</p>
        </CardContent>
      </Card>
    )
  }

  if (error) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>选择拨测节点</CardTitle>
        </CardHeader>
        <CardContent>
          <Badge variant="destructive">{error}</Badge>
        </CardContent>
      </Card>
    )
  }

  const activeNodes = nodes.filter(n => n.is_active)

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle>选择拨测节点</CardTitle>
          <button
            onClick={handleSelectAll}
            className="text-sm text-primary hover:underline"
          >
            {selectedNodes.length === activeNodes.length ? "取消全选" : "全选"}
          </button>
        </div>
      </CardHeader>
      <CardContent>
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-2">
          {activeNodes.map((node) => {
            const isSelected = selectedNodes.includes(node.id)
            return (
              <button
                key={node.id}
                onClick={() => handleToggleNode(node.id)}
                className={cn(
                  "px-3 py-2 rounded-md border text-left text-sm transition-colors",
                  isSelected
                    ? "bg-primary text-primary-foreground border-primary"
                    : "bg-card hover:bg-muted border-border"
                )}
              >
                <div className="font-medium">{node.name}</div>
                <div className="text-xs opacity-80">
                  {node.country_code} • {node.city} • {node.tier}
                </div>
              </button>
            )
          })}
        </div>
        {selectedNodes.length === 0 && (
          <p className="text-sm text-muted-foreground mt-4">
            未选择节点时，将使用所有可用节点进行拨测
          </p>
        )}
      </CardContent>
    </Card>
  )
}
