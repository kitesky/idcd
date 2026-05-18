"use client"

import { useTranslations } from "next-intl"
import { Card, CardContent, CardHeader, CardTitle, Badge } from "@/components/ui"
import { ToolQueryLayout } from "@/components/tools/ToolQueryLayout"
import { apiRequest } from "@/lib/api"

interface IPInfo {
  ip: string
  country: string
  city: string
  asn: string
  isp: string
  is_datacenter: boolean
  is_proxy: boolean
}

function getIPInfo(q: string): Promise<IPInfo> {
  return apiRequest<IPInfo>(`/v1/info/ip?q=${encodeURIComponent(q)}`)
}

export default function IpInfoClient() {
  const t = useTranslations("tools.ip")
  const helpTips = t.raw("probe.helpTips") as string[]

  return (
    <ToolQueryLayout<IPInfo>
      title={t("title")}
      description={t("description")}
      inputLabel={t("probe.label")}
      inputPlaceholder={t("probe.placeholder")}
      inputId="ip-query"
      onQuery={getIPInfo}
      renderResult={(result) => {
        const rows: [string, string][] = [
          [t("probe.result.rows.ip"), result.ip],
          [t("probe.result.rows.country"), result.country],
          [t("probe.result.rows.city"), result.city],
          [t("probe.result.rows.asn"), result.asn],
          [t("probe.result.rows.isp"), result.isp],
        ]
        return (
          <Card>
            <CardHeader>
              <div className="flex items-center justify-between">
                <CardTitle>{t("probe.result.title")}</CardTitle>
                <div className="flex gap-2">
                  {result.is_datacenter && (
                    <Badge variant="secondary">
                      {t("probe.result.badge.datacenter")}
                    </Badge>
                  )}
                  {result.is_proxy && (
                    <Badge variant="destructive">
                      {t("probe.result.badge.proxy")}
                    </Badge>
                  )}
                </div>
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
