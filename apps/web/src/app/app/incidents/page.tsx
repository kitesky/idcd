"use client"

import { useState, useEffect } from "react"
import { useRouter } from "next/navigation"
import { useTranslations } from "next-intl"
import { FileWarning, Clock, Zap, AlertCircle } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { apiRequest } from "@/lib/api"

interface Incident {
  event_id: string
  monitor_id: string
  monitor_name: string
  status: string
  started_at: string
  resolved_at: string | null
  has_draft: boolean
}

function formatDate(isoString: string | null): string {
  if (!isoString) return "—"
  const d = new Date(isoString)
  if (isNaN(d.getTime())) return isoString
  const pad = (n: number) => String(n).padStart(2, "0")
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

function IncidentsTableSkeleton() {
  return (
    <div className="space-y-2" data-testid="incidents-skeleton">
      {[1, 2, 3, 4, 5].map((i) => (
        <div key={i} className="flex items-center gap-4 py-3 px-1">
          <Skeleton className="h-4 w-32" />
          <Skeleton className="h-4 w-36" />
          <Skeleton className="h-4 w-20" />
          <Skeleton className="h-5 w-12" />
          <Skeleton className="h-5 w-14" />
          <Skeleton className="h-8 w-20" />
        </div>
      ))}
    </div>
  )
}

export default function IncidentsPage() {
  const router = useRouter()
  const t = useTranslations("incidents")
  const [incidents, setIncidents] = useState<Incident[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [generating, setGenerating] = useState<string | null>(null)
  const [generateError, setGenerateError] = useState<string | null>(null)
  const [limit, setLimit] = useState(20)
  const [hasMore, setHasMore] = useState(false)
  const [loadingMore, setLoadingMore] = useState(false)

  useEffect(() => {
    async function fetchIncidents() {
      setLoading(true)
      setError(null)
      try {
        const res = await apiRequest<{ data: { incidents: Incident[] } }>(`/v1/incidents?limit=${limit}`)
        const list = res.data.incidents ?? []
        setIncidents(list)
        setHasMore(list.length === limit)
      } catch (err) {
        setError(err instanceof Error ? err.message : t("loadFailed"))
      } finally {
        setLoading(false)
      }
    }
    fetchIncidents()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [limit])

  async function handleLoadMore() {
    setLoadingMore(true)
    setLimit((l) => l + 20)
  }

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- incidents 异步加载完成后重置 loadingMore，无法 derive
    setLoadingMore(false)
  }, [incidents])

  async function handleGenerate(eventId: string) {
    setGenerating(eventId)
    setGenerateError(null)
    try {
      const res = await apiRequest<{ data: { id: string; title: string } }>(`/v1/incidents/${eventId}/draft`, {
        method: "POST",
      })
      router.push(`/app/incidents/${res.data.id}`)
    } catch (err) {
      setGenerateError(err instanceof Error ? err.message : t("generateFailed"))
      setGenerating(null)
    }
  }

  return (
    <div data-testid="incidents-page">
      <div className="mb-6 flex items-center gap-3">
        <FileWarning className="h-6 w-6 text-muted-foreground" />
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{t("title")}</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            {t("subtitle")}
          </p>
        </div>
      </div>

        {error && (
          <Alert variant="destructive" className="mb-6" data-testid="incidents-error-alert">
            <AlertCircle className="h-4 w-4" />
            <AlertTitle>{t("loadErrorTitle")}</AlertTitle>
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        )}

        {generateError && (
          <Alert variant="destructive" className="mb-6" data-testid="generate-error-alert">
            <AlertCircle className="h-4 w-4" />
            <AlertTitle>{t("operationErrorTitle")}</AlertTitle>
            <AlertDescription>{generateError}</AlertDescription>
          </Alert>
        )}

        <Card data-testid="incidents-table-card">
          <CardHeader>
            <CardTitle>{t("alertEvents")}</CardTitle>
          </CardHeader>
          <CardContent className="overflow-x-auto">
            {loading ? (
              <IncidentsTableSkeleton />
            ) : incidents.length === 0 ? (
              <div className="flex flex-col items-center justify-center py-12 text-center" data-testid="incidents-empty-state">
                <p className="text-sm text-muted-foreground">{t("empty")}</p>
              </div>
            ) : (
              <div className="space-y-4">
                <Table data-testid="incidents-table">
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t("table.monitor")}</TableHead>
                      <TableHead>{t("table.startedAt")}</TableHead>
                      <TableHead>{t("table.draftStatus")}</TableHead>
                      <TableHead>{t("table.actions")}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {incidents.map((incident) => (
                      <TableRow key={incident.event_id} data-testid={`incident-row-${incident.event_id}`}>
                        <TableCell className="font-medium">{incident.monitor_name}</TableCell>
                        <TableCell>
                          <span className="flex items-center gap-1 text-sm text-muted-foreground">
                            <Clock className="h-3 w-3" />
                            {formatDate(incident.started_at)}
                          </span>
                        </TableCell>
                        <TableCell>
                          {incident.has_draft ? (
                            <Badge variant="secondary" data-testid={`postmortem-status-${incident.event_id}`}>
                              {t("draftGenerated")}
                            </Badge>
                          ) : (
                            <Badge variant="outline" data-testid={`postmortem-status-${incident.event_id}`}>
                              {t("draftNotGenerated")}
                            </Badge>
                          )}
                        </TableCell>
                        <TableCell>
                          <Button
                            size="sm"
                            variant="outline"
                            onClick={() => handleGenerate(incident.event_id)}
                            disabled={generating === incident.event_id}
                            data-testid={`generate-btn-${incident.event_id}`}
                          >
                            <Zap className="mr-1 h-3 w-3" />
                            {generating === incident.event_id ? t("generating") : t("generate")}
                          </Button>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
                {hasMore && (
                  <div className="flex justify-center pt-2" data-testid="load-more-container">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={handleLoadMore}
                      disabled={loadingMore}
                      data-testid="load-more-btn"
                    >
                      {loadingMore ? t("loadingMore") : t("loadMore")}
                    </Button>
                  </div>
                )}
              </div>
            )}
          </CardContent>
        </Card>
    </div>
  )
}
