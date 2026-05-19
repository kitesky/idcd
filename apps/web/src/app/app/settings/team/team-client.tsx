"use client"

import { useEffect, useState } from "react"
import { useTranslations } from "next-intl"
import {
  Avatar,
  AvatarFallback,
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  Input,
  Badge,
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
  Alert,
  AlertDescription,
  Skeleton,
} from "@/components/ui"
import { CheckCircle2, Copy, Key, Plus, UserPlus, Users } from "lucide-react"
import { toast } from "sonner"
import { apiRequest } from "@/lib/api"

interface TeamAPIKey {
  id: string
  name: string
  prefix: string
  key_type: "production" | "test"
  created_at: string
}

interface TeamMember {
  id: string
  user_id: string
  email: string
  role: "owner" | "admin" | "member"
  joined_at: string
}

interface PendingInvitation {
  id: string
  email: string
  role: string
  expires_at: string
}

interface Team {
  id: string
  name: string
  slug: string
  plan: string
  member_count: number
}

function RoleBadge({ role }: { role: string }) {
  const t = useTranslations("settings")
  // Map role to its translated label. Unknown roles fall back to the raw value
  // so we surface unexpected data instead of silently rendering "member".
  const labelKey =
    role === "owner" || role === "admin" || role === "member"
      ? (`team.roles.${role}` as const)
      : null
  const label = labelKey ? t(labelKey) : role
  if (role === "owner") {
    return (
      <Badge
        variant="outline"
        className="border-primary text-primary text-xs"
        data-testid={`role-badge-${role}`}
      >
        {label}
      </Badge>
    )
  }
  if (role === "admin") {
    return (
      <Badge
        variant="outline"
        className="border-info text-info text-xs"
        data-testid={`role-badge-${role}`}
      >
        {label}
      </Badge>
    )
  }
  return (
    <Badge
      variant="outline"
      className="border-muted-foreground text-muted-foreground text-xs"
      data-testid={`role-badge-${role}`}
    >
      {label}
    </Badge>
  )
}

function MemberAvatar({ email }: { email: string }) {
  const initials = email.slice(0, 2).toUpperCase()
  // shadcn Avatar gives us the same visual + accessible name (aria-label on
  // Radix's Avatar Root) plus future-proofs for AvatarImage when we ship
  // gravatar / uploaded avatars to the members list.
  return (
    <Avatar className="h-8 w-8" aria-label={email}>
      <AvatarFallback className="text-xs font-medium">{initials}</AvatarFallback>
    </Avatar>
  )
}

function EmptyState({ onCreateTeam }: { onCreateTeam: () => void }) {
  const t = useTranslations("settings")
  return (
    <div
      className="flex flex-col items-center justify-center py-16 gap-4"
      data-testid="team-empty-state"
    >
      <div className="flex h-14 w-14 items-center justify-center rounded-full bg-muted">
        <Users className="h-7 w-7 text-muted-foreground" />
      </div>
      <div className="text-center">
        <p className="text-sm font-medium">{t("team.noTeam")}</p>
        <p className="text-xs text-muted-foreground mt-1">
          {t("team.noTeamDesc")}
        </p>
      </div>
      <Button
        size="sm"
        onClick={onCreateTeam}
        data-testid="btn-create-team-empty"
      >
        <Plus className="h-4 w-4 mr-1" />
        {t("team.createTeam")}
      </Button>
    </div>
  )
}

function TableSkeleton({ rows = 3, cols = 4 }: { rows?: number; cols?: number }) {
  return (
    <>
      {Array.from({ length: rows }).map((_, i) => (
        <TableRow key={i}>
          {Array.from({ length: cols }).map((__, j) => (
            <TableCell key={j}>
              <Skeleton className="h-4 w-full" />
            </TableCell>
          ))}
        </TableRow>
      ))}
    </>
  )
}

export function TeamClient() {
  const t = useTranslations("settings")
  const [team, setTeam] = useState<Team | null>(null)
  const [members, setMembers] = useState<TeamMember[]>([])
  const [invitations, setInvitations] = useState<PendingInvitation[]>([])
  const [teamKeys, setTeamKeys] = useState<TeamAPIKey[]>([])
  const [teamPlan, setTeamPlan] = useState<string>("free")

  const [loadingTeam, setLoadingTeam] = useState(true)
  const [loadingMembers, setLoadingMembers] = useState(false)
  const [loadingInvitations, setLoadingInvitations] = useState(false)
  const [loadingKeys, setLoadingKeys] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const [showCreateTeamDialog, setShowCreateTeamDialog] = useState(false)
  const [newTeamName, setNewTeamName] = useState("")
  const [newTeamSlug, setNewTeamSlug] = useState("")
  const [creatingTeam, setCreatingTeam] = useState(false)

  const [showInviteDialog, setShowInviteDialog] = useState(false)
  const [inviteEmail, setInviteEmail] = useState("")
  const [inviteRole, setInviteRole] = useState("member")
  const [inviting, setInviting] = useState(false)
  // createdInvite holds the one-time-visible secret returned by
  // POST /v1/teams/{id}/invitations. Token + invite_url come back exactly
  // once — the backend stores SHA-256(token), so dismissing this dialog
  // without copying means re-issuing a fresh invitation. URL is empty when
  // server.app_base_url is unset (dev fallback); we still surface the raw
  // token in that case so the operator has something to paste.
  const [createdInvite, setCreatedInvite] = useState<{ token: string; url: string } | null>(null)
  const [copiedInviteLink, setCopiedInviteLink] = useState(false)
  const [copiedInviteToken, setCopiedInviteToken] = useState(false)

  const [showAddKeyDialog, setShowAddKeyDialog] = useState(false)
  const [createdKeyValue, setCreatedKeyValue] = useState<string | null>(null)
  const [copiedKey, setCopiedKey] = useState(false)
  const [newKeyName, setNewKeyName] = useState("")
  const [newKeyType, setNewKeyType] = useState<"production" | "test">("production")
  const [addingKey, setAddingKey] = useState(false)

  // Load team on mount
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- 初次挂载重置 loading，随后异步 fetch
    setLoadingTeam(true)
    apiRequest<{ data: { teams: Team[] } }>("/v1/teams")
      .then((json) => {
        const teams = json.data?.teams ?? []
        if (teams.length > 0) {
          // Don't shadow the outer `t` (useTranslations) — the .catch below
          // reaches up the closure for it and a local `t = teams[0]` made
          // the code path fragile to refactor.
          const firstTeam = teams[0]!
          setTeam(firstTeam)
          setTeamPlan(firstTeam.plan)
          loadTeamDetails(firstTeam.id)
        }
      })
      .catch((err) => setError(err instanceof Error ? err.message : t("team.loadFailed")))
      .finally(() => setLoadingTeam(false))
    // eslint-disable-next-line react-hooks/exhaustive-deps -- 仅初次挂载加载团队；t 用于 fallback 文案，不需要重跑 effect
  }, [])

  function loadTeamDetails(teamId: string) {
    setLoadingMembers(true)
    apiRequest<{ data: { members: TeamMember[] } }>(`/v1/teams/${teamId}/members`)
      .then((json) => setMembers(json.data?.members ?? []))
      .catch(() => setMembers([]))
      .finally(() => setLoadingMembers(false))

    setLoadingInvitations(true)
    apiRequest<{ data: { invitations: PendingInvitation[] } }>(`/v1/teams/${teamId}/invitations`)
      .then((json) => setInvitations(json.data?.invitations ?? []))
      .catch(() => setInvitations([]))
      .finally(() => setLoadingInvitations(false))

    setLoadingKeys(true)
    apiRequest<{ data: { api_keys: TeamAPIKey[] } }>(`/v1/teams/${teamId}/api-keys`)
      .then((json) => setTeamKeys(json.data?.api_keys ?? []))
      .catch(() => setTeamKeys([]))
      .finally(() => setLoadingKeys(false))
  }

  async function handleCreateTeam() {
    if (!newTeamName.trim() || !newTeamSlug.trim()) return
    setError(null)
    setCreatingTeam(true)
    try {
      const json = await apiRequest<{ data: { team: Team } }>("/v1/teams", {
        method: "POST",
        body: JSON.stringify({ name: newTeamName, slug: newTeamSlug }),
      })
      const created = json.data?.team
      if (created) {
        setTeam(created)
        setTeamPlan(created.plan)
        loadTeamDetails(created.id)
        toast.success(t("team.createSuccess", { name: created.name }))
      }
      setNewTeamName("")
      setNewTeamSlug("")
      setShowCreateTeamDialog(false)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : t("team.createFailed")
      setError(msg)
      toast.error(msg)
    } finally {
      setCreatingTeam(false)
    }
  }

  async function handleInvite() {
    if (!inviteEmail.trim() || !team) return
    const targetEmail = inviteEmail.trim()
    setError(null)
    setInviting(true)
    try {
      // Response widens PendingInvitation with the one-time `token` and
      // optional `invite_url` (server.WithAppBaseURL fills it when set).
      // Cast on the way out so the existing list state stays typed.
      const json = await apiRequest<{
        data: { invitation: PendingInvitation & { token?: string; invite_url?: string } }
      }>(`/v1/teams/${team.id}/invitations`, {
        method: "POST",
        body: JSON.stringify({ email: targetEmail, role: inviteRole }),
      })
      const inv = json.data?.invitation
      if (inv) {
        const { token, invite_url, ...listEntry } = inv
        setInvitations((prev) => [...prev, listEntry])
        if (token) {
          // Keep the dialog open in "reveal" mode; the inviter has to copy
          // before closing because the server only stores the hash.
          setCreatedInvite({ token, url: invite_url ?? "" })
        } else {
          // Defensive: older API build that doesn't surface the token.
          setShowInviteDialog(false)
        }
      }
      toast.success(t("team.inviteSuccess", { email: targetEmail }))
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : t("team.inviteFailed")
      setError(msg)
      toast.error(msg)
    } finally {
      setInviting(false)
    }
  }

  async function handleAddKey() {
    if (!newKeyName.trim() || !team) return
    setError(null)
    setAddingKey(true)
    try {
      const json = await apiRequest<{ data: { api_key: TeamAPIKey & { key: string } } }>(
        `/v1/teams/${team.id}/api-keys`,
        {
          method: "POST",
          body: JSON.stringify({ name: newKeyName, key_type: newKeyType }),
        }
      )
      const key = json.data?.api_key
      if (key) {
        setTeamKeys((prev) => [...prev, key])
        setCreatedKeyValue(key.key)
        toast.success(t("team.addKeySuccess"))
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : t("team.addKeyFailed")
      setError(msg)
      toast.error(msg)
    } finally {
      setAddingKey(false)
    }
  }

  async function handleRevokeKey(keyID: string) {
    if (!team) return
    setError(null)
    try {
      await apiRequest(`/v1/teams/${team.id}/api-keys/${keyID}`, { method: "DELETE" })
      setTeamKeys((prev) => prev.filter((k) => k.id !== keyID))
      toast.success(t("team.revokeKeySuccess"))
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : t("team.revokeKeyFailed")
      setError(msg)
      toast.error(msg)
    }
  }

  async function handleRemoveMember(userId: string) {
    if (!team) return
    setError(null)
    try {
      await apiRequest(`/v1/teams/${team.id}/members/${userId}`, { method: "DELETE" })
      setMembers((prev) => prev.filter((m) => m.user_id !== userId))
      setTeam((prev) => prev ? { ...prev, member_count: prev.member_count - 1 } : prev)
      toast.success(t("team.removeMemberSuccess"))
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : t("team.removeMemberFailed")
      setError(msg)
      toast.error(msg)
    }
  }

  if (loadingTeam) {
    return (
      <div data-testid="team-page">
        <Card>
          <CardHeader>
            <Skeleton className="h-5 w-40" data-testid="skeleton-team-name" />
            <Skeleton className="h-4 w-64 mt-1" />
          </CardHeader>
          <CardContent className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-10" />
                  <TableHead>{t("team.tableEmail")}</TableHead>
                  <TableHead>{t("team.tableRole")}</TableHead>
                  <TableHead>{t("team.tableJoinedAt")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                <TableSkeleton rows={3} cols={4} />
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </div>
    )
  }

  if (!team) {
    return (
      <div data-testid="team-page">
        {error && (
          <Alert variant="destructive" className="mb-4" data-testid="team-error-alert">
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        )}
        <EmptyState onCreateTeam={() => setShowCreateTeamDialog(true)} />

        <Dialog
          open={showCreateTeamDialog}
          onOpenChange={setShowCreateTeamDialog}
        >
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{t("team.createTeamTitle")}</DialogTitle>
              <DialogDescription>
                {t("team.createTeamDesc")}
              </DialogDescription>
            </DialogHeader>
            <div className="flex flex-col gap-3 mt-2">
              <Input
                placeholder={t("team.teamNamePlaceholder")}
                value={newTeamName}
                onChange={(e) => setNewTeamName(e.target.value)}
                data-testid="input-team-name"
              />
              <Input
                placeholder={t("team.teamSlugPlaceholder")}
                value={newTeamSlug}
                onChange={(e) => setNewTeamSlug(e.target.value)}
                data-testid="input-team-slug"
              />
              <Button
                onClick={handleCreateTeam}
                disabled={creatingTeam}
                data-testid="btn-confirm-create-team"
              >
                {creatingTeam ? t("team.creating") : t("team.createBtn")}
              </Button>
            </div>
          </DialogContent>
        </Dialog>
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-6" data-testid="team-page">
      {error && (
        <Alert variant="destructive" data-testid="team-error-alert">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      <Card data-testid="team-info-card">
        <CardHeader className="flex flex-row items-start justify-between space-y-0 pb-4">
          <div>
            <CardTitle className="text-base" data-testid="team-name">
              {team.name}
            </CardTitle>
            <CardDescription className="mt-1">
              {t("team.summary", { slug: team.slug, count: team.member_count, plan: team.plan })}
            </CardDescription>
          </div>
          <Button
            size="sm"
            variant="outline"
            onClick={() => setShowInviteDialog(true)}
            data-testid="btn-invite-member"
          >
            <UserPlus className="h-4 w-4 mr-1" />
            {t("team.invite")}
          </Button>
        </CardHeader>

        <CardContent className="overflow-x-auto">
          <Table data-testid="members-table">
            <TableHeader>
              <TableRow>
                <TableHead className="w-10" />
                <TableHead>{t("team.tableEmail")}</TableHead>
                <TableHead>{t("team.tableRole")}</TableHead>
                <TableHead>{t("team.tableJoinedAt")}</TableHead>
                <TableHead />
              </TableRow>
            </TableHeader>
            <TableBody>
              {loadingMembers ? (
                <TableSkeleton rows={3} cols={5} />
              ) : members.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={5} className="text-center text-xs text-muted-foreground py-6">
                    {t("team.noMembers")}
                  </TableCell>
                </TableRow>
              ) : (
                members.map((m) => (
                  <TableRow key={m.id} data-testid={`member-row-${m.id}`}>
                    <TableCell>
                      <MemberAvatar email={m.email} />
                    </TableCell>
                    <TableCell className="text-sm">{m.email}</TableCell>
                    <TableCell>
                      <RoleBadge role={m.role} />
                    </TableCell>
                    <TableCell className="text-xs text-muted-foreground">
                      {m.joined_at}
                    </TableCell>
                    <TableCell>
                      {m.role !== "owner" && (
                        <Button
                          size="sm"
                          variant="ghost"
                          className="text-destructive hover:text-destructive text-xs"
                          onClick={() => handleRemoveMember(m.user_id)}
                          data-testid={`btn-remove-member-${m.id}`}
                        >
                          {t("team.removeBtn")}
                        </Button>
                      )}
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      {(loadingInvitations || invitations.length > 0) && (
        <Card data-testid="pending-invitations-card">
          <CardHeader>
            <CardTitle className="text-base">{t("team.pendingInvitations")}</CardTitle>
          </CardHeader>
          <CardContent className="overflow-x-auto">
            <Table data-testid="invitations-table">
              <TableHeader>
                <TableRow>
                  <TableHead>{t("team.tableEmail")}</TableHead>
                  <TableHead>{t("team.tableRole")}</TableHead>
                  <TableHead>{t("team.tableExpiresAt")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {loadingInvitations ? (
                  <TableSkeleton rows={2} cols={3} />
                ) : (
                  invitations.map((inv) => (
                    <TableRow key={inv.id} data-testid={`invitation-row-${inv.id}`}>
                      <TableCell className="text-sm">{inv.email}</TableCell>
                      <TableCell>
                        <RoleBadge role={inv.role} />
                      </TableCell>
                      <TableCell className="text-xs text-muted-foreground">
                        {inv.expires_at}
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      )}

      <Card data-testid="team-api-keys-card">
        <CardHeader className="flex flex-row items-start justify-between space-y-0 pb-4">
          <div>
            <CardTitle className="text-base">{t("team.teamApiKeys")}</CardTitle>
            <CardDescription className="mt-1">
              {t("team.teamApiKeysDesc")}
            </CardDescription>
          </div>
          <Button
            size="sm"
            variant="outline"
            onClick={() => setShowAddKeyDialog(true)}
            data-testid="btn-add-team-key"
          >
            <Key className="h-4 w-4 mr-1" />
            {t("team.addKey")}
          </Button>
        </CardHeader>
        <CardContent className="overflow-x-auto">
          <Table data-testid="team-keys-table">
            <TableHeader>
              <TableRow>
                <TableHead>{t("team.tableName")}</TableHead>
                <TableHead>{t("team.tablePrefix")}</TableHead>
                <TableHead>{t("team.tableType")}</TableHead>
                <TableHead>{t("team.tableCreatedAt")}</TableHead>
                <TableHead />
              </TableRow>
            </TableHeader>
            <TableBody>
              {loadingKeys ? (
                <TableSkeleton rows={2} cols={5} />
              ) : teamKeys.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={5} className="text-center text-xs text-muted-foreground py-6">
                    {t("team.noKeys")}
                  </TableCell>
                </TableRow>
              ) : (
                teamKeys.map((k) => (
                  <TableRow key={k.id} data-testid={`key-row-${k.id}`}>
                    <TableCell className="text-sm font-medium">{k.name}</TableCell>
                    <TableCell className="font-mono text-xs text-muted-foreground">
                      {k.prefix}
                    </TableCell>
                    <TableCell>
                      <Badge
                        variant={k.key_type === "production" ? "default" : "secondary"}
                        className="text-xs"
                        data-testid={`key-type-badge-${k.id}`}
                      >
                        {k.key_type === "production" || k.key_type === "test"
                          ? t(`team.keyTypes.${k.key_type}`)
                          : k.key_type}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-xs text-muted-foreground">
                      {k.created_at}
                    </TableCell>
                    <TableCell>
                      <Button
                        size="sm"
                        variant="ghost"
                        className="text-destructive hover:text-destructive text-xs"
                        onClick={() => handleRevokeKey(k.id)}
                        data-testid={`btn-revoke-key-${k.id}`}
                      >
                        {t("team.revokeBtn")}
                      </Button>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      <Card data-testid="team-subscription-card">
        <CardHeader>
          <CardTitle className="text-base">{t("team.teamSubscription")}</CardTitle>
          <CardDescription>{t("team.teamSubscriptionDesc")}</CardDescription>
        </CardHeader>
        <CardContent className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <span className="text-sm">{t("team.currentPlan")}</span>
            <Badge
              variant={teamPlan === "agent_pro" ? "default" : "secondary"}
              data-testid="team-plan-badge"
            >
              {teamPlan === "agent_pro" ? "Agent Pro" : "Free"}
            </Badge>
          </div>
          {teamPlan !== "agent_pro" && (
            <Button size="sm" data-testid="btn-upgrade-team">
              {t("team.upgradeAgentPro")}
            </Button>
          )}
        </CardContent>
      </Card>

      <Dialog
        open={showAddKeyDialog}
        onOpenChange={(open) => {
          if (!open) {
            setShowAddKeyDialog(false)
            setCreatedKeyValue(null)
            setCopiedKey(false)
            setNewKeyName("")
            setNewKeyType("production")
          }
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("team.addKeyTitle")}</DialogTitle>
            {!createdKeyValue && (
              <DialogDescription>{t("team.addKeyDesc")}</DialogDescription>
            )}
          </DialogHeader>

          {createdKeyValue ? (
            <div className="flex flex-col gap-3 mt-2">
              <Alert data-testid="new-team-key-reveal">
                <AlertDescription className="space-y-2">
                  <p className="text-amber-600 dark:text-amber-400 font-medium text-sm">
                    {t("team.keyOnceWarning")}
                  </p>
                  <code className="block bg-muted p-2 rounded text-xs break-all" data-testid="new-team-key-value">
                    {createdKeyValue}
                  </code>
                </AlertDescription>
              </Alert>
              <div className="flex justify-end gap-2">
                <Button
                  variant="outline"
                  onClick={async () => {
                    await navigator.clipboard.writeText(createdKeyValue)
                    setCopiedKey(true)
                    setTimeout(() => setCopiedKey(false), 2000)
                  }}
                  data-testid="btn-copy-team-key"
                >
                  {copiedKey ? (
                    <><CheckCircle2 className="mr-2 h-4 w-4 text-green-500" />{t("team.copied")}</>
                  ) : (
                    <><Copy className="mr-2 h-4 w-4" />{t("team.copyKey")}</>
                  )}
                </Button>
                <Button
                  onClick={() => {
                    setShowAddKeyDialog(false)
                    setCreatedKeyValue(null)
                    setCopiedKey(false)
                    setNewKeyName("")
                    setNewKeyType("production")
                  }}
                  data-testid="btn-done-team-key"
                >
                  {t("team.done")}
                </Button>
              </div>
            </div>
          ) : (
            <div className="flex flex-col gap-3 mt-2">
              <Input
                placeholder={t("team.keyNamePlaceholder")}
                value={newKeyName}
                onChange={(e) => setNewKeyName(e.target.value)}
                data-testid="input-key-name"
              />
              <Select
                value={newKeyType}
                onValueChange={(v) => setNewKeyType(v as "production" | "test")}
              >
                <SelectTrigger data-testid="select-key-type">
                  <SelectValue placeholder={t("team.selectType")} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="production">{t("team.keyTypes.production")}</SelectItem>
                  <SelectItem value="test">{t("team.keyTypes.test")}</SelectItem>
                </SelectContent>
              </Select>
              <Button
                onClick={handleAddKey}
                disabled={addingKey || !newKeyName.trim()}
                data-testid="btn-confirm-add-key"
              >
                {addingKey ? t("team.creatingKey") : t("team.createKey")}
              </Button>
            </div>
          )}
        </DialogContent>
      </Dialog>

      <Dialog
        open={showInviteDialog}
        onOpenChange={(open) => {
          if (!open) {
            setShowInviteDialog(false)
            setCreatedInvite(null)
            setInviteEmail("")
            setInviteRole("member")
            setCopiedInviteLink(false)
            setCopiedInviteToken(false)
          }
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {createdInvite ? t("team.inviteCreatedTitle") : t("team.inviteTitle")}
            </DialogTitle>
            {!createdInvite && (
              <DialogDescription>{t("team.inviteDesc")}</DialogDescription>
            )}
          </DialogHeader>

          {createdInvite ? (
            <div className="flex flex-col gap-3 mt-2">
              <Alert data-testid="invite-reveal-alert">
                <AlertDescription className="space-y-3">
                  <p className="text-amber-600 dark:text-amber-400 font-medium text-sm">
                    {t("team.inviteOnceWarning")}
                  </p>

                  {createdInvite.url && (
                    <div className="space-y-1">
                      <p className="text-xs font-medium text-muted-foreground">
                        {t("team.inviteLinkLabel")}
                      </p>
                      <code
                        className="block bg-muted p-2 rounded text-xs break-all"
                        data-testid="invite-url-value"
                      >
                        {createdInvite.url}
                      </code>
                    </div>
                  )}

                  <div className="space-y-1">
                    <p className="text-xs font-medium text-muted-foreground">
                      {t("team.inviteTokenLabel")}
                    </p>
                    <code
                      className="block bg-muted p-2 rounded text-xs break-all"
                      data-testid="invite-token-value"
                    >
                      {createdInvite.token}
                    </code>
                  </div>
                </AlertDescription>
              </Alert>

              <div className="flex justify-end gap-2 flex-wrap">
                {createdInvite.url && (
                  <Button
                    variant="outline"
                    onClick={async () => {
                      await navigator.clipboard.writeText(createdInvite.url)
                      setCopiedInviteLink(true)
                      setTimeout(() => setCopiedInviteLink(false), 2000)
                    }}
                    data-testid="btn-copy-invite-link"
                  >
                    {copiedInviteLink ? (
                      <><CheckCircle2 className="mr-2 h-4 w-4 text-green-500" />{t("team.copied")}</>
                    ) : (
                      <><Copy className="mr-2 h-4 w-4" />{t("team.copyLink")}</>
                    )}
                  </Button>
                )}
                <Button
                  variant="outline"
                  onClick={async () => {
                    await navigator.clipboard.writeText(createdInvite.token)
                    setCopiedInviteToken(true)
                    setTimeout(() => setCopiedInviteToken(false), 2000)
                  }}
                  data-testid="btn-copy-invite-token"
                >
                  {copiedInviteToken ? (
                    <><CheckCircle2 className="mr-2 h-4 w-4 text-green-500" />{t("team.copied")}</>
                  ) : (
                    <><Copy className="mr-2 h-4 w-4" />{t("team.copyToken")}</>
                  )}
                </Button>
                <Button
                  onClick={() => {
                    setShowInviteDialog(false)
                    setCreatedInvite(null)
                    setInviteEmail("")
                    setInviteRole("member")
                    setCopiedInviteLink(false)
                    setCopiedInviteToken(false)
                  }}
                  data-testid="btn-done-invite"
                >
                  {t("team.done")}
                </Button>
              </div>
            </div>
          ) : (
            <div className="flex flex-col gap-3 mt-2">
              <Input
                type="email"
                placeholder={t("team.emailPlaceholder")}
                value={inviteEmail}
                onChange={(e) => setInviteEmail(e.target.value)}
                data-testid="input-invite-email"
              />
              <Select value={inviteRole} onValueChange={setInviteRole}>
                <SelectTrigger data-testid="select-invite-role">
                  <SelectValue placeholder={t("team.rolePlaceholder")} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="admin">{t("team.roles.admin")}</SelectItem>
                  <SelectItem value="member">{t("team.roles.member")}</SelectItem>
                </SelectContent>
              </Select>
              <Button
                onClick={handleInvite}
                disabled={inviting}
                data-testid="btn-confirm-invite"
              >
                {inviting ? t("team.sending") : t("team.sendInvite")}
              </Button>
            </div>
          )}
        </DialogContent>
      </Dialog>
    </div>
  )
}
