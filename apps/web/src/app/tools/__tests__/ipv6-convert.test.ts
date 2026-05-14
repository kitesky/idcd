import { describe, it, expect } from 'vitest'
import { normalizeIPv6, compressIPv6, ipv4ToIPv6Mapped, ipv6ToPTR } from '@/lib/tool-functions'

describe('normalizeIPv6（展开）', () => {
  it('展开 ::1', () => {
    expect(normalizeIPv6('::1')).toBe('0000:0000:0000:0000:0000:0000:0000:0001')
  })

  it('展开 fe80::1', () => {
    expect(normalizeIPv6('fe80::1')).toBe('fe80:0000:0000:0000:0000:0000:0000:0001')
  })

  it('完整格式不变（仅补前导零）', () => {
    expect(normalizeIPv6('2001:db8:0:0:0:0:0:1')).toBe('2001:0db8:0000:0000:0000:0000:0000:0001')
  })

  it('无效格式抛出错误', () => {
    expect(() => normalizeIPv6('::1::1')).toThrow()
    expect(() => normalizeIPv6('not-ipv6')).toThrow()
  })
})

describe('compressIPv6（压缩）', () => {
  it('压缩完整 IPv6 → 最短形式', () => {
    expect(compressIPv6('2001:0db8:0000:0000:0000:0000:0000:0001')).toBe('2001:db8::1')
  })

  it('压缩 ::1 不变', () => {
    expect(compressIPv6('::1')).toBe('::1')
  })

  it('压缩全零 → ::', () => {
    expect(compressIPv6('::')).toBe('::')
  })

  it('压缩 fe80::1 不变', () => {
    expect(compressIPv6('fe80::1')).toBe('fe80::1')
  })
})

describe('ipv4ToIPv6Mapped', () => {
  it('1.2.3.4 → ::ffff:1.2.3.4', () => {
    expect(ipv4ToIPv6Mapped('1.2.3.4')).toBe('::ffff:1.2.3.4')
  })

  it('192.168.1.1 → ::ffff:192.168.1.1', () => {
    expect(ipv4ToIPv6Mapped('192.168.1.1')).toBe('::ffff:192.168.1.1')
  })

  it('无效 IPv4 抛出错误', () => {
    expect(() => ipv4ToIPv6Mapped('999.0.0.1')).toThrow()
    expect(() => ipv4ToIPv6Mapped('not-an-ip')).toThrow()
  })
})

describe('ipv6ToPTR', () => {
  it('::1 → PTR 记录', () => {
    expect(ipv6ToPTR('::1')).toBe(
      '1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.ip6.arpa'
    )
  })

  it('2001:db8::1 → PTR 记录', () => {
    const ptr = ipv6ToPTR('2001:db8::1')
    expect(ptr).toMatch(/\.ip6\.arpa$/)
    expect(ptr.split('.').length).toBe(34)
  })

  it('PTR 记录包含 32 十六进制 nibbles', () => {
    const ptr = ipv6ToPTR('fe80::1')
    const nibbles = ptr.replace('.ip6.arpa', '').split('.')
    expect(nibbles).toHaveLength(32)
    nibbles.forEach(n => expect(n).toMatch(/^[0-9a-f]$/))
  })
})
