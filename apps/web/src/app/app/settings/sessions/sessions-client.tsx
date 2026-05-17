"use client"

import { useState, useEffect } from "react"
import { useTranslations } from "next-intl"
import {
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  Badge,
  Skeleton,
  AlertDialog,
  AlertDialogTrigger,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogFooter,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogAction,
  AlertDialogCancel,
} from "@/components/ui"
import { Monitor, Smartphone, Laptop } from "lucide-react"
import { apiRequest } from "@/lib/api"

// ── Types ─────────────────────────────────────────────────────────────────────

interface Session {
  id: string
  created_at: string
  last_seen_at?: string
  is_current: boolean
  user_agent?: string
}

// ── API helpers ───────────────────────────────────────────────────────────────

async function fetchSessions(): Promise<Session[]> {
  const body = await apiRequest<{ data: { sessions: Session[] } }>("/v1/account/sessions")
  return body.data?.sessions ?? []
}

async function revokeSession(id: string): Promise<void> {
  await apiRequest(`/v1/account/sessions/${id}`, { method: "DELETE" })
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
  if (diffDay < 30) return `${diffDay} 天前`
  const diffMo = Math.floor(diffDay / 30)
  return `${diffMo} 个月前`
}


function formatDateTime(iso: string): string {
  return new Date(iso).toLocaleString("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  })
}

/** Detect device type from User-Agent string. */
function detectDeviceType(ua?: string): "mobile" | "desktop" | "unknown" {
  if (!ua) return "unknown"
  const lower = ua.toLowerCase()
  if (/iphone|ipad|ipod|android|mobile|blackberry|windows phone/.test(lower)) {
    return "mobile"
  }
  return "desktop"
}

function DeviceIcon({
  ua,
  isCurrent,
  className,
}: {
  ua?: string
  isCurrent?: boolean
  className?: string
}) {
  const type = detectDeviceType(ua)
  if (type === "mobile") return <Smartphone className={className} aria-hidden />
  if (type === "desktop") return <Laptop className={className} aria-hidden />
  // No UA info: current session shows Monitor (this device), others show Laptop
  if (isCurrent) return <Monitor className={className} aria-hidden />
  return <Laptop className={className} aria-hidden />
}

// ── SessionsClient ────────────────────────────────────────────────────────────

export function SessionsClient() {
  const t = useTranslations("settings")
  const [sessions, setSessions] = useState<Session[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [revoking, setRevoking] = useState<string | null>(null)
  const [revokingAll, setRevokingAll] = useState(false)
  const [revokeError, setRevokeError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    // eslint-disable-next-line react-hooks/set-state-in-effect -- 初次挂载重置 loading，随后异步 fetch
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
      const msg = e instanceof Error ? e.message : t("sessions.revokeError")
      setRevokeError(msg)
    } finally {
      setRevoking(null)
    }
  }

  async function handleRevokeAll() {
    setRevokingAll(true)
    setRevokeError(null)
    const otherSessions = sessions.filter((s) => !s.is_current)
    const results = await Promise.allSettled(
      otherSessions.map((s) => revokeSession(s.id))
    )
    const failedCount = results.filter((r) => r.status === "rejected").length
    if (failedCount > 0) {
      setRevokeError(`${t("sessions.revokeError")} (${failedCount})`)
    }
    // Refresh list regardless to show accurate state
    try {
      const fresh = await fetchSessions()
      setSessions(fresh)
    } catch {
      setSessions((prev) => prev.filter((s) => s.is_current))
    }
    setRevokingAll(false)
  }

  const otherSessionCount = sessions.filter((s) => !s.is_current).length
  const hasOtherSessions = otherSessionCount > 0

  return (
    <div data-testid="sessions-page" className="space-y-6">
      <Card data-testid="sessions-card">
        <CardHeader className="flex flex-row items-start justify-between gap-4">
          <div className="space-y-1">
            <CardTitle>{t("sessions.title")}</CardTitle>
            <CardDescription>{t("sessions.desc")}</CardDescription>
          </div>

          {/* Revoke all button — only shown when there are non-current sessions */}
          {!loading && !error && hasOtherSessions && (
            <AlertDialog>
              <AlertDialogTrigger asChild>
                <Button
                  variant="outline"
                  size="sm"
                  className="text-destructive border-destructive/40 hover:bg-destructive/10 hover:text-destructive shrink-0"
                  disabled={revokingAll}
                  data-testid="btn-revoke-all"
                >
                  {revokingAll ? t("sessions.revokingAll") : t("sessions.revokeAll")}
                </Button>
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>{t("sessions.revokeAllTitle")}</AlertDialogTitle>
                  <AlertDialogDescription>
                    将撤销除当前会话外的其他 {otherSessionCount} 个活跃会话。其他设备上的登录状态将立即失效，此操作不可撤回。
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>{t("sessions.revokeAllCancel")}</AlertDialogCancel>
                  <AlertDialogAction
                    className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                    onClick={handleRevokeAll}
                  >
                    {t("sessions.revokeAllConfirm")}
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          )}
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
              {t("sessions.empty")}
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
                    <DeviceIcon
                      ua={sess.user_agent}
                      isCurrent={sess.is_current}
                      className="h-5 w-5 text-muted-foreground"
                    />
                  </div>

                  {/* Session info */}
                  <div className="flex-1 min-w-0 space-y-0.5">
                    <div className="flex items-center gap-2 flex-wrap">
                      <span className="text-sm font-medium truncate">
                        {sess.user_agent
                          ? sess.user_agent.split(" ").slice(0, 3).join(" ")
                          : sess.is_current
                          ? t("sessions.currentDevice")
                          : `${t("sessions.current")} ${sess.id.slice(0, 8)}…`}
                      </span>
                      {sess.is_current && (
                        <Badge
                          variant="secondary"
                          className="text-xs shrink-0"
                          data-testid={`badge-current-${sess.id}`}
                        >
                          {t("sessions.current")}
                        </Badge>
                      )}
                    </div>
                    <p
                      className="text-xs text-muted-foreground"
                      title={formatDateTime(sess.created_at)}
                    >
                      {t("sessions.loginAt")} {formatRelativeTime(sess.created_at)}
                      {sess.last_seen_at && sess.last_seen_at !== sess.created_at && (
                        <span className="ml-2 text-muted-foreground/70">
                          {t("sessions.lastSeen")} {formatRelativeTime(sess.last_seen_at)}
                        </span>
                      )}
                    </p>
                  </div>

                  {/* Revoke button */}
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={sess.is_current || revoking === sess.id || revokingAll}
                    data-testid={`btn-revoke-${sess.id}`}
                    onClick={() => handleRevoke(sess.id)}
                  >
                    {revoking === sess.id ? t("sessions.revoking") : t("sessions.revoke")}
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
