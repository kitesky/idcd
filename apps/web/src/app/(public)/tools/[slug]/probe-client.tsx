"use client"

import { useState } from 'react'
import { useSearchParams } from 'next/navigation'
import { Link2, Check } from 'lucide-react'
import { useTranslations } from 'next-intl'
import {
  Card, CardContent, CardHeader, CardTitle,
  Input, Button, Badge, Label,
} from '@/components/ui'

// Maps slug → translation sub-keys under `tools.<slug>.probe.*`.
// Keeping the slug list small means CI lint can detect a probe shipping
// without its companion translation block.
const PROBE_SLUGS = [
  'ssl', 'whois', 'icp', 'ip', 'tcp', 'mtr', 'smtp', 'rdns', 'asn',
  'mx', 'spf', 'dmarc', 'ntp', 'dkim', 'bgp',
] as const

type ProbeSlug = (typeof PROBE_SLUGS)[number]

function isProbeSlug(slug: string): slug is ProbeSlug {
  return (PROBE_SLUGS as readonly string[]).includes(slug)
}

interface ProbeClientProps {
  slug: string
}

export default function ProbeToolClient({ slug }: ProbeClientProps) {
  const t = useTranslations('tools')
  const tCommon = useTranslations('common')

  const searchParams = useSearchParams()
  const initialTarget = searchParams.get('target') ?? ''

  const [target, setTarget] = useState(initialTarget)
  const [extra, setExtra] = useState('')
  const [submitted, setSubmitted] = useState(false)
  const [loading, setLoading] = useState(false)
  const [copied, setCopied] = useState(false)

  // Resolve translated probe metadata via lookups — fall back to generic keys
  // if a particular slug doesn't ship its probe-specific labels.
  const title = String(t(`${slug}.title`) ?? slug)
  const description = String(t(`${slug}.description`) ?? '')
  const probeLabel = isProbeSlug(slug)
    ? String(t(`${slug}.probe.label`) ?? t('_probe.defaults.label'))
    : String(t('_probe.defaults.label'))
  const probePlaceholder = isProbeSlug(slug)
    ? String(t(`${slug}.probe.placeholder`) ?? '')
    : ''
  const probeExtraHint = isProbeSlug(slug)
    ? String(t(`${slug}.probe.extraHint`) ?? '')
    : ''
  const showExtraField = slug === 'tcp' || slug === 'dkim'
  const extraFieldLabel = showExtraField
    ? String(t(`${slug}.probe.extraFieldLabel`) ?? '')
    : ''
  const extraFieldPlaceholder = showExtraField
    ? String(t(`${slug}.probe.extraFieldPlaceholder`) ?? '')
    : ''

  const handleCopyLink = () => {
    const url =
      window.location.origin +
      window.location.pathname +
      '?target=' +
      encodeURIComponent(target)
    navigator.clipboard.writeText(url)
    setCopied(true)
    setTimeout(() => setCopied(false), 1500)
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!target.trim()) return
    setLoading(true)
    setTimeout(() => {
      setLoading(false)
      setSubmitted(true)
    }, 800)
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">{title}</h1>
        {description && (
          <p className="text-muted-foreground mt-2">{description}</p>
        )}
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{t('_probe.queryParams')}</CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-1">
              <Label htmlFor="target">{probeLabel}</Label>
              <Input
                id="target"
                value={target}
                onChange={e => setTarget(e.target.value)}
                placeholder={probePlaceholder}
                required
              />
              {probeExtraHint && (
                <p className="text-xs text-muted-foreground">{probeExtraHint}</p>
              )}
            </div>

            {showExtraField && (
              <div className="space-y-1">
                <Label>{extraFieldLabel}</Label>
                <Input
                  value={extra}
                  onChange={e => setExtra(e.target.value)}
                  placeholder={extraFieldPlaceholder}
                />
              </div>
            )}

            <div className="flex items-center gap-2">
              <Button type="submit" disabled={loading || !target.trim()}>
                {loading ? t('_probe.querying') : t('_probe.runQuery')}
              </Button>
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={handleCopyLink}
                disabled={!target.trim()}
                data-testid="copy-link-button"
              >
                {copied ? (
                  <>
                    <Check className="mr-1 h-4 w-4" />
                    {tCommon('copied')}
                  </>
                ) : (
                  <>
                    <Link2 className="mr-1 h-4 w-4" />
                    {t('_probe.copyLink')}
                  </>
                )}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>

      {submitted && (
        <Card>
          <CardHeader>
            <div className="flex items-center gap-2">
              <CardTitle>{t('_probe.results')}</CardTitle>
              <Badge variant="secondary">{t('_probe.sampleData')}</Badge>
            </div>
          </CardHeader>
          <CardContent className="space-y-4">
            <ProbeResultPlaceholder slug={slug} target={target} />
          </CardContent>
        </Card>
      )}

      <ProbeHelpCard slug={slug} />
    </div>
  )
}

function ProbeResultPlaceholder({ slug, target }: { slug: string; target: string }) {
  const t = useTranslations('tools')

  // Translated result placeholder rows. Each tool defines a list of
  // `{label, value}` pairs under `tools.<slug>.probe.placeholderRows`.
  // `{target}` interpolation gives us a per-tool sample without locale-specific
  // hard-coding. `t.raw` returns the underlying JSON value; absent in the
  // test mock — fall back gracefully below.
  type Row = { label: string; value: string }
  let rows: Row[] = []
  const tRaw = (t as unknown as { raw?: (key: string) => unknown }).raw
  if (isProbeSlug(slug) && typeof tRaw === 'function') {
    try {
      const raw = tRaw(`${slug}.probe.placeholderRows`)
      if (Array.isArray(raw)) {
        rows = raw.map((r) => {
          const row = r as { label?: string; value?: string }
          const value = (row.value ?? '').replace(/\{target\}/g, target)
          return { label: row.label ?? '', value }
        })
      }
    } catch {
      // missing translation block — fall back to generic rows below
    }
  }
  if (rows.length === 0) {
    rows = [
      { label: t('_probe.status'), value: t('_probe.fallbackStatus') },
      { label: t('_probe.targetLabel'), value: target },
      { label: t('_probe.note'), value: t('_probe.fallbackNote') },
    ]
  }

  return (
    <div className="space-y-2">
      {rows.map((row, i) => (
        <div key={i} className="flex gap-2 text-sm">
          <span className="text-muted-foreground w-32 shrink-0 font-medium">{row.label}</span>
          <span className="font-mono break-all">{row.value}</span>
        </div>
      ))}
    </div>
  )
}

function ProbeHelpCard({ slug }: { slug: string }) {
  const t = useTranslations('tools')

  let tips: string[] = []
  const tRaw = (t as unknown as { raw?: (key: string) => unknown }).raw
  if (isProbeSlug(slug) && typeof tRaw === 'function') {
    try {
      const raw = tRaw(`${slug}.probe.helpTips`)
      if (Array.isArray(raw)) {
        tips = raw.map((s) => String(s))
      }
    } catch {
      // ignore
    }
  }
  if (tips.length === 0) {
    tips = [t('_probe.fallbackTip1'), t('_probe.fallbackTip2')]
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('_probe.usage')}</CardTitle>
      </CardHeader>
      <CardContent className="text-sm text-muted-foreground space-y-1">
        {tips.map((tip, i) => (
          <p key={i}>• {tip}</p>
        ))}
      </CardContent>
    </Card>
  )
}
