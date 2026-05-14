"use client"

import { useState } from "react"
import {
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  Input,
  Badge,
  Alert,
  AlertDescription,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui"
import { Copy, CheckCircle2, Plus, Trash2 } from "lucide-react"

// ── Types ─────────────────────────────────────────────────────────────────

interface APIKey {
  id: string
  name: string
  prefix: string
  key_type: string
  created_at: string
  last_used_at: string | null
}

// ── Mock data ─────────────────────────────────────────────────────────────

const MOCK_KEYS: APIKey[] = [
  {
    id: "key_001",
    name: "生产环境",
    prefix: "sk_live_abc...xyz",
    key_type: "production",
    created_at: "2025-03-01",
    last_used_at: "2025-05-13",
  },
  {
    id: "key_002",
    name: "CI/CD 流水线",
    prefix: "sk_live_def...uvw",
    key_type: "production",
    created_at: "2025-04-15",
    last_used_at: null,
  },
]

// ── KeyTypeBadge ──────────────────────────────────────────────────────────

function KeyTypeBadge({ keyType }: { keyType: string }) {
  if (keyType === "test") {
    return (
      <Badge
        variant="outline"
        className="text-xs border-orange-400 text-orange-500"
        data-testid="badge-test"
      >
        测试
      </Badge>
    )
  }
  return (
    <Badge
      variant="outline"
      className="text-xs border-blue-400 text-blue-500"
      data-testid="badge-production"
    >
      生产
    </Badge>
  )
}

// ── APIKeysClient ─────────────────────────────────────────────────────────

export function APIKeysClient() {
  const [keys, setKeys] = useState<APIKey[]>(MOCK_KEYS)

  // ── Create dialog state ──────────────────────────────────────────────────
  const [showCreateDialog, setShowCreateDialog] = useState(false)
  const [newKeyName, setNewKeyName] = useState("")
  const [newKeyType, setNewKeyType] = useState<string>("production")
  const [creating, setCreating] = useState(false)
  const [createError, setCreateError] = useState<string | null>(null)
  const [createdKey, setCreatedKey] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  // ── Revoke confirm state ─────────────────────────────────────────────────
  const [revokeTarget, setRevokeTarget] = useState<string | null>(null)
  const [revokeLoading, setRevokeLoading] = useState(false)

  // ── Handlers ─────────────────────────────────────────────────────────────

  async function handleCreate() {
    if (!newKeyName.trim()) {
      setCreateError("请输入 Key 名称")
      return
    }
    setCreating(true)
    setCreateError(null)
    try {
      // Mock: real impl calls POST /v1/account/api-keys
      await new Promise((r) => setTimeout(r, 300))
      const pfx = newKeyType === "test" ? "sk_test_" : "sk_live_"
      const mockFullKey = `${pfx}${Math.random().toString(36).slice(2, 10)}...${Math.random().toString(36).slice(2, 6)}`
      const newKey: APIKey = {
        id: `key_${Date.now()}`,
        name: newKeyName.trim(),
        prefix: mockFullKey.slice(0, 18) + "...",
        key_type: newKeyType,
        created_at: new Date().toISOString().slice(0, 10),
        last_used_at: null,
      }
      setKeys((prev) => [newKey, ...prev])
      setCreatedKey(mockFullKey)
    } catch {
      setCreateError("创建失败，请稍后重试")
    } finally {
      setCreating(false)
    }
  }

  function handleCloseCreateDialog() {
    setShowCreateDialog(false)
    setNewKeyName("")
    setNewKeyType("production")
    setCreateError(null)
    setCreatedKey(null)
    setCopied(false)
  }

  async function handleCopy() {
    if (!createdKey) return
    try {
      await navigator.clipboard.writeText(createdKey)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      // fallback: select text
    }
  }

  async function handleRevoke(id: string) {
    setRevokeLoading(true)
    try {
      // Mock: real impl calls DELETE /v1/account/api-keys/:id
      await new Promise((r) => setTimeout(r, 300))
      setKeys((prev) => prev.filter((k) => k.id !== id))
      setRevokeTarget(null)
    } catch {
      // ignore for mock
    } finally {
      setRevokeLoading(false)
    }
  }

  return (
    <div data-testid="api-keys-page" className="space-y-6">
      <Card data-testid="api-keys-card">
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle>API Keys</CardTitle>
              <CardDescription className="mt-1">
                用于通过 API 访问 idcd 服务。请妥善保管，切勿泄露。
              </CardDescription>
            </div>
            <Button
              data-testid="btn-create-key"
              onClick={() => setShowCreateDialog(true)}
            >
              <Plus className="mr-2 h-4 w-4" />
              创建 API Key
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          {keys.length === 0 ? (
            <p
              className="text-sm text-muted-foreground py-4 text-center"
              data-testid="empty-keys-message"
            >
              暂无 API Key，点击"创建 API Key"开始使用
            </p>
          ) : (
            <Table data-testid="api-keys-table">
              <TableHeader>
                <TableRow>
                  <TableHead>名称</TableHead>
                  <TableHead>类型</TableHead>
                  <TableHead>前缀</TableHead>
                  <TableHead>创建时间</TableHead>
                  <TableHead>最后使用</TableHead>
                  <TableHead className="text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {keys.map((key) => (
                  <TableRow key={key.id} data-testid={`key-row-${key.id}`}>
                    <TableCell className="font-medium">{key.name}</TableCell>
                    <TableCell>
                      <KeyTypeBadge keyType={key.key_type} />
                    </TableCell>
                    <TableCell>
                      <code className="text-xs bg-muted px-1.5 py-0.5 rounded">
                        {key.prefix}
                      </code>
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {key.created_at}
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {key.last_used_at ?? (
                        <Badge variant="secondary" className="text-xs">
                          从未使用
                        </Badge>
                      )}
                    </TableCell>
                    <TableCell className="text-right">
                      {revokeTarget === key.id ? (
                        <div className="flex items-center justify-end gap-2">
                          <span className="text-xs text-muted-foreground">
                            确认撤销？
                          </span>
                          <Button
                            variant="destructive"
                            size="sm"
                            disabled={revokeLoading}
                            data-testid={`btn-confirm-revoke-${key.id}`}
                            onClick={() => handleRevoke(key.id)}
                          >
                            {revokeLoading ? "撤销中..." : "确认"}
                          </Button>
                          <Button
                            variant="outline"
                            size="sm"
                            disabled={revokeLoading}
                            data-testid={`btn-cancel-revoke-${key.id}`}
                            onClick={() => setRevokeTarget(null)}
                          >
                            取消
                          </Button>
                        </div>
                      ) : (
                        <Button
                          variant="ghost"
                          size="sm"
                          className="text-destructive hover:text-destructive"
                          data-testid={`btn-revoke-${key.id}`}
                          onClick={() => setRevokeTarget(key.id)}
                        >
                          <Trash2 className="h-4 w-4 mr-1" />
                          撤销
                        </Button>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      {/* ── Create Key Dialog ─────────────────────────────────────────── */}
      <Dialog open={showCreateDialog} onOpenChange={(open) => !open && handleCloseCreateDialog()}>
        <DialogContent data-testid="create-key-dialog">
          <DialogHeader>
            <DialogTitle>创建 API Key</DialogTitle>
            {!createdKey && (
              <DialogDescription>为新 API Key 设置名称和类型</DialogDescription>
            )}
          </DialogHeader>

          {!createdKey ? (
            <div className="space-y-4">
              {createError && (
                <Alert variant="destructive" data-testid="create-key-error">
                  <AlertDescription>{createError}</AlertDescription>
                </Alert>
              )}

              <div className="space-y-2">
                <label className="text-sm font-medium">Key 名称</label>
                <Input
                  placeholder="例如：生产环境、CI/CD"
                  value={newKeyName}
                  onChange={(e) => {
                    setNewKeyName(e.target.value)
                    setCreateError(null)
                  }}
                  disabled={creating}
                  data-testid="input-key-name"
                  autoFocus
                />
              </div>

              <div className="space-y-2">
                <label className="text-sm font-medium">类型</label>
                <Select
                  value={newKeyType}
                  onValueChange={setNewKeyType}
                  disabled={creating}
                >
                  <SelectTrigger data-testid="select-key-type">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="production" data-testid="select-item-production">生产（Production）</SelectItem>
                    <SelectItem value="test" data-testid="select-item-test">测试（Test）</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className="flex gap-3 justify-end">
                <Button
                  variant="outline"
                  onClick={handleCloseCreateDialog}
                  disabled={creating}
                  data-testid="btn-cancel-create"
                >
                  取消
                </Button>
                <Button
                  onClick={handleCreate}
                  disabled={creating || newKeyName.trim() === ""}
                  data-testid="btn-submit-create"
                >
                  {creating ? "创建中..." : "创建"}
                </Button>
              </div>
            </div>
          ) : (
            <div className="space-y-4">
              <Alert data-testid="new-key-reveal">
                <AlertDescription className="space-y-2">
                  <p className="text-amber-600 dark:text-amber-400 font-medium text-sm">
                    此 key 只显示一次，请立即复制并妥善保管
                  </p>
                  <code
                    className="block bg-muted p-2 rounded text-xs break-all"
                    data-testid="new-key-value"
                  >
                    {createdKey}
                  </code>
                </AlertDescription>
              </Alert>

              <div className="flex gap-3 justify-end">
                <Button
                  variant="outline"
                  onClick={handleCopy}
                  data-testid="btn-copy-key"
                >
                  {copied ? (
                    <>
                      <CheckCircle2 className="mr-2 h-4 w-4 text-green-500" />
                      已复制
                    </>
                  ) : (
                    <>
                      <Copy className="mr-2 h-4 w-4" />
                      复制
                    </>
                  )}
                </Button>
                <Button
                  onClick={handleCloseCreateDialog}
                  data-testid="btn-done-create"
                >
                  完成
                </Button>
              </div>
            </div>
          )}
        </DialogContent>
      </Dialog>

      {/* ── Security note ─────────────────────────────────────────────── */}
      <Alert data-testid="api-keys-security-note">
        <AlertDescription className="text-sm">
          API Key 拥有您账号的完整权限，请勿在客户端代码中硬编码，建议通过环境变量注入。
        </AlertDescription>
      </Alert>
    </div>
  )
}
