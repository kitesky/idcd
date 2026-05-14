"use client"

import { useState } from "react"
import { ExternalLink, Plus } from "lucide-react"
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Separator } from "@/components/ui/separator"

interface StatusPageItem {
  id: string
  name: string
  slug: string
  monitorCount: number
  url: string
}

const MOCK_STATUS_PAGES: StatusPageItem[] = [
  {
    id: "sp-001",
    name: "acme.com 服务状态",
    slug: "demo",
    monitorCount: 8,
    url: "https://demo.status.idcd.com",
  },
]

const IS_FREE_PLAN = true

interface UpgradeDialogProps {
  open: boolean
  onClose: () => void
}

function UpgradeDialog({ open, onClose }: UpgradeDialogProps) {
  if (!open) return null
  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60"
      data-testid="upgrade-dialog"
    >
      <Card className="w-full max-w-sm mx-4">
        <CardHeader>
          <CardTitle>升级解锁状态页</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-sm text-muted-foreground">
            Free 档不支持创建状态页。升级到 Pro 可创建最多 3 个状态页，支持自定义品牌与监控绑定。
          </p>
          <div className="flex gap-2">
            <Button className="flex-1" data-testid="upgrade-confirm-button">
              升级到 Pro
            </Button>
            <Button variant="outline" onClick={onClose} data-testid="upgrade-cancel-button">
              稍后再说
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}

interface CreateSheetProps {
  open: boolean
  onClose: () => void
}

function CreateSheet({ open, onClose }: CreateSheetProps) {
  const [name, setName] = useState("")
  const [slug, setSlug] = useState("")
  const [desc, setDesc] = useState("")

  if (!open) return null

  function handleNameChange(val: string) {
    setName(val)
    setSlug(
      val
        .toLowerCase()
        .replace(/\s+/g, "-")
        .replace(/[^a-z0-9-]/g, "")
    )
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-end sm:items-center justify-center bg-black/60"
      data-testid="create-sheet"
    >
      <Card className="w-full max-w-lg mx-4 mb-0 sm:mb-0 rounded-t-xl sm:rounded-xl">
        <CardHeader>
          <CardTitle>新建状态页</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="sp-name">页面名称</Label>
            <Input
              id="sp-name"
              placeholder="例：acme.com 服务状态"
              value={name}
              onChange={(e) => handleNameChange(e.target.value)}
              data-testid="sp-name-input"
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="sp-slug">Slug（访问路径）</Label>
            <div className="flex items-center gap-2">
              <span className="text-sm text-muted-foreground shrink-0">
                .status.idcd.com/
              </span>
              <Input
                id="sp-slug"
                placeholder="acme"
                value={slug}
                onChange={(e) => setSlug(e.target.value)}
                data-testid="sp-slug-input"
              />
            </div>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="sp-desc">描述（可选）</Label>
            <Textarea
              id="sp-desc"
              placeholder="简短说明该状态页用途..."
              value={desc}
              onChange={(e) => setDesc(e.target.value)}
              rows={3}
              data-testid="sp-desc-input"
            />
          </div>
          <Separator />
          <div className="flex gap-2">
            <Button className="flex-1" data-testid="create-sp-button">
              创建状态页
            </Button>
            <Button variant="outline" onClick={onClose} data-testid="cancel-sp-button">
              取消
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}

export function StatusPagesClient() {
  const [showUpgradeDialog, setShowUpgradeDialog] = useState(false)
  const [showCreateSheet, setShowCreateSheet] = useState(false)

  function handleNewPage() {
    if (IS_FREE_PLAN) {
      setShowUpgradeDialog(true)
    } else {
      setShowCreateSheet(true)
    }
  }

  return (
    <div className="space-y-6" data-testid="status-pages-page">
      {IS_FREE_PLAN && (
        <Alert data-testid="free-plan-notice">
          <AlertTitle className="flex items-center gap-2">
            Free 档限制
            <Badge variant="warning" className="text-xs">
              升级 Pro 解锁
            </Badge>
          </AlertTitle>
          <AlertDescription>
            Free 档不支持创建状态页。升级到 Pro 可获得最多 3 个状态页，Team 可获得 10 个。
          </AlertDescription>
        </Alert>
      )}

      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold">状态页列表</h2>
        <Button onClick={handleNewPage} data-testid="new-page-button">
          <Plus className="mr-2 h-4 w-4" />
          新建状态页
        </Button>
      </div>

      {MOCK_STATUS_PAGES.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-12 text-center">
            <p className="text-sm text-muted-foreground">暂无状态页</p>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-3" data-testid="status-pages-list">
          {MOCK_STATUS_PAGES.map((sp) => (
            <Card key={sp.id} data-testid={`status-page-card-${sp.id}`}>
              <CardContent className="flex items-center justify-between py-4 px-5">
                <div className="space-y-1">
                  <div className="flex items-center gap-2">
                    <span className="font-medium text-sm">{sp.name}</span>
                  </div>
                  <div className="flex items-center gap-3 text-xs text-muted-foreground">
                    <span className="font-mono">{sp.slug}</span>
                    <span>·</span>
                    <span>{sp.monitorCount} 个监控项</span>
                  </div>
                </div>
                <a
                  href={sp.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-1 text-xs text-primary hover:underline underline-offset-4"
                  data-testid={`status-page-link-${sp.id}`}
                >
                  访问
                  <ExternalLink className="h-3 w-3" />
                </a>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      <UpgradeDialog
        open={showUpgradeDialog}
        onClose={() => setShowUpgradeDialog(false)}
      />
      <CreateSheet
        open={showCreateSheet}
        onClose={() => setShowCreateSheet(false)}
      />
    </div>
  )
}
