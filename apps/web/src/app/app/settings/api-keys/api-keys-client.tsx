"use client"

import { useState, useEffect, useCallback } from "react"
import { useTranslations, useLocale } from "next-intl"
import { bcp47Of } from "@/i18n/registry"
import {
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  Input,
  Label,
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
import { apiRequest } from "@/lib/api"

// ── Types ─────────────────────────────────────────────────────────────────

interface APIKey {
  id: string
  name: string
  key_prefix: string
  type: string
  status: string
  created_at: string
  last_used_at: string | null
  expires_at: string | null
}

// ── KeyTypeBadge ──────────────────────────────────────────────────────────

function KeyTypeBadge({ keyType }: { keyType: string }) {
  const t = useTranslations("settings")
  if (keyType === "test") {
    return (
      <Badge
        variant="outline"
        className="text-xs border-orange-400 text-orange-500"
        data-testid="badge-test"
      >
        {t("apiKeys.typeTest")}
      </Badge>
    )
  }
  return (
    <Badge
      variant="outline"
      className="text-xs border-blue-400 text-blue-500"
      data-testid="badge-production"
    >
      {t("apiKeys.typeLive")}
    </Badge>
  )
}

// ── APIKeysClient ─────────────────────────────────────────────────────────

export function APIKeysClient() {
  const t = useTranslations("settings")
  const locale = useLocale()
  const bcp47 = bcp47Of(locale)
  const [keys, setKeys] = useState<APIKey[]>([])
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState<string | null>(null)

  // ── Create dialog state ──────────────────────────────────────────────────
  const [showCreateDialog, setShowCreateDialog] = useState(false)
  const [newKeyName, setNewKeyName] = useState("")
  const [newKeyType, setNewKeyType] = useState<string>("live")
  const [newKeyExpiry, setNewKeyExpiry] = useState<string>("never")
  const [creating, setCreating] = useState(false)
  const [createError, setCreateError] = useState<string | null>(null)
  const [createdKey, setCreatedKey] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  // ── Search / filter state ─────────────────────────────────────────────────
  const [search, setSearch] = useState("")
  const [typeFilter, setTypeFilter] = useState<string>("all")

  // ── Revoke confirm state ─────────────────────────────────────────────────
  const [revokeTarget, setRevokeTarget] = useState<string | null>(null)
  const [revokeLoading, setRevokeLoading] = useState(false)
  // Per-row revoke error so a failed delete is visible next to the row instead
  // of silently leaving the confirm panel open with no feedback.
  const [revokeError, setRevokeError] = useState<string | null>(null)

  // ── Load keys ─────────────────────────────────────────────────────────────

  const loadKeys = useCallback(async () => {
    setLoading(true)
    setLoadError(null)
    try {
      const res = await apiRequest<{ data: { api_keys: APIKey[] } }>(
        "/v1/account/api-keys"
      )
      setKeys(res.data.api_keys ?? [])
    } catch (err) {
      setLoadError(err instanceof Error ? err.message : t("apiKeys.loadFailed"))
    } finally {
      setLoading(false)
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps -- t 来自 i18n hook，引用稳定
  }, [])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- loadKeys 内部 await 后 setState；初次挂载触发
    void loadKeys()
  }, [loadKeys])

  // ── Handlers ─────────────────────────────────────────────────────────────

  async function handleCreate() {
    if (!newKeyName.trim()) {
      setCreateError(t("apiKeys.keyNameRequired"))
      return
    }
    setCreating(true)
    setCreateError(null)
    try {
      const res = await apiRequest<{ data: APIKey & { key: string } }>(
        "/v1/account/api-keys",
        {
          method: "POST",
          body: JSON.stringify({
            name: newKeyName.trim(),
            type: newKeyType,
            expires_in: newKeyExpiry,
          }),
        }
      )
      setKeys((prev) => [res.data, ...prev])
      setCreatedKey(res.data.key)
    } catch (err) {
      setCreateError(
        err instanceof Error ? err.message : t("apiKeys.createFailed")
      )
    } finally {
      setCreating(false)
    }
  }

  function handleCloseCreateDialog() {
    setShowCreateDialog(false)
    setNewKeyName("")
    setNewKeyType("live")
    setNewKeyExpiry("never")
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
    setRevokeError(null)
    try {
      await apiRequest(`/v1/account/api-keys/${id}`, { method: "DELETE" })
      setKeys((prev) => prev.filter((k) => k.id !== id))
      setRevokeTarget(null)
    } catch (err) {
      // Surface failures so the user knows the key is still active and can
      // retry. Previously the catch was empty and the confirm panel just
      // stopped spinning with no feedback.
      setRevokeError(err instanceof Error ? err.message : t("apiKeys.revokeFailed"))
    } finally {
      setRevokeLoading(false)
    }
  }

  // ── Derived: filtered keys ────────────────────────────────────────────────

  const filteredKeys = keys.filter((k) => {
    const matchesSearch = k.name.toLowerCase().includes(search.toLowerCase())
    const matchesType = typeFilter === "all" || k.type === typeFilter
    return matchesSearch && matchesType
  })

  return (
    <div data-testid="api-keys-page" className="space-y-6">
      <Card data-testid="api-keys-card">
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle>{t("apiKeys.title")}</CardTitle>
              <CardDescription className="mt-1">
                {t("apiKeys.cardDesc")}
              </CardDescription>
            </div>
            <Button
              data-testid="btn-create-key"
              onClick={() => setShowCreateDialog(true)}
            >
              <Plus className="mr-2 h-4 w-4" />
              {t("apiKeys.create")}
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          {loading ? (
            <p
              className="text-sm text-muted-foreground py-4 text-center"
              data-testid="loading-keys-message"
            >
              {t("apiKeys.loading")}
            </p>
          ) : loadError ? (
            <Alert variant="destructive" data-testid="load-keys-error">
              <AlertDescription>{loadError}</AlertDescription>
            </Alert>
          ) : keys.length === 0 ? (
            <p
              className="text-sm text-muted-foreground py-4 text-center"
              data-testid="empty-keys-message"
            >
              {t("apiKeys.empty")}
            </p>
          ) : (
            <>
              {/* ── Search & type filter bar ── */}
              <div className="flex gap-3 mb-4" data-testid="keys-filter-bar">
                <Input
                  placeholder={t("apiKeys.searchPlaceholder")}
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                  className="max-w-xs"
                  data-testid="input-search-keys"
                />
                <Select
                  value={typeFilter}
                  onValueChange={setTypeFilter}
                >
                  <SelectTrigger className="w-36" data-testid="select-filter-type">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="all">{t("apiKeys.filterAll")}</SelectItem>
                    <SelectItem value="live">{t("apiKeys.filterLive")}</SelectItem>
                    <SelectItem value="test">{t("apiKeys.filterTest")}</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              {filteredKeys.length === 0 ? (
                <p
                  className="text-sm text-muted-foreground py-6 text-center"
                  data-testid="no-match-keys-message"
                >
                  {t("apiKeys.noMatch")}
                </p>
              ) : (
            <Table data-testid="api-keys-table">
              <TableHeader>
                <TableRow>
                  <TableHead>{t("apiKeys.tableName")}</TableHead>
                  <TableHead>{t("apiKeys.tableType")}</TableHead>
                  <TableHead>{t("apiKeys.tablePrefix")}</TableHead>
                  <TableHead>{t("apiKeys.tableCreatedAt")}</TableHead>
                  <TableHead>{t("apiKeys.tableLastUsed")}</TableHead>
                  <TableHead className="text-right">{t("apiKeys.tableActions")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filteredKeys.map((key) => (
                  <TableRow key={key.id} data-testid={`key-row-${key.id}`}>
                    <TableCell className="font-medium">{key.name}</TableCell>
                    <TableCell>
                      <KeyTypeBadge keyType={key.type} />
                    </TableCell>
                    <TableCell>
                      <code className="text-xs bg-muted px-1.5 py-0.5 rounded">
                        {key.key_prefix}
                      </code>
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {new Date(key.created_at).toLocaleDateString(bcp47)}
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {key.last_used_at ? (
                        new Date(key.last_used_at).toLocaleDateString(bcp47)
                      ) : (
                        <Badge variant="secondary" className="text-xs">
                          {t("apiKeys.neverUsed")}
                        </Badge>
                      )}
                    </TableCell>
                    <TableCell className="text-right">
                      {revokeTarget === key.id ? (
                        <div className="flex flex-col items-end gap-2">
                          <div className="flex items-center justify-end gap-2">
                            <span className="text-xs text-muted-foreground">
                              {t("apiKeys.revokeConfirm")}
                            </span>
                            <Button
                              variant="destructive"
                              size="sm"
                              disabled={revokeLoading}
                              data-testid={`btn-confirm-revoke-${key.id}`}
                              onClick={() => handleRevoke(key.id)}
                            >
                              {revokeLoading ? t("apiKeys.revoking") : t("apiKeys.confirmRevoke")}
                            </Button>
                            <Button
                              variant="outline"
                              size="sm"
                              disabled={revokeLoading}
                              data-testid={`btn-cancel-revoke-${key.id}`}
                              onClick={() => {
                                setRevokeTarget(null)
                                setRevokeError(null)
                              }}
                            >
                              {t("apiKeys.cancelRevoke")}
                            </Button>
                          </div>
                          {revokeError && (
                            <span
                              className="text-xs text-destructive"
                              data-testid={`revoke-error-${key.id}`}
                            >
                              {revokeError}
                            </span>
                          )}
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
                          {t("apiKeys.revoke")}
                        </Button>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
              )}
            </>
          )}
        </CardContent>
      </Card>

      {/* ── Create Key Dialog ─────────────────────────────────────────── */}
      <Dialog open={showCreateDialog} onOpenChange={(open) => !open && handleCloseCreateDialog()}>
        <DialogContent data-testid="create-key-dialog">
          <DialogHeader>
            <DialogTitle>{t("apiKeys.createTitle")}</DialogTitle>
            {!createdKey && (
              <DialogDescription>{t("apiKeys.createDesc")}</DialogDescription>
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
                <Label htmlFor="api-key-name">{t("apiKeys.keyNameLabel")}</Label>
                <Input
                  id="api-key-name"
                  placeholder={t("apiKeys.keyNamePlaceholder")}
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
                <Label htmlFor="api-key-type">{t("apiKeys.keyType")}</Label>
                <Select
                  value={newKeyType}
                  onValueChange={setNewKeyType}
                  disabled={creating}
                >
                  <SelectTrigger id="api-key-type" data-testid="select-key-type">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="live" data-testid="select-item-production">{t("apiKeys.filterLive")}</SelectItem>
                    <SelectItem value="test" data-testid="select-item-test">{t("apiKeys.filterTest")}</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-2">
                <Label htmlFor="api-key-expiry">{t("apiKeys.validity")}</Label>
                <Select
                  value={newKeyExpiry}
                  onValueChange={setNewKeyExpiry}
                  disabled={creating}
                >
                  <SelectTrigger id="api-key-expiry" data-testid="select-key-expiry">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="30d">{t("apiKeys.validityOptions.d30")}</SelectItem>
                    <SelectItem value="90d">{t("apiKeys.validityOptions.d90")}</SelectItem>
                    <SelectItem value="365d">{t("apiKeys.validityOptions.y1")}</SelectItem>
                    <SelectItem value="never">{t("apiKeys.validityOptions.never")}</SelectItem>
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
                  {t("apiKeys.cancel")}
                </Button>
                <Button
                  onClick={handleCreate}
                  disabled={creating || newKeyName.trim() === ""}
                  data-testid="btn-submit-create"
                >
                  {creating ? t("apiKeys.creating") : t("apiKeys.createBtn")}
                </Button>
              </div>
            </div>
          ) : (
            <div className="space-y-4">
              <Alert data-testid="new-key-reveal">
                <AlertDescription className="space-y-2">
                  <p className="text-amber-600 dark:text-amber-400 font-medium text-sm">
                    {t("apiKeys.onceWarning")}
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
                      {t("apiKeys.copied")}
                    </>
                  ) : (
                    <>
                      <Copy className="mr-2 h-4 w-4" />
                      {t("apiKeys.copy")}
                    </>
                  )}
                </Button>
                <Button
                  onClick={handleCloseCreateDialog}
                  data-testid="btn-done-create"
                >
                  {t("apiKeys.done")}
                </Button>
              </div>
            </div>
          )}
        </DialogContent>
      </Dialog>

      {/* ── Security note ─────────────────────────────────────────────── */}
      <Alert data-testid="api-keys-security-note">
        <AlertDescription className="text-sm">
          {t("apiKeys.securityNote")}
        </AlertDescription>
      </Alert>
    </div>
  )
}
