import { describe, it, expect } from 'vitest'
import { checkIPv6 } from '@/lib/tool-functions'

describe('checkIPv6', () => {
  it('有效 IPv6 地址返回 valid=true', () => {
    expect(checkIPv6('2001:db8::1').valid).toBe(true)
  })

  it('无效字符串返回 valid=false', () => {
    expect(checkIPv6('not-an-ipv6').valid).toBe(false)
    expect(checkIPv6('1.2.3.4').valid).toBe(false)
    expect(checkIPv6('').valid).toBe(false)
  })

  it('::1 类型为 Loopback', () => {
    const r = checkIPv6('::1')
    expect(r.valid).toBe(true)
    expect(r.type).toBe('Loopback')
  })

  it('fe80::1 类型为 Link-local', () => {
    const r = checkIPv6('fe80::1')
    expect(r.valid).toBe(true)
    expect(r.type).toBe('Link-local')
  })

  it('fc00::1 类型为 Unique Local', () => {
    const r = checkIPv6('fc00::1')
    expect(r.valid).toBe(true)
    expect(r.type).toBe('Unique Local')
  })

  it('ff02::1 类型为 Multicast', () => {
    const r = checkIPv6('ff02::1')
    expect(r.valid).toBe(true)
    expect(r.type).toBe('Multicast')
  })

  it(':: 类型为 Unspecified', () => {
    const r = checkIPv6('::')
    expect(r.valid).toBe(true)
    expect(r.type).toBe('Unspecified')
  })

  it('2001:db8::1 类型为 Documentation', () => {
    const r = checkIPv6('2001:db8::1')
    expect(r.valid).toBe(true)
    expect(r.type).toBe('Documentation')
  })

  it('::ffff:1.2.3.4 形式：isIPv4Mapped=true', () => {
    const r = checkIPv6('::ffff:102:304')
    expect(r.valid).toBe(true)
    expect(r.isIPv4Mapped).toBe(true)
    expect(r.type).toBe('IPv4-Mapped')
  })

  it('fe80::1 展开为完整格式', () => {
    const r = checkIPv6('fe80::1')
    expect(r.expanded).toBe('fe80:0000:0000:0000:0000:0000:0000:0001')
  })

  it('压缩格式正确', () => {
    const r = checkIPv6('2001:0db8:0000:0000:0000:0000:0000:0001')
    expect(r.compressed).toBe('2001:db8::1')
  })

  it('有多个 :: 返回 valid=false', () => {
    expect(checkIPv6('::1::1').valid).toBe(false)
  })
})
