"use client"

import { useTranslations } from "next-intl"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import type { WhoisInfo, ICPInfo, SSLInfo, DNSInfo } from "@/lib/api"

export type DomainResult =
  | { kind: "whois"; data: WhoisInfo }
  | { kind: "icp"; data: ICPInfo }
  | { kind: "ssl"; data: SSLInfo }
  | { kind: "dns"; data: DNSInfo }

interface DomainResultPanelProps {
  result: DomainResult
}

export function DomainResultPanel({ result }: DomainResultPanelProps) {
  switch (result.kind) {
    case "whois":
      return <WhoisCard data={result.data} />
    case "icp":
      return <IcpCard data={result.data} />
    case "ssl":
      return <SslCard data={result.data} />
    case "dns":
      return <DnsCard data={result.data} />
  }
}

function Row({ label, value }: { label: string; value?: React.ReactNode }) {
  return (
    <div className="flex gap-2">
      <span className="text-muted-foreground w-24 shrink-0 font-medium">{label}</span>
      <span className="font-mono break-all">{value || "-"}</span>
    </div>
  )
}

function WhoisCard({ data }: { data: WhoisInfo }) {
  const t = useTranslations("tools.whois.probe.result")
  const rows: [string, string | undefined][] = [
    [t("rows.domain"), data.domain],
    [t("rows.registrar"), data.registrar],
    [t("rows.createdAt"), data.creation_date],
    [t("rows.expiresAt"), data.expiry_date ?? data.expiration_date],
  ]
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("title")}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3 text-sm">
        {rows.map(([k, v]) => (
          <Row key={k} label={k} value={v} />
        ))}
        {data.name_servers && data.name_servers.length > 0 && (
          <div className="flex gap-2 flex-wrap items-start">
            <span className="text-muted-foreground w-24 shrink-0 font-medium">
              {t("rows.nameServers")}
            </span>
            <div className="flex gap-1 flex-wrap">
              {data.name_servers.map((ns) => (
                <Badge key={ns} variant="secondary" className="font-mono text-xs">
                  {ns}
                </Badge>
              ))}
            </div>
          </div>
        )}
        {data.note && (
          <div className="flex gap-2">
            <span className="text-muted-foreground w-24 shrink-0 font-medium">
              {t("rows.note")}
            </span>
            <span className="text-muted-foreground break-all">{data.note}</span>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function IcpCard({ data }: { data: ICPInfo }) {
  const t = useTranslations("tools.icp.probe.result")
  const rows: [string, string | undefined][] = [
    [t("rows.domain"), data.domain],
    [t("rows.icpNumber"), data.icp_number],
    [t("rows.company"), data.company],
    [t("rows.type"), data.type],
    [t("rows.filedAt"), data.filed_at],
  ]
  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle>{t("title")}</CardTitle>
          {data.icp_number ? (
            <Badge variant="default">{data.icp_number}</Badge>
          ) : (
            <Badge variant="secondary">{t("noFiling")}</Badge>
          )}
        </div>
      </CardHeader>
      <CardContent className="space-y-3 text-sm">
        {rows.map(([k, v]) => (
          <Row key={k} label={k} value={v} />
        ))}
        {data.note && (
          <div className="flex gap-2">
            <span className="text-muted-foreground w-24 shrink-0 font-medium">
              {t("rows.note")}
            </span>
            <span className="text-muted-foreground break-all">{data.note}</span>
          </div>
        )}
        {/* 未命中本地库 → 给跳转工信部按钮(后端注入 inquiry_url)。 */}
        {data.inquiry_url && !data.icp_number && (
          <a
            href={data.inquiry_url}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex h-8 items-center rounded-md bg-primary px-3 text-xs font-medium text-primary-foreground hover:bg-primary/90"
          >
            前往工信部查询 →
          </a>
        )}
      </CardContent>
    </Card>
  )
}

function SslCard({ data }: { data: SSLInfo }) {
  const t = useTranslations("tools.ssl.probe.result")
  const days = data.days_until_expiry
  // valid=证书链验证 + SAN 匹配 OK;过期(days≤0)也是 invalid 的一种,但更直观。
  const isHealthy = data.valid && days > 0
  const rows: [string, string][] = [
    [t("rows.domain"), data.domain],
    [t("rows.issuer"), data.issuer],
    [t("rows.validFrom"), data.not_before],
    [t("rows.validTo"), data.not_after],
    [
      t("rows.certStatus"),
      isHealthy ? t("status.valid") : t("status.invalid"),
    ],
  ]
  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle>{t("title")}</CardTitle>
          <Badge variant={isHealthy && days > 30 ? "default" : "destructive"}>
            {days > 0
              ? t("expiresInDays", { days })
              : t("expired")}
          </Badge>
        </div>
      </CardHeader>
      <CardContent className="space-y-3 text-sm">
        {rows.map(([k, v]) => (
          <Row key={k} label={k} value={v} />
        ))}
        {/* 链验证失败原因(过期/自签/SAN 不匹配等)。让用户直接看到问题。 */}
        {!data.valid && data.verify_error && (
          <div className="flex gap-2">
            <span className="text-destructive w-24 shrink-0 font-medium">
              验证错误
            </span>
            <span className="text-destructive break-all">
              {data.verify_error}
            </span>
          </div>
        )}
        {data.san_domains && data.san_domains.length > 0 && (
          <div className="flex gap-2 flex-wrap items-start">
            <span className="text-muted-foreground w-24 shrink-0 font-medium">
              SAN
            </span>
            <div className="flex gap-1 flex-wrap">
              {data.san_domains.map((d) => (
                <Badge key={d} variant="secondary" className="font-mono text-xs">
                  {d}
                </Badge>
              ))}
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function DnsCard({ data }: { data: DNSInfo }) {
  const t = useTranslations("home.categories.domain.dns")
  const isMX = data.type === "MX"
  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle>{t("recordsTitle")}</CardTitle>
          <Badge variant="secondary" className="font-mono">
            {data.type}
          </Badge>
        </div>
      </CardHeader>
      <CardContent className="space-y-3 text-sm">
        <Row label={t("typeLabel")} value={`${data.domain} · ${data.type}`} />
        {data.records.length === 0 ? (
          <p className="text-muted-foreground text-sm">
            {t("noRecords", { type: data.type })}
          </p>
        ) : (
          <div className="space-y-1.5">
            <div className="flex gap-4 text-xs text-muted-foreground border-b pb-1.5">
              {isMX && <span className="w-12 text-right">优先级</span>}
              <span className="flex-1">{t("valueCol")}</span>
              <span className="w-20 text-right">{t("ttlCol")}</span>
            </div>
            {data.records.map((r, idx) => (
              <div key={idx} className="flex gap-4 items-center">
                {isMX && (
                  <span className="w-12 text-right font-mono text-muted-foreground text-xs">
                    {r.priority ?? "-"}
                  </span>
                )}
                <span className="flex-1 font-mono break-all">{r.value}</span>
                <span className="w-20 text-right font-mono text-muted-foreground text-xs">
                  {r.ttl ?? "-"}
                </span>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  )
}
