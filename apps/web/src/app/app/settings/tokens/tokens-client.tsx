"use client"

import { useState, useEffect, useCallback } from "react"
import { useTranslations } from "next-intl"
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
  Checkbox,
  Label,
} from "@/components/ui"
import { Copy, CheckCircle2, Plus, Trash2 } from "lucide-react"
import { apiRequest } from "@/lib/api"

// ── Types ─────────────────────────────────────────────────────────────────────

interface PAT {
  id: string
  name: string
  // key_prefix is returned from backend as part of the Token object
  key_prefix?: string
  scopes: string[]
  status: string
  created_at: string
  expires_at: string | null
  last_used_at: string | null
}

const AVAILABLE_SCOPES = [
  { value: "read:monitors", label: "read:monitors" },
  { value: "write:monitors", label: "write:monitors" },
  { value: "read:alerts", label: "read:alerts" },
  { value: "read:billing", label: "read:billing" },
]

const EXPIRY_OPTIONS = [
  { value: "7d", label: "7 天" },
  { value: "30d", label: "30 天" },
  { value: "90d", label: "90 天" },
  { value: "365d", label: "1 年" },
  { value: "never", label: "永不过期" },
]

// ── TokensClient ──────────────────────────────────────────────────────────────

export function TokensClient() {
  const t = useTranslations("settings")
  const [tokens, setTokens] = useState<PAT[]>([])
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState<string | null>(null)

  // ── Create dialog state ──────────────────────────────────────────────────
  const [showCreateDialog, setShowCreateDialog] = useState(false)
  const [newTokenName, setNewTokenName] = useState("")
  const [selectedScopes, setSelectedScopes] = useState<string[]>([])
  const [expiresIn, setExpiresIn] = useState("365d")
  const [creating, setCreating] = useState(false)
  const [createError, setCreateError] = useState<string | null>(null)
  const [createdToken, setCreatedToken] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  // ── Revoke state ─────────────────────────────────────────────────────────
  const [revokeTarget, setRevokeTarget] = useState<string | null>(null)
  const [revokeLoading, setRevokeLoading] = useState(false)

  // ── Load tokens ───────────────────────────────────────────────────────────

  const loadTokens = useCallback(async () => {
    setLoading(true)
    setLoadError(null)
    try {
      const res = await apiRequest<{ data: { tokens: PAT[] } }>(
        "/v1/account/tokens"
      )
      setTokens(res.data.tokens ?? [])
    } catch (err) {
      setLoadError(err instanceof Error ? err.message : t("tokens.loadFailed"))
    } finally {
      setLoading(false)
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps -- t 来自 i18n hook，引用稳定
  }, [])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- loadTokens 内部 await 后 setState；初次挂载触发
    void loadTokens()
  }, [loadTokens])

  // ── Handlers ─────────────────────────────────────────────────────────────

  function toggleScope(scope: string) {
    setSelectedScopes((prev) =>
      prev.includes(scope) ? prev.filter((s) => s !== scope) : [...prev, scope]
    )
  }

  async function handleCreate() {
    if (!newTokenName.trim()) {
      setCreateError(t("tokens.tokenNameRequired"))
      return
    }
    setCreating(true)
    setCreateError(null)
    try {
      const res = await apiRequest<{ data: PAT & { token: string } }>(
        "/v1/account/tokens",
        {
          method: "POST",
          body: JSON.stringify({
            name: newTokenName.trim(),
            scopes: selectedScopes,
            expires_in: expiresIn,
          }),
        }
      )
      setTokens((prev) => [res.data, ...prev])
      setCreatedToken(res.data.token)
    } catch (err) {
      setCreateError(
        err instanceof Error ? err.message : t("tokens.createFailed")
      )
    } finally {
      setCreating(false)
    }
  }

  function handleCloseCreateDialog() {
    setShowCreateDialog(false)
    setNewTokenName("")
    setSelectedScopes([])
    setExpiresIn("365d")
    setCreateError(null)
    setCreatedToken(null)
    setCopied(false)
  }

  async function handleCopy() {
    if (!createdToken) return
    try {
      await navigator.clipboard.writeText(createdToken)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      // fallback
    }
  }

  async function handleRevoke(id: string) {
    setRevokeLoading(true)
    try {
      await apiRequest(`/v1/account/tokens/${id}`, { method: "DELETE" })
      setTokens((prev) => prev.filter((t) => t.id !== id))
      setRevokeTarget(null)
    } catch {
      // keep confirm panel open on error; user can retry
    } finally {
      setRevokeLoading(false)
    }
  }

  function formatExpiry(expiresAt: string | null): React.ReactNode {
    if (!expiresAt) {
      return (
        <Badge variant="secondary" className="text-xs" data-testid="badge-no-expiry">
          {t("tokens.noExpiry")}
        </Badge>
      )
    }
    return (
      <span className="text-sm text-muted-foreground">
        {new Date(expiresAt).toLocaleDateString("zh-CN")}
      </span>
    )
  }

  return (
    <div data-testid="tokens-page" className="space-y-6">
      <Card data-testid="tokens-card">
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle>{t("tokens.title")}</CardTitle>
              <CardDescription className="mt-1">
                {t("tokens.desc")}
              </CardDescription>
            </div>
            <Button
              data-testid="btn-create-token"
              onClick={() => setShowCreateDialog(true)}
            >
              <Plus className="mr-2 h-4 w-4" />
              {t("tokens.create")}
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          {loading ? (
            <p
              className="text-sm text-muted-foreground py-4 text-center"
              data-testid="loading-tokens-message"
            >
              {t("tokens.loading")}
            </p>
          ) : loadError ? (
            <Alert variant="destructive" data-testid="load-tokens-error">
              <AlertDescription>{loadError}</AlertDescription>
            </Alert>
          ) : tokens.length === 0 ? (
            <p
              className="text-sm text-muted-foreground py-4 text-center"
              data-testid="empty-tokens-message"
            >
              {t("tokens.empty")}
            </p>
          ) : (
            <Table data-testid="tokens-table">
              <TableHeader>
                <TableRow>
                  <TableHead>名称</TableHead>
                  <TableHead>前缀</TableHead>
                  <TableHead>Scopes</TableHead>
                  <TableHead>到期时间</TableHead>
                  <TableHead>创建时间</TableHead>
                  <TableHead className="text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {tokens.map((tok) => (
                  <TableRow key={tok.id} data-testid={`token-row-${tok.id}`}>
                    <TableCell className="font-medium">{tok.name}</TableCell>
                    <TableCell>
                      <code className="text-xs bg-muted px-1.5 py-0.5 rounded">
                        {tok.key_prefix ?? "—"}
                      </code>
                    </TableCell>
                    <TableCell>
                      <div className="flex flex-wrap gap-1">
                        {tok.scopes.length === 0 ? (
                          <Badge variant="secondary" className="text-xs">
                            无 scope
                          </Badge>
                        ) : (
                          tok.scopes.map((s) => (
                            <Badge
                              key={s}
                              variant="outline"
                              className="text-xs"
                              data-testid={`scope-badge-${s}`}
                            >
                              {s}
                            </Badge>
                          ))
                        )}
                      </div>
                    </TableCell>
                    <TableCell>{formatExpiry(tok.expires_at)}</TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {new Date(tok.created_at).toLocaleDateString("zh-CN")}
                    </TableCell>
                    <TableCell className="text-right">
                      {revokeTarget === tok.id ? (
                        <div className="flex items-center justify-end gap-2">
                          <span className="text-xs text-muted-foreground">
                            确认撤销？
                          </span>
                          <Button
                            variant="destructive"
                            size="sm"
                            disabled={revokeLoading}
                            data-testid={`btn-confirm-revoke-${tok.id}`}
                            onClick={() => handleRevoke(tok.id)}
                          >
                            {revokeLoading ? "撤销中..." : "确认"}
                          </Button>
                          <Button
                            variant="outline"
                            size="sm"
                            disabled={revokeLoading}
                            data-testid={`btn-cancel-revoke-${tok.id}`}
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
                          data-testid={`btn-revoke-${tok.id}`}
                          onClick={() => setRevokeTarget(tok.id)}
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

      {/* ── Create Token Dialog ────────────────────────────────────────── */}
      <Dialog
        open={showCreateDialog}
        onOpenChange={(open) => !open && handleCloseCreateDialog()}
      >
        <DialogContent data-testid="create-token-dialog">
          <DialogHeader>
            <DialogTitle>生成新 Token</DialogTitle>
            {!createdToken && (
              <DialogDescription>
                为新 Token 设置名称、权限范围和有效期
              </DialogDescription>
            )}
          </DialogHeader>

          {!createdToken ? (
            <div className="space-y-4">
              {createError && (
                <Alert variant="destructive" data-testid="create-token-error">
                  <AlertDescription>{createError}</AlertDescription>
                </Alert>
              )}

              <div className="space-y-2">
                <Label htmlFor="token-name">Token 名称</Label>
                <Input
                  id="token-name"
                  placeholder="例如：本地 CLI、MCP 集成"
                  value={newTokenName}
                  onChange={(e) => {
                    setNewTokenName(e.target.value)
                    setCreateError(null)
                  }}
                  disabled={creating}
                  data-testid="input-token-name"
                  autoFocus
                />
              </div>

              <div className="space-y-2">
                <Label>权限范围（Scopes）</Label>
                <div
                  className="space-y-2"
                  data-testid="scopes-checkboxes"
                >
                  {AVAILABLE_SCOPES.map((scope) => (
                    <div
                      key={scope.value}
                      className="flex items-center gap-2"
                      data-testid={`scope-option-${scope.value}`}
                    >
                      <Checkbox
                        id={`scope-${scope.value}`}
                        checked={selectedScopes.includes(scope.value)}
                        onCheckedChange={() => toggleScope(scope.value)}
                        disabled={creating}
                        data-testid={`checkbox-scope-${scope.value}`}
                      />
                      <Label
                        htmlFor={`scope-${scope.value}`}
                        className="font-mono text-sm cursor-pointer"
                      >
                        {scope.label}
                      </Label>
                    </div>
                  ))}
                </div>
              </div>

              <div className="space-y-2">
                <Label htmlFor="token-expiry">有效期</Label>
                <Select
                  value={expiresIn}
                  onValueChange={setExpiresIn}
                  disabled={creating}
                >
                  <SelectTrigger
                    id="token-expiry"
                    data-testid="select-expiry"
                  >
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {EXPIRY_OPTIONS.map((opt) => (
                      <SelectItem
                        key={opt.value}
                        value={opt.value}
                        data-testid={`select-expiry-${opt.value}`}
                      >
                        {opt.label}
                      </SelectItem>
                    ))}
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
                  disabled={creating || newTokenName.trim() === ""}
                  data-testid="btn-submit-create"
                >
                  {creating ? "生成中..." : "生成"}
                </Button>
              </div>
            </div>
          ) : (
            <div className="space-y-4">
              <Alert data-testid="new-token-reveal">
                <AlertDescription className="space-y-2">
                  <p className="text-amber-600 dark:text-amber-400 font-medium text-sm">
                    此 token 只显示一次，请立即复制并妥善保管
                  </p>
                  <code
                    className="block bg-muted p-2 rounded text-xs break-all"
                    data-testid="new-token-value"
                  >
                    {createdToken}
                  </code>
                </AlertDescription>
              </Alert>

              <div className="flex gap-3 justify-end">
                <Button
                  variant="outline"
                  onClick={handleCopy}
                  data-testid="btn-copy-token"
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

      {/* ── Security note ────────────────────────────────────────────────── */}
      <Alert data-testid="tokens-security-note">
        <AlertDescription className="text-sm">
          Personal Access Token 具有账号访问权限，请勿泄露或写入代码仓库，建议通过环境变量注入。
        </AlertDescription>
      </Alert>
    </div>
  )
}
