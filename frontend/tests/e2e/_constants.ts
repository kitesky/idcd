export const E2E_USER = {
  email: 'e2e-test@idcd.local',
  password: 'E2eTest!2026',
}

// Filesystem-derived tool slugs (from apps/web/src/app/(public)/tools/).
// Excludes 'diagnose' (covered by dedicated spec) and dynamic '[slug]'.
export const TOOL_SLUGS = [
  'asn', 'base64', 'bgp', 'cidr-calculator', 'cron-parser', 'dkim', 'dmarc',
  'dns', 'hash', 'http', 'icp', 'ip', 'ipv6-converter', 'json-formatter',
  'jwt-decoder', 'mtr', 'mx', 'ntp', 'ping', 'qrcode', 'rdns', 'regex-tester',
  'smtp', 'speedtest', 'spf', 'ssl', 'tcp', 'tcping', 'timestamp', 'traceroute',
  'whois',
] as const

export const APP_PAGES = [
  '/app/dashboard',
  '/app/monitors',
  '/app/alerts',
  '/app/incidents',
  '/app/oncall',
  '/app/cert',
  '/app/nodes',
  '/app/status-pages',
  '/app/billing',
  '/app/usage',
  '/app/reports',
  '/app/referral',
  '/app/verdict',
  '/app/settings',
] as const

export const PUBLIC_PAGES = [
  '/',
  '/docs',
  '/pricing',
  '/tools',
  '/about',
  '/privacy',
  '/terms',
  '/contact',
  '/changelog',
] as const
