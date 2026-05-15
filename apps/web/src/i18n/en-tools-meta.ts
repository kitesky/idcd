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
  {
    slug: 'base64',
    title: 'Base64 Encoder/Decoder — Encode & Decode Online | idcd',
    description: 'Instantly encode or decode Base64 strings and files in your browser. Supports URL-safe Base64 with no data ever leaving your device.',
    schemaName: 'Base64 Encoder/Decoder',
  },
  {
    slug: 'cidr-calculator',
    title: 'CIDR Calculator — Subnet & IP Range Tool | idcd',
    description: 'Calculate subnet masks, broadcast addresses, and host ranges from any CIDR notation. Supports IPv4 and IPv6 supernetting.',
    schemaName: 'CIDR Calculator',
  },
  {
    slug: 'cron-parser',
    title: 'Cron Expression Parser — Schedule Visualizer | idcd',
    description: 'Parse and visualize any cron expression with a human-readable schedule preview. See the next 10 run times at a glance.',
    schemaName: 'Cron Expression Parser',
  },
  {
    slug: 'hash',
    title: 'Hash Generator — MD5, SHA-1, SHA-256 & More | idcd',
    description: 'Generate MD5, SHA-1, SHA-256, and SHA-512 hashes from text or files instantly. All computation runs locally in your browser.',
    schemaName: 'Hash Generator',
  },
  {
    slug: 'ipv6-converter',
    title: 'IPv6 Converter — Format & Notation Converter | idcd',
    description: 'Convert IPv6 addresses between full, compressed, and mixed notations. Also converts between IPv6 and decimal or hex representations.',
    schemaName: 'IPv6 Converter',
  },
  {
    slug: 'json-formatter',
    title: 'JSON Formatter — Beautify & Validate JSON | idcd',
    description: 'Format, minify, and validate JSON with syntax highlighting and error detection. Supports large payloads with instant client-side processing.',
    schemaName: 'JSON Formatter',
  },
  {
    slug: 'jwt-decoder',
    title: 'JWT Decoder — Decode & Inspect JWTs | idcd',
    description: 'Decode JWT headers, payloads, and signatures without sending tokens to any server. Verify expiry and claims in one click.',
    schemaName: 'JWT Decoder',
  },
  {
    slug: 'qrcode',
    title: 'QR Code Generator — Create QR Codes Online | idcd',
    description: 'Generate QR codes for URLs, text, or contact info with customizable size and error correction. Download as PNG or SVG instantly.',
    schemaName: 'QR Code Generator',
  },
  {
    slug: 'regex-tester',
    title: 'Regex Tester — Live Regular Expression Editor | idcd',
    description: 'Test and debug regular expressions with real-time match highlighting and group capture display. Supports PCRE, JavaScript, and Python flavors.',
    schemaName: 'Regex Tester',
  },
  {
    slug: 'tcping',
    title: 'TCP Ping — Port Connectivity Tester | idcd',
    description: 'Test TCP port reachability and measure connection latency to any host from global probe nodes. Instantly detect blocked or closed ports.',
    schemaName: 'TCP Ping',
  },
  {
    slug: 'timestamp',
    title: 'Timestamp Converter — Unix Time Converter | idcd',
    description: 'Convert Unix timestamps to human-readable dates and back, across any timezone. Supports seconds, milliseconds, and ISO 8601 formats.',
    schemaName: 'Timestamp Converter',
  },
]

export function getEnToolMeta(slug: string): EnToolMeta | undefined {
  return EN_TOOLS_META.find(t => t.slug === slug)
}
