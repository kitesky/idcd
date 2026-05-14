export interface EnToolMeta {
  slug: string
  title: string
  description: string
  schemaName: string
}

export const EN_TOOLS_META: EnToolMeta[] = [
  {
    slug: 'ping',
    title: 'Ping Check — Global ICMP Latency Test | idcd',
    description: 'Test ICMP ping latency and packet loss from 100+ real probe nodes worldwide. Identify network issues in seconds.',
    schemaName: 'Ping Check',
  },
  {
    slug: 'http',
    title: 'HTTP Check — Multi-Region Uptime Test | idcd',
    description: 'Verify HTTP/HTTPS availability and response times from multiple global regions. Catch downtime before your users do.',
    schemaName: 'HTTP Check',
  },
  {
    slug: 'dns',
    title: 'DNS Lookup — Global DNS Resolution Test | idcd',
    description: 'Query DNS records from resolvers worldwide and detect DNS pollution or hijacking. Supports A, AAAA, MX, TXT, CNAME.',
    schemaName: 'DNS Lookup',
  },
  {
    slug: 'traceroute',
    title: 'Traceroute — Global Network Path Tracing | idcd',
    description: 'Trace the route your packets take to any destination from global nodes. Pinpoint latency bottlenecks hop by hop.',
    schemaName: 'Traceroute',
  },
  {
    slug: 'ssl',
    title: 'SSL Certificate Check — TLS Validity & Config | idcd',
    description: "Verify your domain's SSL/TLS certificate expiry, issuer chain, and cipher configuration. Get alerted before certs expire.",
    schemaName: 'SSL Certificate Check',
  },
  {
    slug: 'ip',
    title: 'IP Geolocation — IP Address Lookup | idcd',
    description: 'Look up any IP address for geolocation, ASN, ISP, and organization data. Supports IPv4 and IPv6.',
    schemaName: 'IP Geolocation Lookup',
  },
  {
    slug: 'whois',
    title: 'WHOIS Lookup — Domain Registration Info | idcd',
    description: "Query domain WHOIS records for registrar, registrant, creation date, expiry date, and name servers.",
    schemaName: 'WHOIS Lookup',
  },
  {
    slug: 'icp',
    title: 'ICP Filing Check — China ICP Beian Lookup | idcd',
    description: "Check whether a domain has a valid China ICP (Internet Content Provider) filing (ICP 备案). Required for hosting in mainland China.",
    schemaName: 'ICP Filing Check',
  },
  {
    slug: 'diagnose',
    title: 'Network Diagnostics — One-Click Network Check | idcd',
    description: 'Run a comprehensive network diagnostic in one click. Automatically detects DNS failures, latency spikes, routing issues, and more.',
    schemaName: 'Network Diagnostics',
  },
  {
    slug: 'ipv6-check',
    title: 'IPv6 Check — Address Validation & Conversion | idcd',
    description: 'Validate IPv6 addresses, convert between expanded and compressed formats, and detect address type (global, link-local, loopback).',
    schemaName: 'IPv6 Check',
  },
]

export function getEnToolMeta(slug: string): EnToolMeta | undefined {
  return EN_TOOLS_META.find(t => t.slug === slug)
}
