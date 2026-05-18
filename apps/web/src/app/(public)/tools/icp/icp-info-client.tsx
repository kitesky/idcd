"use client"

import { useTranslations } from "next-intl"
import { Card, CardContent, CardHeader, CardTitle, Badge } from "@/components/ui"
import { ToolQueryLayout } from "@/components/tools/ToolQueryLayout"
import { getICPInfo, type ICPInfo } from "@/lib/api"

export default function IcpInfoClient() {
  const t = useTranslations("tools.icp")
  const helpTips = t.raw("probe.helpTips") as string[]

  return (
    <ToolQueryLayout<ICPInfo>
      title={t("title")}
      description={t("description")}
      inputLabel={t("probe.label")}
      inputPlaceholder={t("probe.placeholder")}
      inputId="icp-query"
      onQuery={getICPInfo}
      renderResult={(result) => {
        const rows: [string, string][] = [
          [t("probe.result.rows.domain"), result.domain],
          [t("probe.result.rows.icpNumber"), result.icp_number],
          [t("probe.result.rows.company"), result.company],
          [t("probe.result.rows.type"), result.type],
          [t("probe.result.rows.filedAt"), result.filed_at],
        ]
        return (
          <Card>
            <CardHeader>
              <div className="flex items-center justify-between">
                <CardTitle>{t("probe.result.title")}</CardTitle>
                {result.icp_number ? (
                  <Badge variant="default">{result.icp_number}</Badge>
                ) : (
                  <Badge variant="secondary">{t("probe.result.noFiling")}</Badge>
                )}
              </div>
            </CardHeader>
            <CardContent className="space-y-3 text-sm">
              {rows.map(([k, v]) => (
                <div key={k} className="flex gap-2">
                  <span className="text-muted-foreground w-24 shrink-0 font-medium">
                    {k}
                  </span>
                  <span className="font-mono break-all">{v || "-"}</span>
                </div>
              ))}
              {result.note && (
                <div className="flex gap-2">
                  <span className="text-muted-foreground w-24 shrink-0 font-medium">
                    {t("probe.result.rows.note")}
                  </span>
                  <span className="text-muted-foreground break-all">
                    {result.note}
                  </span>
                </div>
              )}
            </CardContent>
          </Card>
        )
      }}
      tips={
        <>
          {helpTips.map((tip, i) => (
            <p key={i}>• {tip}</p>
          ))}
        </>
      }
    />
  )
}
