/**
 * Static tool registry — slug + category only. Display names and descriptions
 * live in `messages/{locale}/tools.json` under `tools.<slug>.title` /
 * `tools.<slug>.description`. Resolve them via `useTranslations('tools')` in
 * client components or `getT('tools')` server-side.
 *
 * Adding a new tool: append a `{ slug, category }` entry here, then add the
 * matching `<slug>` key in `messages/cn/tools.json` and `messages/en/tools.json`.
 */
export interface ToolDef {
  slug: string
  category: 'probe' | 'utility'
}

export const ALL_TOOLS: ToolDef[] = [
  // ── 拨测工具（16 个）────────────────────────────────────────────────
  { slug: 'ssl', category: 'probe' },
  { slug: 'whois', category: 'probe' },
  { slug: 'icp', category: 'probe' },
  { slug: 'ip', category: 'probe' },
  { slug: 'tcp', category: 'probe' },
  { slug: 'mtr', category: 'probe' },
  { slug: 'smtp', category: 'probe' },
  { slug: 'rdns', category: 'probe' },
  { slug: 'asn', category: 'probe' },
  { slug: 'mx', category: 'probe' },
  { slug: 'spf', category: 'probe' },
  { slug: 'dmarc', category: 'probe' },
  { slug: 'ntp', category: 'probe' },
  { slug: 'dkim', category: 'probe' },
  { slug: 'bgp', category: 'probe' },
  { slug: 'speedtest', category: 'probe' },

  // ── 辅助工具（35 个）────────────────────────────────────────────────
  { slug: 'yaml-formatter', category: 'utility' },
  { slug: 'xml-formatter', category: 'utility' },
  { slug: 'url-encode', category: 'utility' },
  { slug: 'unicode', category: 'utility' },
  { slug: 'jwt-decoder', category: 'utility' },
  { slug: 'regex-tester', category: 'utility' },
  { slug: 'cron-parser', category: 'utility' },
  { slug: 'cidr-calculator', category: 'utility' },
  { slug: 'ipv6-check', category: 'utility' },
  { slug: 'color-picker', category: 'utility' },
  { slug: 'markdown', category: 'utility' },
  { slug: 'diff', category: 'utility' },
  { slug: 'word-counter', category: 'utility' },
  { slug: 'password-gen', category: 'utility' },
  { slug: 'uuid-gen', category: 'utility' },
  { slug: 'number-convert', category: 'utility' },
  { slug: 'text-case', category: 'utility' },
  { slug: 'html-encode', category: 'utility' },
  { slug: 'json-to-yaml', category: 'utility' },
  { slug: 'url-parser', category: 'utility' },
  { slug: 'user-agent', category: 'utility' },
  { slug: 'http-status', category: 'utility' },
  { slug: 'mime-type', category: 'utility' },
  { slug: 'timezone', category: 'utility' },
  { slug: 'date-calc', category: 'utility' },
  { slug: 'image-base64', category: 'utility' },
  { slug: 'csv-formatter', category: 'utility' },
  { slug: 'number-format', category: 'utility' },
  { slug: 'lorem', category: 'utility' },
  { slug: 'line-sort', category: 'utility' },
  { slug: 'duplicate-remover', category: 'utility' },
  { slug: 'text-stats', category: 'utility' },
  { slug: 'escape-html', category: 'utility' },
  { slug: 'chmod-calc', category: 'utility' },
  { slug: 'sort-json', category: 'utility' },
]

export const PROBE_TOOLS = ALL_TOOLS.filter(t => t.category === 'probe')
export const UTILITY_TOOLS = ALL_TOOLS.filter(t => t.category === 'utility')

export function getToolBySlug(slug: string): ToolDef | undefined {
  return ALL_TOOLS.find(t => t.slug === slug)
}
