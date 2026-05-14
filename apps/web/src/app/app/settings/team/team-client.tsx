"use client"

import { useEffect, useState } from "react"
import {
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
} from "@/components/ui"
import { Alert, AlertDescription, Skeleton } from "@/components/ui"
import { Key, Plus, UserPlus, Users } from "lucide-react"
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
  if (role === "owner") {
    return (
      <Badge
        variant="outline"
        className="border-purple-400 text-purple-500 text-xs"
        data-testid={`role-badge-${role}`}
      >
        owner
      </Badge>
    )
  }
  if (role === "admin") {
    return (
      <Badge
        variant="outline"
        className="border-blue-400 text-blue-500 text-xs"
        data-testid={`role-badge-${role}`}
      >
        admin
      </Badge>
    )
  }
  return (
    <Badge
      variant="outline"
      className="border-muted-foreground text-muted-foreground text-xs"
      data-testid={`role-badge-${role}`}
    >
      member
    </Badge>
  )
}

function MemberAvatar({ email }: { email: string }) {
  const initials = email.slice(0, 2).toUpperCase()
  return (
    <div
      className="flex h-8 w-8 items-center justify-center rounded-full bg-muted text-xs font-medium text-muted-foreground"
      aria-label={email}
    >
      {initials}
    </div>
  )
}

function EmptyState({ onCreateTeam }: { onCreateTeam: () => void }) {
  return (
    <div
      className="flex flex-col items-center justify-center py-16 gap-4"
      data-testid="team-empty-state"
    >
      <div className="flex h-14 w-14 items-center justify-center rounded-full bg-muted">
        <Users className="h-7 w-7 text-muted-foreground" />
      </div>
      <div className="text-center">
        <p className="text-sm font-medium">暂无团队</p>
        <p className="text-xs text-muted-foreground mt-1">
          创建一个团队，与成员共享资源
        </p>
      </div>
      <Button
        size="sm"
        onClick={onCreateTeam}
        data-testid="btn-create-team-empty"
      >
        <Plus className="h-4 w-4 mr-1" />
        创建新团队
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

  const [showAddKeyDialog, setShowAddKeyDialog] = useState(false)
  const [newKeyName, setNewKeyName] = useState("")
  const [newKeyType, setNewKeyType] = useState<"production" | "test">("production")
  const [addingKey, setAddingKey] = useState(false)

  // Load team on mount
  useEffect(() => {
    setLoadingTeam(true)
    apiRequest<{ data: { teams: Team[] } }>("/v1/teams")
      .then((json) => {
        const teams = json.data?.teams ?? []
        if (teams.length > 0) {
          const t = teams[0]
          setTeam(t)
          setTeamPlan(t.plan)
          loadTeamDetails(t.id)
        }
      })
      .catch((err) => setError(err.message ?? "加载团队失败"))
      .finally(() => setLoadingTeam(false))
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
    setCreatingTeam(true)
    try {
      const json = await apiRequest<{ data: { team: Team } }>("/v1/teams", {
        method: "POST",
        body: JSON.stringify({ name: newTeamName, slug: newTeamSlug }),
      })
      const t = json.data?.team
      if (t) {
        setTeam(t)
        setTeamPlan(t.plan)
        loadTeamDetails(t.id)
      }
      setNewTeamName("")
      setNewTeamSlug("")
      setShowCreateTeamDialog(false)
    } catch (err: any) {
      setError(err.message ?? "创建团队失败")
    } finally {
      setCreatingTeam(false)
    }
  }

  async function handleInvite() {
    if (!inviteEmail.trim() || !team) return
    setInviting(true)
    try {
      const json = await apiRequest<{ data: { invitation: PendingInvitation } }>(
        `/v1/teams/${team.id}/invitations`,
        {
          method: "POST",
          body: JSON.stringify({ email: inviteEmail, role: inviteRole }),
        }
      )
      const inv = json.data?.invitation
      if (inv) {
        setInvitations((prev) => [...prev, inv])
      }
      setInviteEmail("")
      setInviteRole("member")
      setShowInviteDialog(false)
    } catch (err: any) {
      setError(err.message ?? "邀请成员失败")
    } finally {
      setInviting(false)
    }
  }

  async function handleAddKey() {
    if (!newKeyName.trim() || !team) return
    setAddingKey(true)
    try {
      const json = await apiRequest<{ data: { api_key: TeamAPIKey } }>(
        `/v1/teams/${team.id}/api-keys`,
        {
          method: "POST",
          body: JSON.stringify({ name: newKeyName, key_type: newKeyType }),
        }
      )
      const key = json.data?.api_key
      if (key) {
        setTeamKeys((prev) => [...prev, key])
      }
      setNewKeyName("")
      setNewKeyType("production")
      setShowAddKeyDialog(false)
    } catch (err: any) {
      setError(err.message ?? "创建 API Key 失败")
    } finally {
      setAddingKey(false)
    }
  }

  async function handleRevokeKey(keyID: string) {
    if (!team) return
    try {
      await apiRequest(`/v1/teams/${team.id}/api-keys/${keyID}`, { method: "DELETE" })
      setTeamKeys((prev) => prev.filter((k) => k.id !== keyID))
    } catch (err: any) {
      setError(err.message ?? "撤销 API Key 失败")
    }
  }

  async function handleRemoveMember(userId: string) {
    if (!team) return
    try {
      await apiRequest(`/v1/teams/${team.id}/members/${userId}`, { method: "DELETE" })
      setMembers((prev) => prev.filter((m) => m.user_id !== userId))
      setTeam((prev) => prev ? { ...prev, member_count: prev.member_count - 1 } : prev)
    } catch (err: any) {
      setError(err.message ?? "移除成员失败")
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
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-10" />
                  <TableHead>邮箱</TableHead>
                  <TableHead>角色</TableHead>
                  <TableHead>加入时间</TableHead>
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
              <DialogTitle>创建新团队</DialogTitle>
              <DialogDescription>
                输入团队名称和唯一标识符
              </DialogDescription>
            </DialogHeader>
            <div className="flex flex-col gap-3 mt-2">
              <Input
                placeholder="团队名称"
                value={newTeamName}
                onChange={(e) => setNewTeamName(e.target.value)}
                data-testid="input-team-name"
              />
              <Input
                placeholder="slug（URL 标识符）"
                value={newTeamSlug}
                onChange={(e) => setNewTeamSlug(e.target.value)}
                data-testid="input-team-slug"
              />
              <Button
                onClick={handleCreateTeam}
                disabled={creatingTeam}
                data-testid="btn-confirm-create-team"
              >
                {creatingTeam ? "创建中..." : "创建团队"}
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
              slug: {team.slug} · {team.member_count} 名成员 · {team.plan} 计划
            </CardDescription>
          </div>
          <Button
            size="sm"
            variant="outline"
            onClick={() => setShowInviteDialog(true)}
            data-testid="btn-invite-member"
          >
            <UserPlus className="h-4 w-4 mr-1" />
            邀请成员
          </Button>
        </CardHeader>

        <CardContent>
          <Table data-testid="members-table">
            <TableHeader>
              <TableRow>
                <TableHead className="w-10" />
                <TableHead>邮箱</TableHead>
                <TableHead>角色</TableHead>
                <TableHead>加入时间</TableHead>
                <TableHead />
              </TableRow>
            </TableHeader>
            <TableBody>
              {loadingMembers ? (
                <TableSkeleton rows={3} cols={5} />
              ) : members.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={5} className="text-center text-xs text-muted-foreground py-6">
                    暂无成员
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
                          移除
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
            <CardTitle className="text-base">待处理邀请</CardTitle>
          </CardHeader>
          <CardContent>
            <Table data-testid="invitations-table">
              <TableHeader>
                <TableRow>
                  <TableHead>邮箱</TableHead>
                  <TableHead>角色</TableHead>
                  <TableHead>过期时间</TableHead>
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
            <CardTitle className="text-base">团队 API Keys</CardTitle>
            <CardDescription className="mt-1">
              用于 CI/CD 或自动化集成的团队级密钥
            </CardDescription>
          </div>
          <Button
            size="sm"
            variant="outline"
            onClick={() => setShowAddKeyDialog(true)}
            data-testid="btn-add-team-key"
          >
            <Key className="h-4 w-4 mr-1" />
            添加 Key
          </Button>
        </CardHeader>
        <CardContent>
          <Table data-testid="team-keys-table">
            <TableHeader>
              <TableRow>
                <TableHead>名称</TableHead>
                <TableHead>前缀</TableHead>
                <TableHead>类型</TableHead>
                <TableHead>创建时间</TableHead>
                <TableHead />
              </TableRow>
            </TableHeader>
            <TableBody>
              {loadingKeys ? (
                <TableSkeleton rows={2} cols={5} />
              ) : teamKeys.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={5} className="text-center text-xs text-muted-foreground py-6">
                    暂无团队 API Key
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
                        {k.key_type}
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
                        撤销
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
          <CardTitle className="text-base">团队订阅</CardTitle>
          <CardDescription>管理团队的 Agent Pro 订阅</CardDescription>
        </CardHeader>
        <CardContent className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <span className="text-sm">当前套餐</span>
            <Badge
              variant={teamPlan === "agent_pro" ? "default" : "secondary"}
              data-testid="team-plan-badge"
            >
              {teamPlan === "agent_pro" ? "Agent Pro" : "Free"}
            </Badge>
          </div>
          {teamPlan !== "agent_pro" && (
            <Button size="sm" data-testid="btn-upgrade-team">
              升级到 Agent Pro（¥299/月）
            </Button>
          )}
        </CardContent>
      </Card>

      <Dialog open={showAddKeyDialog} onOpenChange={setShowAddKeyDialog}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>添加团队 API Key</DialogTitle>
            <DialogDescription>创建一个团队共享的 API 密钥</DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-3 mt-2">
            <Input
              placeholder="Key 名称（如 CI/CD Key）"
              value={newKeyName}
              onChange={(e) => setNewKeyName(e.target.value)}
              data-testid="input-key-name"
            />
            <Select
              value={newKeyType}
              onValueChange={(v) => setNewKeyType(v as "production" | "test")}
            >
              <SelectTrigger data-testid="select-key-type">
                <SelectValue placeholder="选择类型" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="production">production</SelectItem>
                <SelectItem value="test">test</SelectItem>
              </SelectContent>
            </Select>
            <Button
              onClick={handleAddKey}
              disabled={addingKey}
              data-testid="btn-confirm-add-key"
            >
              {addingKey ? "创建中..." : "创建 Key"}
            </Button>
          </div>
        </DialogContent>
      </Dialog>

      <Dialog open={showInviteDialog} onOpenChange={setShowInviteDialog}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>邀请成员</DialogTitle>
            <DialogDescription>
              输入邮箱地址并选择角色
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-3 mt-2">
            <Input
              type="email"
              placeholder="邮箱地址"
              value={inviteEmail}
              onChange={(e) => setInviteEmail(e.target.value)}
              data-testid="input-invite-email"
            />
            <Select value={inviteRole} onValueChange={setInviteRole}>
              <SelectTrigger data-testid="select-invite-role">
                <SelectValue placeholder="选择角色" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="admin">admin</SelectItem>
                <SelectItem value="member">member</SelectItem>
              </SelectContent>
            </Select>
            <Button
              onClick={handleInvite}
              disabled={inviting}
              data-testid="btn-confirm-invite"
            >
              {inviting ? "发送中..." : "发送邀请"}
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}
