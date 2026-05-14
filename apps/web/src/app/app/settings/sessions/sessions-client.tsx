"use client"

import { useState, useEffect } from "react"
import {
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  Badge,
  Skeleton,
} from "@/components/ui"
import { Monitor, Smartphone, Laptop } from "lucide-react"

// ── Types ─────────────────────────────────────────────────────────────────────

interface Session {
  id: string
  created_at: string
  is_current: boolean
}

// ── API helpers ───────────────────────────────────────────────────────────────

async function fetchSessions(): Promise<Session[]> {
  const res = await fetch("/api/v1/account/sessions", { credentials: "include" })
  if (!res.ok) throw new Error("failed to load sessions")
  const body = await res.json()
  return (body.data?.sessions ?? []) as Session[]
}

async function revokeSession(id: string): Promise<void> {
  const res = await fetch(`/api/v1/account/sessions/${id}`, {
    method: "DELETE",
    credentials: "include",
  })
  if (!res.ok && res.status !== 204) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body?.error?.message ?? "failed to revoke session")
  }
}

// ── Utilities ─────────────────────────────────────────────────────────────────

function formatRelativeTime(iso: string): string {
  const date = new Date(iso)
  const diffMs = Date.now() - date.getTime()
  const diffMin = Math.floor(diffMs / 60_000)
  if (diffMin < 1) return "刚刚"
  if (diffMin < 60) return `${diffMin} 分钟前`
  const diffHr = Math.floor(diffMin / 60)
  if (diffHr < 24) return `${diffHr} 小时前`
  const diffDay = Math.floor(diffHr / 24)
  return `${diffDay} 天前`
}

function DeviceIcon({ className }: { className?: string }) {
  return <Monitor className={className} aria-hidden />
}

// ── SessionsClient ────────────────────────────────────────────────────────────

export function SessionsClient() {
  const [sessions, setSessions] = useState<Session[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [revoking, setRevoking] = useState<string | null>(null)
  const [revokeError, setRevokeError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    fetchSessions()
      .then((data) => {
        if (!cancelled) setSessions(data)
      })
      .catch((e) => {
        if (!cancelled) setError(e.message ?? "加载失败")
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [])

  async function handleRevoke(id: string) {
    setRevoking(id)
    setRevokeError(null)
    try {
      await revokeSession(id)
      setSessions((prev) => prev.filter((s) => s.id !== id))
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : "撤销失败，请稍后重试"
      setRevokeError(msg)
    } finally {
      setRevoking(null)
    }
  }

  return (
    <div data-testid="sessions-page" className="space-y-6">
      <Card data-testid="sessions-card">
        <CardHeader>
          <CardTitle>活跃会话</CardTitle>
          <CardDescription>以下是您账号上目前活跃的登录会话</CardDescription>
        </CardHeader>
        <CardContent>
          {loading ? (
            <div className="space-y-3" data-testid="sessions-skeleton">
              {[1, 2, 3].map((i) => (
                <div key={i} className="flex items-center gap-4">
                  <Skeleton className="h-10 w-10 rounded-full" />
                  <div className="flex-1 space-y-2">
                    <Skeleton className="h-4 w-1/3" />
                    <Skeleton className="h-3 w-1/4" />
                  </div>
                  <Skeleton className="h-8 w-16" />
                </div>
              ))}
            </div>
          ) : error ? (
            <p
              className="text-sm text-destructive py-4 text-center"
              data-testid="sessions-error"
            >
              {error}
            </p>
          ) : sessions.length === 0 ? (
            <p
              className="text-sm text-muted-foreground py-4 text-center"
              data-testid="sessions-empty"
            >
              暂无活跃会话
            </p>
          ) : (
            <ul className="divide-y" data-testid="sessions-list">
              {sessions.map((sess) => (
                <li
                  key={sess.id}
                  className="flex items-center gap-4 py-4 first:pt-0 last:pb-0"
                  data-testid={`session-row-${sess.id}`}
                >
                  {/* Device icon */}
                  <div className="flex h-10 w-10 items-center justify-center rounded-full bg-muted shrink-0">
                    <DeviceIcon className="h-5 w-5 text-muted-foreground" />
                  </div>

                  {/* Session info */}
                  <div className="flex-1 min-w-0 space-y-0.5">
                    <div className="flex items-center gap-2 flex-wrap">
                      <span className="text-sm font-medium truncate">
                        {sess.id}
                      </span>
                      {sess.is_current && (
                        <Badge
                          variant="secondary"
                          className="text-xs shrink-0"
                          data-testid={`badge-current-${sess.id}`}
                        >
                          当前会话
                        </Badge>
                      )}
                    </div>
                    <p
                      className="text-xs text-muted-foreground"
                      title={new Date(sess.created_at).toLocaleString("zh-CN")}
                    >
                      登录于 {formatRelativeTime(sess.created_at)}
                    </p>
                  </div>

                  {/* Revoke button */}
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={sess.is_current || revoking === sess.id}
                    data-testid={`btn-revoke-${sess.id}`}
                    onClick={() => handleRevoke(sess.id)}
                  >
                    {revoking === sess.id ? "撤销中..." : "撤销"}
                  </Button>
                </li>
              ))}
            </ul>
          )}

          {revokeError && (
            <p
              className="mt-3 text-sm text-destructive"
              data-testid="revoke-error"
            >
              {revokeError}
            </p>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
