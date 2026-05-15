// ── URL 编解码 ───────────────────────────────────────────────────────────────

export function urlEncode(str: string): string {
  return encodeURIComponent(str)
}

export function urlDecode(str: string): string {
  return decodeURIComponent(str)
}

export function urlEncodeFull(str: string): string {
  return encodeURI(str)
}

// ── Unicode 字符转换 ─────────────────────────────────────────────────────────

export function charToCodePoint(char: string): string {
  if (!char) return ''
  const cp = char.codePointAt(0)
  if (cp === undefined) return ''
  return `U+${cp.toString(16).toUpperCase().padStart(4, '0')}`
}

export function codePointToChar(codePoint: string): string {
  const hex = codePoint.trim().replace(/^U\+/i, '').replace(/^0x/i, '')
  const cp = parseInt(hex, 16)
  if (isNaN(cp) || cp < 0 || cp > 0x10ffff) throw new Error('无效的码点')
  return String.fromCodePoint(cp)
}

export function stringToUnicode(str: string): string {
  return Array.from(str)
    .map(c => {
      const cp = c.codePointAt(0)!
      return `\\u{${cp.toString(16).toUpperCase()}}`
    })
    .join('')
}

export function unicodeToString(escaped: string): string {
  return escaped.replace(/\\u\{([0-9a-fA-F]+)\}/g, (_, hex) =>
    String.fromCodePoint(parseInt(hex, 16))
  )
}

// ── JWT 解码 ─────────────────────────────────────────────────────────────────

function decodeBase64Url(str: string): string {
  const base64 = str.replace(/-/g, '+').replace(/_/g, '/')
  const padded = base64 + '='.repeat((4 - (base64.length % 4)) % 4)
  return atob(padded)
}

export interface JWTDecoded {
  header: Record<string, unknown>
  payload: Record<string, unknown>
  signature: string
}

export function decodeJWT(token: string): JWTDecoded {
  const parts = token.trim().split('.')
  if (parts.length !== 3) throw new Error('无效的 JWT 格式，需要 3 个部分（Header.Payload.Signature）')

  const [headerB64, payloadB64, signature] = parts as [string, string, string]

  let header: Record<string, unknown>
  let payload: Record<string, unknown>

  try {
    header = JSON.parse(decodeBase64Url(headerB64))
  } catch {
    throw new Error('无法解码 JWT Header')
  }

  try {
    payload = JSON.parse(decodeBase64Url(payloadB64))
  } catch {
    throw new Error('无法解码 JWT Payload')
  }

  return { header, payload, signature }
}

// ── CIDR 计算 ────────────────────────────────────────────────────────────────

export interface CIDRInfo {
  network: string
  broadcast: string
  firstHost: string
  lastHost: string
  hosts: number
  totalAddresses: number
  mask: string
  cidr: number
  ipClass: string
}

export function parseCIDR(cidr: string): CIDRInfo {
  const parts = cidr.trim().split('/')
  if (parts.length !== 2) throw new Error('格式应为 IP/前缀，如 192.168.1.0/24')

  const [ip, prefixStr] = parts as [string, string]
  const prefix = parseInt(prefixStr, 10)

  if (isNaN(prefix) || prefix < 0 || prefix > 32) {
    throw new Error('前缀长度必须在 0-32 之间')
  }

  const octets = ip.split('.').map(Number)
  if (octets.length !== 4 || octets.some(o => isNaN(o) || o < 0 || o > 255)) {
    throw new Error('无效的 IPv4 地址')
  }

  const ipInt = octets.reduce((acc, val) => ((acc << 8) | val) >>> 0, 0)
  const maskInt = prefix === 0 ? 0 : ((0xffffffff << (32 - prefix)) >>> 0)
  const networkInt = (ipInt & maskInt) >>> 0
  const broadcastInt = (networkInt | (~maskInt >>> 0)) >>> 0

  const intToIP = (n: number) =>
    [(n >>> 24) & 0xff, (n >>> 16) & 0xff, (n >>> 8) & 0xff, n & 0xff].join('.')

  const totalAddresses = Math.pow(2, 32 - prefix)
  const hosts = prefix <= 30 ? totalAddresses - 2 : prefix === 31 ? 2 : 1

  const firstOctet = (networkInt >>> 24) & 0xff
  const ipClass =
    firstOctet < 128 ? 'A' :
    firstOctet < 192 ? 'B' :
    firstOctet < 224 ? 'C' :
    firstOctet < 240 ? 'D (组播)' : 'E (保留)'

  const maskParts = [
    (maskInt >>> 24) & 0xff,
    (maskInt >>> 16) & 0xff,
    (maskInt >>> 8) & 0xff,
    maskInt & 0xff,
  ]

  return {
    network: intToIP(networkInt),
    broadcast: intToIP(broadcastInt),
    firstHost: prefix < 31 ? intToIP(networkInt + 1) : intToIP(networkInt),
    lastHost: prefix < 31 ? intToIP(broadcastInt - 1) : intToIP(broadcastInt),
    hosts: Math.max(0, hosts),
    totalAddresses,
    mask: maskParts.join('.'),
    cidr: prefix,
    ipClass,
  }
}

// ── IPv6 处理 ────────────────────────────────────────────────────────────────

export type IPv6AddressType =
  | 'Loopback'
  | 'Unspecified'
  | 'Link-local'
  | 'Unique Local'
  | 'Multicast'
  | 'IPv4-Mapped'
  | 'Global Unicast'
  | 'Documentation'

export interface IPv6CheckResult {
  valid: boolean
  expanded: string
  compressed: string
  type: IPv6AddressType
  isIPv4Mapped: boolean
}

export function checkIPv6(input: string): IPv6CheckResult {
  const trimmed = input.trim()
  let expanded: string
  try {
    expanded = normalizeIPv6(trimmed)
  } catch {
    return { valid: false, expanded: '', compressed: '', type: 'Global Unicast', isIPv4Mapped: false }
  }
  const compressed = compressIPv6(trimmed)
  const isIPv4Mapped = /^0000:0000:0000:0000:0000:ffff:/i.test(expanded)
  let type: IPv6AddressType
  if (expanded === '0000:0000:0000:0000:0000:0000:0000:0000') {
    type = 'Unspecified'
  } else if (expanded === '0000:0000:0000:0000:0000:0000:0000:0001') {
    type = 'Loopback'
  } else if (isIPv4Mapped) {
    type = 'IPv4-Mapped'
  } else if (/^fe[89ab]/i.test(expanded.replace(/:/g, '').slice(0, 4))) {
    type = 'Link-local'
  } else if (/^f[cd]/i.test(expanded.slice(0, 2))) {
    type = 'Unique Local'
  } else if (/^ff/i.test(expanded.slice(0, 2))) {
    type = 'Multicast'
  } else if (expanded.startsWith('2001:0db8')) {
    type = 'Documentation'
  } else {
    type = 'Global Unicast'
  }
  return { valid: true, expanded, compressed, type, isIPv4Mapped }
}

export function ipv4ToIPv6Mapped(ipv4: string): string {
  const parts = ipv4.trim().split('.')
  if (parts.length !== 4) throw new Error('无效的 IPv4 地址')
  if (parts.some(p => isNaN(Number(p)) || Number(p) < 0 || Number(p) > 255)) {
    throw new Error('无效的 IPv4 地址')
  }
  return `::ffff:${ipv4.trim()}`
}

export function ipv6ToPTR(address: string): string {
  const expanded = normalizeIPv6(address)
  const nibbles = expanded.replace(/:/g, '').split('').reverse()
  return nibbles.join('.') + '.ip6.arpa'
}

export function normalizeIPv6(address: string): string {
  const zoneIdx = address.indexOf('%')
  const addr = zoneIdx >= 0 ? address.slice(0, zoneIdx) : address

  const colonPairs = addr.split('::')
  if (colonPairs.length > 2) throw new Error('无效的 IPv6：不能有多个 ::')

  let groups: string[]

  if (colonPairs.length === 2) {
    const left = colonPairs[0] ? colonPairs[0].split(':') : []
    const right = colonPairs[1] ? colonPairs[1].split(':') : []
    const missing = 8 - left.length - right.length
    if (missing < 0) throw new Error('IPv6 组数太多')
    const middle = Array(missing).fill('0')
    groups = [...left, ...middle, ...right]
  } else {
    groups = addr.split(':')
  }

  if (groups.length !== 8) throw new Error('IPv6 必须有 8 组')
  if (groups.some(g => !/^[0-9a-fA-F]{0,4}$/.test(g))) throw new Error('无效的 IPv6 字符')

  return groups.map(g => g.padStart(4, '0')).join(':')
}

export function compressIPv6(address: string): string {
  const full = normalizeIPv6(address)
  const withoutLeadingZeros = full
    .split(':')
    .map(g => parseInt(g, 16).toString(16))
    .join(':')

  // Find longest run of :0: groups
  let best = { start: -1, len: 0 }
  let cur = { start: -1, len: 0 }

  const segs = withoutLeadingZeros.split(':')
  for (let i = 0; i < segs.length; i++) {
    if (segs[i] === '0') {
      if (cur.start === -1) cur = { start: i, len: 1 }
      else cur.len++
      if (cur.len > best.len) best = { ...cur }
    } else {
      cur = { start: -1, len: 0 }
    }
  }

  if (best.len < 2) return withoutLeadingZeros

  const left = segs.slice(0, best.start).join(':')
  const right = segs.slice(best.start + best.len).join(':')
  return (left ? left + '::' : '::') + (right || '')
}

export function getIPv6Type(address: string): string {
  try {
    const full = normalizeIPv6(address)
    if (full === '0000:0000:0000:0000:0000:0000:0000:0001') return '回环地址 (::1)'
    if (full.startsWith('fe80')) return '链路本地地址 (fe80::/10)'
    if (full.startsWith('fc') || full.startsWith('fd')) return '唯一本地地址 (fc00::/7)'
    if (full.startsWith('2001:0db8')) return '文档示例地址 (2001:db8::/32)'
    if (full.startsWith('2002')) return '6to4 地址 (2002::/16)'
    if (full.startsWith('ff')) return '多播地址 (ff00::/8)'
    if (/^0000:0000:0000:0000:0000:ffff/.test(full)) return 'IPv4 映射地址'
    return '全局单播地址'
  } catch {
    return '未知'
  }
}

// ── 颜色转换 ─────────────────────────────────────────────────────────────────

export interface RGBColor {
  r: number
  g: number
  b: number
}

export interface HSLColor {
  h: number
  s: number
  l: number
}

export function hexToRgb(hex: string): RGBColor | null {
  const clean = hex.replace('#', '').trim()
  const expanded = clean.length === 3
    ? clean.split('').map(c => c + c).join('')
    : clean

  if (!/^[0-9a-fA-F]{6}$/.test(expanded)) return null

  return {
    r: parseInt(expanded.slice(0, 2), 16),
    g: parseInt(expanded.slice(2, 4), 16),
    b: parseInt(expanded.slice(4, 6), 16),
  }
}

export function rgbToHex(r: number, g: number, b: number): string {
  const clamp = (n: number) => Math.max(0, Math.min(255, Math.round(n)))
  return '#' + [clamp(r), clamp(g), clamp(b)]
    .map(n => n.toString(16).padStart(2, '0'))
    .join('')
}

export function rgbToHsl(r: number, g: number, b: number): HSLColor {
  const rn = r / 255
  const gn = g / 255
  const bn = b / 255

  const max = Math.max(rn, gn, bn)
  const min = Math.min(rn, gn, bn)
  const l = (max + min) / 2

  if (max === min) return { h: 0, s: 0, l: Math.round(l * 100) }

  const d = max - min
  const s = l > 0.5 ? d / (2 - max - min) : d / (max + min)

  let h: number
  switch (max) {
    case rn: h = ((gn - bn) / d + (gn < bn ? 6 : 0)) / 6; break
    case gn: h = ((bn - rn) / d + 2) / 6; break
    default: h = ((rn - gn) / d + 4) / 6
  }

  return {
    h: Math.round(h * 360),
    s: Math.round(s * 100),
    l: Math.round(l * 100),
  }
}

export function hslToRgb(h: number, s: number, l: number): RGBColor {
  const sn = s / 100
  const ln = l / 100

  const c = (1 - Math.abs(2 * ln - 1)) * sn
  const x = c * (1 - Math.abs(((h / 60) % 2) - 1))
  const m = ln - c / 2

  let rp = 0, gp = 0, bp = 0
  if (h < 60) { rp = c; gp = x }
  else if (h < 120) { rp = x; gp = c }
  else if (h < 180) { gp = c; bp = x }
  else if (h < 240) { gp = x; bp = c }
  else if (h < 300) { rp = x; bp = c }
  else { rp = c; bp = x }

  return {
    r: Math.round((rp + m) * 255),
    g: Math.round((gp + m) * 255),
    b: Math.round((bp + m) * 255),
  }
}

// ── HTML 编解码 ──────────────────────────────────────────────────────────────

export function encodeHTML(str: string): string {
  return str
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#039;')
}

export function decodeHTML(str: string): string {
  return str
    .replace(/&amp;/g, '&')
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
    .replace(/&quot;/g, '"')
    .replace(/&#039;/g, "'")
    .replace(/&#(\d+);/g, (_, code) => String.fromCharCode(parseInt(code, 10)))
    .replace(/&#x([0-9a-fA-F]+);/g, (_, hex) => String.fromCharCode(parseInt(hex, 16)))
}

// ── 进制转换 ─────────────────────────────────────────────────────────────────

export function convertBase(num: string, fromBase: number, toBase: number): string {
  const trimmed = num.trim()
  if (!trimmed) throw new Error('输入不能为空')
  const decimal = parseInt(trimmed, fromBase)
  if (isNaN(decimal)) throw new Error(`"${trimmed}" 不是有效的 ${fromBase} 进制数`)
  return decimal.toString(toBase).toUpperCase()
}

export function numberToAllBases(num: string, fromBase: number): Record<string, string> {
  const decimal = parseInt(num.trim(), fromBase)
  if (isNaN(decimal)) throw new Error('无效数字')
  return {
    二进制: decimal.toString(2),
    八进制: decimal.toString(8),
    十进制: decimal.toString(10),
    十六进制: decimal.toString(16).toUpperCase(),
  }
}

// ── 文本大小写转换 ────────────────────────────────────────────────────────────

export function toCamelCase(str: string): string {
  return str
    .replace(/[_\-\s]+(.)/g, (_, c: string) => c.toUpperCase())
    .replace(/^(.)/, (c: string) => c.toLowerCase())
}

export function toSnakeCase(str: string): string {
  return str
    .replace(/([A-Z])/g, '_$1')
    .replace(/[\s\-]+/g, '_')
    .toLowerCase()
    .replace(/^_/, '')
}

export function toPascalCase(str: string): string {
  return str
    .replace(/[_\-\s]+(.)/g, (_, c: string) => c.toUpperCase())
    .replace(/^(.)/, (c: string) => c.toUpperCase())
}

export function toKebabCase(str: string): string {
  return str
    .replace(/([A-Z])/g, '-$1')
    .replace(/[\s_]+/g, '-')
    .toLowerCase()
    .replace(/^-/, '')
}

export function toConstantCase(str: string): string {
  return toSnakeCase(str).toUpperCase()
}

// ── 日期计算 ─────────────────────────────────────────────────────────────────

export function dateDiff(date1: string, date2: string): number {
  const d1 = new Date(date1)
  const d2 = new Date(date2)
  if (isNaN(d1.getTime()) || isNaN(d2.getTime())) throw new Error('无效的日期格式')
  return Math.round((d2.getTime() - d1.getTime()) / 86400000)
}

export function addDays(date: string, days: number): string {
  const d = new Date(date)
  if (isNaN(d.getTime())) throw new Error('无效的日期格式')
  d.setDate(d.getDate() + days)
  return d.toISOString().split('T')[0]!
}

// ── 数字格式化 ────────────────────────────────────────────────────────────────

export function formatNumber(
  num: number,
  locale = 'zh-CN',
  options?: Intl.NumberFormatOptions
): string {
  return new Intl.NumberFormat(locale, options).format(num)
}

export function formatCurrency(num: number, currency = 'CNY', locale = 'zh-CN'): string {
  return new Intl.NumberFormat(locale, { style: 'currency', currency }).format(num)
}

// ── 字数统计 ─────────────────────────────────────────────────────────────────

export interface TextStats {
  chars: number
  charsNoSpace: number
  words: number
  lines: number
  paragraphs: number
  sentences: number
  chineseChars: number
}

export function countWords(text: string): TextStats {
  const chars = text.length
  const charsNoSpace = text.replace(/\s/g, '').length
  const words = text.trim() === '' ? 0 : text.trim().split(/\s+/).length
  const lines = text === '' ? 0 : text.split('\n').length
  const paragraphs = text.trim() === '' ? 0 : text.trim().split(/\n\s*\n/).length
  const sentences = text.trim() === '' ? 0 : (text.match(/[.!?。！？]/g) ?? []).length
  const chineseChars = (text.match(/[一-鿿]/g) ?? []).length

  return { chars, charsNoSpace, words, lines, paragraphs, sentences, chineseChars }
}

// ── 行排序 ───────────────────────────────────────────────────────────────────

export function sortLines(
  text: string,
  reverse = false,
  caseInsensitive = false
): string {
  if (!text) return ''
  const lines = text.split('\n')
  const sorted = [...lines].sort((a, b) => {
    const x = caseInsensitive ? a.toLowerCase() : a
    const y = caseInsensitive ? b.toLowerCase() : b
    return x.localeCompare(y, 'zh-CN')
  })
  return (reverse ? sorted.reverse() : sorted).join('\n')
}

// ── 去重 ─────────────────────────────────────────────────────────────────────

export function removeDuplicates(text: string, caseInsensitive = false): string {
  if (!text) return ''
  const lines = text.split('\n')
  const seen = new Set<string>()
  const result: string[] = []

  for (const line of lines) {
    const key = caseInsensitive ? line.toLowerCase() : line
    if (!seen.has(key)) {
      seen.add(key)
      result.push(line)
    }
  }

  return result.join('\n')
}

// ── chmod 计算 ────────────────────────────────────────────────────────────────

export interface PermGroup {
  r: boolean
  w: boolean
  x: boolean
}

export interface ChmodPerms {
  owner: PermGroup
  group: PermGroup
  others: PermGroup
}

export function calculateChmod(perms: ChmodPerms): string {
  const toOctal = (p: PermGroup) =>
    ((p.r ? 4 : 0) + (p.w ? 2 : 0) + (p.x ? 1 : 0)).toString()

  return toOctal(perms.owner) + toOctal(perms.group) + toOctal(perms.others)
}

export function parseChmod(octal: string): ChmodPerms {
  const digits = octal.replace(/^0/, '').padStart(3, '0').split('').map(Number)
  if (digits.some(d => isNaN(d) || d > 7)) throw new Error('无效的八进制权限')

  const toBool = (n: number): PermGroup => ({
    r: !!(n & 4),
    w: !!(n & 2),
    x: !!(n & 1),
  })

  return {
    owner: toBool(digits[0]!),
    group: toBool(digits[1]!),
    others: toBool(digits[2]!),
  }
}

export function chmodToSymbolic(octal: string): string {
  const perms = parseChmod(octal)
  const sym = (g: PermGroup) =>
    (g.r ? 'r' : '-') + (g.w ? 'w' : '-') + (g.x ? 'x' : '-')
  return sym(perms.owner) + sym(perms.group) + sym(perms.others)
}

// ── JSON 键排序 ───────────────────────────────────────────────────────────────

export function sortJSON(obj: unknown): unknown {
  if (obj === null || typeof obj !== 'object') return obj
  if (Array.isArray(obj)) return obj.map(sortJSON)

  return Object.keys(obj as Record<string, unknown>)
    .sort()
    .reduce((acc: Record<string, unknown>, key) => {
      acc[key] = sortJSON((obj as Record<string, unknown>)[key])
      return acc
    }, {})
}

// ── JSON → YAML ───────────────────────────────────────────────────────────────

export function jsonToYaml(obj: unknown, indent = 0): string {
  if (obj === null) return 'null'
  if (typeof obj === 'boolean') return obj.toString()
  if (typeof obj === 'number') return obj.toString()
  if (typeof obj === 'string') {
    const needsQuote = /[:#\[\]{},&*?|<>=!%@`'"\\]/.test(obj) ||
      obj.includes('\n') ||
      obj.trim() !== obj ||
      obj === '' ||
      /^(true|false|null|yes|no|on|off|\d.*)$/i.test(obj)
    if (needsQuote) return `"${obj.replace(/\\/g, '\\\\').replace(/"/g, '\\"').replace(/\n/g, '\\n')}"`
    return obj
  }

  const spaces = '  '.repeat(indent)

  if (Array.isArray(obj)) {
    if (obj.length === 0) return '[]'
    return obj.map(item => {
      const v = jsonToYaml(item, indent + 1)
      if (typeof item === 'object' && item !== null && !Array.isArray(item)) {
        return `${spaces}-\n${v}`
      }
      return `${spaces}- ${v}`
    }).join('\n')
  }

  if (typeof obj === 'object') {
    const entries = Object.entries(obj as Record<string, unknown>)
    if (entries.length === 0) return '{}'
    return entries.map(([k, v]) => {
      if (v === null || typeof v !== 'object') {
        return `${spaces}${k}: ${jsonToYaml(v, indent + 1)}`
      }
      const inner = jsonToYaml(v, indent + 1)
      if (Array.isArray(v) && (v as unknown[]).length === 0) {
        return `${spaces}${k}: []`
      }
      if (!Array.isArray(v) && Object.keys(v as object).length === 0) {
        return `${spaces}${k}: {}`
      }
      return `${spaces}${k}:\n${inner}`
    }).join('\n')
  }

  return String(obj)
}

// ── XML 格式化 ────────────────────────────────────────────────────────────────

export function formatXML(xml: string): string {
  const compact = xml.trim().replace(/>\s*</g, '><')
  const tokens = compact.split(/(<[^>]+>)/g)

  let result = ''
  let depth = 0

  for (const token of tokens) {
    if (!token.trim()) continue

    if (token.startsWith('</')) {
      // Closing tag
      depth = Math.max(0, depth - 1)
      result += '  '.repeat(depth) + token + '\n'
    } else if (token.startsWith('<') && !token.startsWith('<?') && !token.startsWith('<!') && !token.endsWith('/>')) {
      // Opening tag
      result += '  '.repeat(depth) + token + '\n'
      depth++
    } else if (token.startsWith('<')) {
      // Self-closing or declaration
      result += '  '.repeat(depth) + token + '\n'
    } else {
      // Text content
      const text = token.trim()
      if (text) result += '  '.repeat(depth) + text + '\n'
    }
  }

  return result.trim()
}

// ── YAML 格式化（基础缩进规范化）──────────────────────────────────────────────

export function formatYAML(yaml: string): string {
  const lines = yaml.split('\n')
  const result: string[] = []

  for (const line of lines) {
    if (!line.trim()) {
      result.push('')
      continue
    }
    const originalIndent = line.length - line.trimStart().length
    const level = Math.round(originalIndent / 2)
    result.push('  '.repeat(level) + line.trimStart())
  }

  // Remove consecutive blank lines
  return result
    .join('\n')
    .replace(/\n{3,}/g, '\n\n')
    .trim()
}

// ── URL 解析 ─────────────────────────────────────────────────────────────────

export function parseURL(url: string): Record<string, string> {
  const u = new URL(url)
  const params: Record<string, string> = {}
  u.searchParams.forEach((v, k) => {
    params[k] = v
  })

  return {
    href: u.href,
    protocol: u.protocol,
    username: u.username,
    password: u.password,
    hostname: u.hostname,
    port: u.port,
    pathname: u.pathname,
    search: u.search,
    hash: u.hash,
    origin: u.origin,
    ...Object.fromEntries(
      Object.entries(params).map(([k, v]) => [`param:${k}`, v])
    ),
  }
}

// ── User-Agent 解析 ───────────────────────────────────────────────────────────

export function parseUserAgent(ua: string): Record<string, string> {
  const result: Record<string, string> = {}

  // Browser
  if (ua.includes('Edg/') || ua.includes('Edge/')) {
    const m = ua.match(/Edg(?:e)?\/([\d.]+)/)
    result['浏览器'] = `Microsoft Edge ${m?.[1] ?? ''}`
  } else if (ua.includes('OPR/') || ua.includes('Opera/')) {
    const m = ua.match(/OPR\/([\d.]+)/)
    result['浏览器'] = `Opera ${m?.[1] ?? ''}`
  } else if (ua.includes('Chrome/') && !ua.includes('Chromium')) {
    const m = ua.match(/Chrome\/([\d.]+)/)
    result['浏览器'] = `Google Chrome ${m?.[1] ?? ''}`
  } else if (ua.includes('Firefox/')) {
    const m = ua.match(/Firefox\/([\d.]+)/)
    result['浏览器'] = `Mozilla Firefox ${m?.[1] ?? ''}`
  } else if (ua.includes('Safari/') && !ua.includes('Chrome')) {
    const m = ua.match(/Version\/([\d.]+)/)
    result['浏览器'] = `Safari ${m?.[1] ?? ''}`
  } else {
    result['浏览器'] = '未知'
  }

  // OS
  if (ua.includes('Windows NT')) {
    const m = ua.match(/Windows NT ([\d.]+)/)
    const v = m?.[1]
    const map: Record<string, string> = { '10.0': '10/11', '6.3': '8.1', '6.2': '8', '6.1': '7', '6.0': 'Vista' }
    result['操作系统'] = `Windows ${map[v ?? ''] ?? v ?? ''}`
  } else if (ua.includes('Mac OS X')) {
    const m = ua.match(/Mac OS X ([0-9_]+)/)
    result['操作系统'] = `macOS ${(m?.[1] ?? '').replace(/_/g, '.')}`
  } else if (ua.includes('Android')) {
    const m = ua.match(/Android ([\d.]+)/)
    result['操作系统'] = `Android ${m?.[1] ?? ''}`
  } else if (ua.includes('iPhone') || ua.includes('iPad') || ua.includes('iPod')) {
    const m = ua.match(/OS ([\d_]+)/)
    result['操作系统'] = `iOS ${(m?.[1] ?? '').replace(/_/g, '.')}`
  } else if (ua.includes('Linux')) {
    result['操作系统'] = 'Linux'
  } else {
    result['操作系统'] = '未知'
  }

  // Device
  if (ua.includes('iPhone')) result['设备'] = 'iPhone'
  else if (ua.includes('iPad')) result['设备'] = 'iPad'
  else if (ua.includes('Android') && ua.includes('Mobile')) result['设备'] = '安卓手机'
  else if (ua.includes('Android')) result['设备'] = '安卓平板'
  else result['设备'] = '桌面端'

  // Engine
  if (ua.includes('Gecko/')) result['引擎'] = 'Gecko'
  else if (ua.includes('WebKit/')) result['引擎'] = 'WebKit'
  else if (ua.includes('Blink')) result['引擎'] = 'Blink'
  else result['引擎'] = '未知'

  return result
}

// ── 文本统计（词频）───────────────────────────────────────────────────────────

export interface TextFreqStats {
  totalWords: number
  uniqueWords: number
  topWords: Array<[string, number]>
}

export function getTextFreqStats(text: string): TextFreqStats {
  const words = text.toLowerCase().match(/[一-龥]+|[a-z0-9]+/g) ?? []
  const freq: Record<string, number> = {}

  for (const w of words) {
    freq[w] = (freq[w] ?? 0) + 1
  }

  const sorted = Object.entries(freq).sort(([, a], [, b]) => b - a)

  return {
    totalWords: words.length,
    uniqueWords: sorted.length,
    topWords: sorted.slice(0, 30),
  }
}

// ── Lorem Ipsum ───────────────────────────────────────────────────────────────

const LOREM_SENTENCES = [
  'Lorem ipsum dolor sit amet, consectetur adipiscing elit.',
  'Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.',
  'Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi.',
  'Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore.',
  'Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt.',
  'Pellentesque habitant morbi tristique senectus et netus et malesuada fames.',
  'Curabitur pretium tincidunt lacus, nulla commodo enim vestibulum feugiat.',
  'Vivamus euismod mauris vitae dolor efficitur, vitae malesuada justo gravida.',
]

export function generateLorem(paragraphs: number, sentencesPerParagraph = 4): string {
  const result: string[] = []
  for (let i = 0; i < paragraphs; i++) {
    const sentences: string[] = []
    for (let j = 0; j < sentencesPerParagraph; j++) {
      sentences.push(LOREM_SENTENCES[(i * sentencesPerParagraph + j) % LOREM_SENTENCES.length]!)
    }
    result.push(sentences.join(' '))
  }
  return result.join('\n\n')
}

// ── CSV 解析 ─────────────────────────────────────────────────────────────────

export function parseCSV(csv: string, delimiter = ','): string[][] {
  const rows = csv.trim().split('\n')
  return rows.map(row => {
    const cells: string[] = []
    let inQuote = false
    let current = ''

    for (let i = 0; i < row.length; i++) {
      const ch = row[i]
      if (ch === '"') {
        if (inQuote && row[i + 1] === '"') {
          current += '"'
          i++
        } else {
          inQuote = !inQuote
        }
      } else if (ch === delimiter && !inQuote) {
        cells.push(current)
        current = ''
      } else {
        current += ch
      }
    }

    cells.push(current)
    return cells
  })
}

// ── 密码生成 ─────────────────────────────────────────────────────────────────

export interface PasswordOptions {
  uppercase: boolean
  lowercase: boolean
  numbers: boolean
  symbols: boolean
}

export function generatePassword(length: number, options: PasswordOptions): string {
  let charset = ''
  if (options.uppercase) charset += 'ABCDEFGHIJKLMNOPQRSTUVWXYZ'
  if (options.lowercase) charset += 'abcdefghijklmnopqrstuvwxyz'
  if (options.numbers) charset += '0123456789'
  if (options.symbols) charset += '!@#$%^&*()_+-=[]{}|;:,.?'

  if (!charset) charset = 'abcdefghijklmnopqrstuvwxyz0123456789'

  const array = new Uint32Array(length)
  crypto.getRandomValues(array)

  return Array.from(array)
    .map(v => charset[v % charset.length])
    .join('')
}

// ── UUID 生成 ─────────────────────────────────────────────────────────────────

export function generateUUID(): string {
  return crypto.randomUUID()
}

// ── Markdown 解析（基础）──────────────────────────────────────────────────────

export function parseMarkdown(text: string): string {
  // Escape HTML first
  let html = text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')

  // Code blocks
  html = html.replace(/```([\s\S]*?)```/g, '<pre><code>$1</code></pre>')
  // Inline code
  html = html.replace(/`([^`]+)`/g, '<code>$1</code>')
  // Headers
  html = html.replace(/^### (.+)$/gm, '<h3>$1</h3>')
  html = html.replace(/^## (.+)$/gm, '<h2>$1</h2>')
  html = html.replace(/^# (.+)$/gm, '<h1>$1</h1>')
  // Bold
  html = html.replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
  html = html.replace(/__(.+?)__/g, '<strong>$1</strong>')
  // Italic
  html = html.replace(/\*(.+?)\*/g, '<em>$1</em>')
  html = html.replace(/_(.+?)_/g, '<em>$1</em>')
  // Blockquote
  html = html.replace(/^&gt; (.+)$/gm, '<blockquote>$1</blockquote>')
  // Links — only allow safe protocols to prevent javascript: injection
  html = html.replace(/\[([^\]]+)\]\(([^)]+)\)/g, (_full, linkText, href) => {
    if (/^(https?:|mailto:)/.test(href)) {
      return `<a href="${href}" target="_blank" rel="noopener noreferrer">${linkText}</a>`
    }
    return linkText
  })
  // Horizontal rule
  html = html.replace(/^---+$/gm, '<hr />')
  // Unordered list
  html = html.replace(/^[*\-] (.+)$/gm, '<li>$1</li>')
  // Ordered list
  html = html.replace(/^\d+\. (.+)$/gm, '<li>$1</li>')
  // Paragraphs
  html = html.replace(/\n\n+/g, '</p><p>')
  html = '<p>' + html + '</p>'
  // Clean up around block elements
  html = html.replace(/<p>(<h[1-6]|<pre|<ul|<ol|<li|<blockquote|<hr)/g, '$1')
  html = html.replace(/(<\/h[1-6]>|<\/pre>|<\/li>|<\/blockquote>|<hr \/>)<\/p>/g, '$1')
  html = html.replace(/<p><\/p>/g, '')

  return html
}

// ── 时区工具 ─────────────────────────────────────────────────────────────────

export const TIMEZONES = [
  { zone: 'UTC', label: 'UTC' },
  { zone: 'America/New_York', label: '美东 (EST/EDT)' },
  { zone: 'America/Chicago', label: '美中 (CST/CDT)' },
  { zone: 'America/Denver', label: '美山 (MST/MDT)' },
  { zone: 'America/Los_Angeles', label: '美西 (PST/PDT)' },
  { zone: 'America/Sao_Paulo', label: '圣保罗 (BRT)' },
  { zone: 'Europe/London', label: '伦敦 (GMT/BST)' },
  { zone: 'Europe/Paris', label: '巴黎 (CET/CEST)' },
  { zone: 'Europe/Moscow', label: '莫斯科 (MSK)' },
  { zone: 'Asia/Dubai', label: '迪拜 (GST)' },
  { zone: 'Asia/Kolkata', label: '孟买 (IST)' },
  { zone: 'Asia/Shanghai', label: '上海 (CST)' },
  { zone: 'Asia/Tokyo', label: '东京 (JST)' },
  { zone: 'Asia/Singapore', label: '新加坡 (SGT)' },
  { zone: 'Australia/Sydney', label: '悉尼 (AEST/AEDT)' },
] as const

export function getTimeInZone(date: Date, zone: string): string {
  return date.toLocaleString('zh-CN', {
    timeZone: zone,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}

// ── HTTP 状态码 ───────────────────────────────────────────────────────────────

export interface HttpStatusCode {
  code: number
  reason: string
  description: string
  category: string
}

export const HTTP_STATUS_CODES: HttpStatusCode[] = [
  // 1xx
  { code: 100, reason: 'Continue', description: '继续发送请求体', category: '1xx 信息' },
  { code: 101, reason: 'Switching Protocols', description: '服务器同意切换协议（如 WebSocket）', category: '1xx 信息' },
  { code: 102, reason: 'Processing', description: '请求已接收，处理中', category: '1xx 信息' },
  // 2xx
  { code: 200, reason: 'OK', description: '请求成功', category: '2xx 成功' },
  { code: 201, reason: 'Created', description: '资源创建成功', category: '2xx 成功' },
  { code: 204, reason: 'No Content', description: '请求成功，无响应体', category: '2xx 成功' },
  { code: 206, reason: 'Partial Content', description: '范围请求成功（断点续传）', category: '2xx 成功' },
  // 3xx
  { code: 301, reason: 'Moved Permanently', description: '永久重定向', category: '3xx 重定向' },
  { code: 302, reason: 'Found', description: '临时重定向', category: '3xx 重定向' },
  { code: 304, reason: 'Not Modified', description: '资源未修改，使用缓存', category: '3xx 重定向' },
  { code: 307, reason: 'Temporary Redirect', description: '临时重定向，保持方法', category: '3xx 重定向' },
  { code: 308, reason: 'Permanent Redirect', description: '永久重定向，保持方法', category: '3xx 重定向' },
  // 4xx
  { code: 400, reason: 'Bad Request', description: '请求语法错误', category: '4xx 客户端错误' },
  { code: 401, reason: 'Unauthorized', description: '需要身份认证', category: '4xx 客户端错误' },
  { code: 403, reason: 'Forbidden', description: '无权限访问', category: '4xx 客户端错误' },
  { code: 404, reason: 'Not Found', description: '资源不存在', category: '4xx 客户端错误' },
  { code: 405, reason: 'Method Not Allowed', description: '不允许的 HTTP 方法', category: '4xx 客户端错误' },
  { code: 408, reason: 'Request Timeout', description: '请求超时', category: '4xx 客户端错误' },
  { code: 409, reason: 'Conflict', description: '资源冲突', category: '4xx 客户端错误' },
  { code: 410, reason: 'Gone', description: '资源已永久删除', category: '4xx 客户端错误' },
  { code: 413, reason: 'Payload Too Large', description: '请求体过大', category: '4xx 客户端错误' },
  { code: 422, reason: 'Unprocessable Entity', description: '请求格式正确但语义错误', category: '4xx 客户端错误' },
  { code: 429, reason: 'Too Many Requests', description: '请求频率超限', category: '4xx 客户端错误' },
  // 5xx
  { code: 500, reason: 'Internal Server Error', description: '服务器内部错误', category: '5xx 服务端错误' },
  { code: 502, reason: 'Bad Gateway', description: '网关收到无效响应', category: '5xx 服务端错误' },
  { code: 503, reason: 'Service Unavailable', description: '服务暂时不可用', category: '5xx 服务端错误' },
  { code: 504, reason: 'Gateway Timeout', description: '网关超时', category: '5xx 服务端错误' },
  { code: 507, reason: 'Insufficient Storage', description: '服务器存储空间不足', category: '5xx 服务端错误' },
]

// ── MIME 类型 ─────────────────────────────────────────────────────────────────

export interface MimeEntry {
  ext: string
  mime: string
  description: string
}

export const MIME_TYPES: MimeEntry[] = [
  { ext: '.html', mime: 'text/html', description: 'HTML 文档' },
  { ext: '.css', mime: 'text/css', description: 'CSS 样式表' },
  { ext: '.js', mime: 'application/javascript', description: 'JavaScript' },
  { ext: '.ts', mime: 'application/typescript', description: 'TypeScript' },
  { ext: '.json', mime: 'application/json', description: 'JSON 数据' },
  { ext: '.xml', mime: 'application/xml', description: 'XML 文档' },
  { ext: '.pdf', mime: 'application/pdf', description: 'PDF 文档' },
  { ext: '.zip', mime: 'application/zip', description: 'ZIP 压缩包' },
  { ext: '.gz', mime: 'application/gzip', description: 'Gzip 压缩' },
  { ext: '.tar', mime: 'application/x-tar', description: 'TAR 归档' },
  { ext: '.png', mime: 'image/png', description: 'PNG 图片' },
  { ext: '.jpg', mime: 'image/jpeg', description: 'JPEG 图片' },
  { ext: '.gif', mime: 'image/gif', description: 'GIF 图片' },
  { ext: '.svg', mime: 'image/svg+xml', description: 'SVG 矢量图' },
  { ext: '.webp', mime: 'image/webp', description: 'WebP 图片' },
  { ext: '.ico', mime: 'image/x-icon', description: '图标文件' },
  { ext: '.mp4', mime: 'video/mp4', description: 'MP4 视频' },
  { ext: '.webm', mime: 'video/webm', description: 'WebM 视频' },
  { ext: '.mp3', mime: 'audio/mpeg', description: 'MP3 音频' },
  { ext: '.wav', mime: 'audio/wav', description: 'WAV 音频' },
  { ext: '.ogg', mime: 'audio/ogg', description: 'OGG 音频' },
  { ext: '.woff', mime: 'font/woff', description: 'Web 字体' },
  { ext: '.woff2', mime: 'font/woff2', description: 'Web 字体 v2' },
  { ext: '.ttf', mime: 'font/ttf', description: 'TrueType 字体' },
  { ext: '.txt', mime: 'text/plain', description: '纯文本' },
  { ext: '.csv', mime: 'text/csv', description: 'CSV 数据' },
  { ext: '.yaml', mime: 'application/yaml', description: 'YAML 配置' },
  { ext: '.wasm', mime: 'application/wasm', description: 'WebAssembly' },
  { ext: '.bin', mime: 'application/octet-stream', description: '二进制数据' },
  { ext: '.exe', mime: 'application/x-msdownload', description: 'Windows 可执行文件' },
]
