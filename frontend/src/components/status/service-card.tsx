"use client"

import * as React from "react"
import { useTranslations } from "next-intl"
import { Card, CardContent } from "@/components/ui/card"
import { cn } from "@/lib/utils"
import { UptimeBar } from "./uptime-bar"
import type { MonitorHistory, ServiceStatus } from "./types"

function statusDotColor(status: ServiceStatus): string {
  const colors: Record<ServiceStatus, string> = {
    operational: "bg-success",
    degraded: "bg-warning",
    outage: "bg-destructive",
    maintenance: "bg-info",
  }
  return colors[status]
}

export interface ServiceCardProps {
  name: string
  status: ServiceStatus
  uptimePercent?: number
  /** When provided, renders an UptimeBar below the header row. */
  history?: MonitorHistory[]
  /** Optional secondary line beneath the service name. */
  description?: string
}

/**
 * Flat, single-service card: name + status dot + uptime% on top, optional
 * uptime bar below. Equivalent to the per-monitor row used in the customer
 * status page, but promoted to its own card for the internal self-status
 * page (no folding/expansion).
 */
export function ServiceCard({
  name,
  status,
  uptimePercent,
  history,
  description,
}: ServiceCardProps) {
  const t = useTranslations("status.page")
  const statusLabel = t(`statusLabel.${status}`)

  return (
    <Card data-testid={`service-card-${name}`}>
      <CardContent className="p-5">
        <div className="flex items-center justify-between gap-3">
          <div className="min-w-0">
            <div className="truncate text-base font-semibold">{name}</div>
            {description ? (
              <div className="mt-0.5 truncate text-xs text-muted-foreground">
                {description}
              </div>
            ) : null}
          </div>
          <div className="flex shrink-0 items-center gap-2">
            {typeof uptimePercent === "number" ? (
              <span className="text-xs text-muted-foreground">
                {uptimePercent.toFixed(2)}%
              </span>
            ) : null}
            <span
              role="img"
              aria-label={statusLabel}
              className={cn(
                "inline-block h-2.5 w-2.5 rounded-full",
                statusDotColor(status),
              )}
            />
          </div>
        </div>
        {history && history.length > 0 ? (
          <div className="mt-4">
            <UptimeBar history={history} showLegend={false} />
          </div>
        ) : null}
      </CardContent>
    </Card>
  )
}
