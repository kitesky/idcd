"use client"

import * as React from "react"
import { useTranslations } from "next-intl"
import { CheckCircle2, AlertTriangle, XCircle } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { cn } from "@/lib/utils"
import type { ServiceStatus } from "./types"

type OverallVariant = "success" | "warning" | "destructive" | "secondary"

interface OverallStatusConfig {
  label: string
  variant: OverallVariant
  icon: React.ReactNode
  bg: string
}

type PageT = ReturnType<typeof useTranslations<"status.page">>

function overallStatusConfig(status: ServiceStatus, t: PageT): OverallStatusConfig {
  switch (status) {
    case "operational":
      return {
        label: t("overall.operational"),
        variant: "success",
        icon: <CheckCircle2 className="h-6 w-6 text-success" />,
        bg: "bg-success/15 border-success/30",
      }
    case "degraded":
      return {
        label: t("overall.degraded"),
        variant: "warning",
        icon: <AlertTriangle className="h-6 w-6 text-warning" />,
        bg: "bg-warning/15 border-warning/30",
      }
    case "outage":
      return {
        label: t("overall.outage"),
        variant: "destructive",
        icon: <XCircle className="h-6 w-6 text-destructive" />,
        bg: "bg-destructive/15 border-destructive/30",
      }
    case "maintenance":
      return {
        label: t("overall.maintenance"),
        variant: "secondary",
        icon: <AlertTriangle className="h-6 w-6 text-info" />,
        bg: "bg-info/15 border-info/30",
      }
  }
}

export interface OverallBannerProps {
  /** Optional page title rendered above the status pill. Not rendered when omitted. */
  title?: string
  status: ServiceStatus
}

/**
 * GitHub Status–style top-of-page banner: shows an optional title and a large
 * status pill ("All Systems Operational" / degraded / outage / maintenance).
 *
 * Reads labels from the `status.page.overall.*` i18n namespace.
 */
export function OverallBanner({ title, status }: OverallBannerProps) {
  const t = useTranslations("status.page")
  const cfg = overallStatusConfig(status, t)

  return (
    <div className="mb-10 text-center">
      {title ? (
        <h1
          className="mb-4 text-3xl font-bold tracking-tight"
          data-testid="status-title"
        >
          {title}
        </h1>
      ) : null}
      <div
        className={cn(
          "inline-flex items-center gap-3 rounded-xl border px-6 py-3",
          cfg.bg,
        )}
        data-testid="overall-status"
      >
        {cfg.icon}
        <Badge variant={cfg.variant} className="px-3 py-1 text-sm">
          {cfg.label}
        </Badge>
      </div>
    </div>
  )
}
