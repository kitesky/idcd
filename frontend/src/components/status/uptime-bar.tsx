"use client"

import * as React from "react"
import { useTranslations } from "next-intl"
import { Card } from "@/components/ui/card"
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { cn } from "@/lib/utils"
import type { MonitorHistory, ServiceStatus } from "./types"

function uptimeDayColor(status: ServiceStatus): string {
  const colors: Record<ServiceStatus, string> = {
    operational: "bg-success",
    degraded: "bg-warning",
    outage: "bg-destructive",
    maintenance: "bg-info",
  }
  return colors[status]
}

export interface UptimeBarProps {
  /**
   * Day-by-day history. Length is rendered as-is (30 or 90 days, etc.).
   * The bar grid expands to fit the array length.
   */
  history: MonitorHistory[]
  /** Show legend + axis labels under the grid. Defaults to true. */
  showLegend?: boolean
  /** Optional heading rendered above the card. Omit to render only the card. */
  label?: string
}

/**
 * GitHub Status–style uptime bar: one rounded cell per day, color-coded by
 * status, with a hover tooltip showing "date + uptime%".
 *
 * Reads labels from the `status.page.uptime.*` i18n namespace and statusLabels
 * from `status.page.statusLabel.*`.
 */
export function UptimeBar({
  history,
  showLegend = true,
  label,
}: UptimeBarProps) {
  const t = useTranslations("status.page")
  const statusLabel = (s: ServiceStatus) => t(`statusLabel.${s}`)
  const columnCount = history.length > 0 ? history.length : 1

  return (
    <div data-testid="uptime-history">
      {label ? <h2 className="mb-4 text-lg font-semibold">{label}</h2> : null}
      <Card className="p-5">
        <TooltipProvider delayDuration={150}>
          <div
            className="grid gap-0.5"
            style={{ gridTemplateColumns: `repeat(${columnCount}, 1fr)` }}
            aria-label={t("uptime.gridAriaLabel")}
            data-testid="uptime-grid"
          >
            {history.map((day, i) => (
              <Tooltip key={`${day.date}-${i}`}>
                <TooltipTrigger asChild>
                  <div
                    role="img"
                    tabIndex={0}
                    aria-label={t("uptime.dayAriaLabel", {
                      date: day.date,
                      uptime: day.uptime.toFixed(1),
                      label: statusLabel(day.status),
                    })}
                    className={cn(
                      "h-5 w-full rounded-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
                      uptimeDayColor(day.status),
                    )}
                  />
                </TooltipTrigger>
                <TooltipContent side="top">
                  <div className="flex flex-col items-center gap-0.5">
                    <span className="font-medium">{day.date}</span>
                    <span>{day.uptime.toFixed(2)}%</span>
                  </div>
                </TooltipContent>
              </Tooltip>
            ))}
          </div>
        </TooltipProvider>
        {showLegend ? (
          <div className="mt-3 flex items-center justify-between text-xs text-muted-foreground">
            <span>{t("uptime.axisLeft")}</span>
            <div className="flex items-center gap-3">
              {[
                { cls: "bg-success", label: t("uptime.legendOperational") },
                { cls: "bg-warning", label: t("uptime.legendDegraded") },
                { cls: "bg-destructive", label: t("uptime.legendOutage") },
              ].map(({ cls, label: legendLabel }) => (
                <span key={legendLabel} className="flex items-center gap-1">
                  <span
                    className={cn("inline-block h-2.5 w-2.5 rounded-sm", cls)}
                  />
                  {legendLabel}
                </span>
              ))}
            </div>
            <span>{t("uptime.axisRight")}</span>
          </div>
        ) : null}
      </Card>
    </div>
  )
}
