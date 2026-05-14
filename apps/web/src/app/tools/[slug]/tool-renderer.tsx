"use client"

import { Card, CardContent } from '@/components/ui'
import { PROBE_TOOLS } from '@/app/tools/tools-config'
import ProbeToolClient from './probe-client'

// Text tools
import {
  WordCounterClient,
  LineSortClient,
  DuplicateRemoverClient,
  TextCaseClient,
  HtmlEncodeClient,
  EscapeHtmlClient,
  TextStatsClient,
  DiffClient,
  MarkdownClient,
} from './text-tools-client'

// Converter tools
import {
  UrlEncodeClient,
  UnicodeClient,
  JwtDecodeClient,
  NumberConvertClient,
  JsonToYamlClient,
  YamlFormatterClient,
  XmlFormatterClient,
  UrlParserClient,
  UserAgentClient,
  NumberFormatClient,
} from './converter-tools-client'

// Generator tools
import {
  PasswordGenClient,
  UuidGenClient,
  LoremClient,
  ChmodCalcClient,
  SortJsonClient,
  ColorPickerClient,
  ImageBase64Client,
} from './generator-tools-client'

// Lookup tools
import {
  RegexClient,
  CronVizClient,
  CidrCalcClient,
  Ipv6CheckClient,
  HttpStatusClient,
  MimeTypeClient,
  TimezoneClient,
  DateCalcClient,
  CsvFormatterClient,
} from './lookup-tools-client'

const PROBE_SLUGS = new Set(PROBE_TOOLS.map(t => t.slug))

const UTILITY_MAP: Record<string, React.FC> = {
  // Text tools
  'word-counter': WordCounterClient,
  'line-sort': LineSortClient,
  'duplicate-remover': DuplicateRemoverClient,
  'text-case': TextCaseClient,
  'html-encode': HtmlEncodeClient,
  'escape-html': EscapeHtmlClient,
  'text-stats': TextStatsClient,
  'diff': DiffClient,
  'markdown': MarkdownClient,

  // Converter tools
  'url-encode': UrlEncodeClient,
  'unicode': UnicodeClient,
  'jwt-decode': JwtDecodeClient,
  'number-convert': NumberConvertClient,
  'json-to-yaml': JsonToYamlClient,
  'yaml-formatter': YamlFormatterClient,
  'xml-formatter': XmlFormatterClient,
  'url-parser': UrlParserClient,
  'user-agent': UserAgentClient,
  'number-format': NumberFormatClient,

  // Generator tools
  'password-gen': PasswordGenClient,
  'uuid-gen': UuidGenClient,
  'lorem': LoremClient,
  'chmod-calc': ChmodCalcClient,
  'sort-json': SortJsonClient,
  'color-picker': ColorPickerClient,
  'image-base64': ImageBase64Client,

  // Lookup tools
  'regex': RegexClient,
  'cron-viz': CronVizClient,
  'cidr-calc': CidrCalcClient,
  'ipv6-check': Ipv6CheckClient,
  'http-status': HttpStatusClient,
  'mime-type': MimeTypeClient,
  'timezone': TimezoneClient,
  'date-calc': DateCalcClient,
  'csv-formatter': CsvFormatterClient,
}

interface ToolRendererProps {
  slug: string
}

export default function ToolRenderer({ slug }: ToolRendererProps) {
  if (PROBE_SLUGS.has(slug)) {
    return <ProbeToolClient slug={slug} />
  }

  const UtilityComponent = UTILITY_MAP[slug]
  if (UtilityComponent) {
    return <UtilityComponent />
  }

  return (
    <Card>
      <CardContent className="pt-6 text-center text-muted-foreground">
        <p className="text-lg">工具 "{slug}" 暂未实现</p>
        <p className="text-sm mt-2">请返回工具列表选择其他工具</p>
      </CardContent>
    </Card>
  )
}
