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
import { Plus, UserPlus, Users } from "lucide-react"

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

const MOCK_TEAM: Team = {
  id: "team_acme01",
  name: "Acme Corp",
  slug: "acme",
  plan: "free",
  member_count: 3,
}

const MOCK_MEMBERS: TeamMember[] = [
  {
    id: "tmb_001",
    user_id: "u_owner01",
    email: "alice@acme.com",
    role: "owner",
    joined_at: "2025-03-01",
  },
  {
    id: "tmb_002",
    user_id: "u_admin01",
    email: "bob@acme.com",
    role: "admin",
    joined_at: "2025-03-15",
  },
  {
    id: "tmb_003",
    user_id: "u_member01",
    email: "carol@acme.com",
    role: "member",
    joined_at: "2025-04-01",
  },
]

const MOCK_INVITATIONS: PendingInvitation[] = [
  {
    id: "tinv_001",
    email: "dave@acme.com",
    role: "member",
    expires_at: "2026-05-21",
  },
]

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

export function TeamClient() {
  const [team, setTeam] = useState<Team | null>(MOCK_TEAM)
  const [members] = useState<TeamMember[]>(MOCK_MEMBERS)
  const [invitations] = useState<PendingInvitation[]>(MOCK_INVITATIONS)

  const [showCreateTeamDialog, setShowCreateTeamDialog] = useState(false)
  const [newTeamName, setNewTeamName] = useState("")
  const [newTeamSlug, setNewTeamSlug] = useState("")

  const [showInviteDialog, setShowInviteDialog] = useState(false)
  const [inviteEmail, setInviteEmail] = useState("")
  const [inviteRole, setInviteRole] = useState("member")

  function handleCreateTeam() {
    if (!newTeamName.trim() || !newTeamSlug.trim()) return
    setTeam({
      id: "team_new01",
      name: newTeamName,
      slug: newTeamSlug,
      plan: "free",
      member_count: 1,
    })
    setNewTeamName("")
    setNewTeamSlug("")
    setShowCreateTeamDialog(false)
  }

  function handleInvite() {
    if (!inviteEmail.trim()) return
    setInviteEmail("")
    setInviteRole("member")
    setShowInviteDialog(false)
  }

  if (!team) {
    return (
      <div data-testid="team-page">
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
              <Button onClick={handleCreateTeam} data-testid="btn-confirm-create-team">
                创建团队
              </Button>
            </div>
          </DialogContent>
        </Dialog>
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-6" data-testid="team-page">
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
              </TableRow>
            </TableHeader>
            <TableBody>
              {members.map((m) => (
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
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      {invitations.length > 0 && (
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
                {invitations.map((inv) => (
                  <TableRow key={inv.id} data-testid={`invitation-row-${inv.id}`}>
                    <TableCell className="text-sm">{inv.email}</TableCell>
                    <TableCell>
                      <RoleBadge role={inv.role} />
                    </TableCell>
                    <TableCell className="text-xs text-muted-foreground">
                      {inv.expires_at}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      )}

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
            <Button onClick={handleInvite} data-testid="btn-confirm-invite">
              发送邀请
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}
