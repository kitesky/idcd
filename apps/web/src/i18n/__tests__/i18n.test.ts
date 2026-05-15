import { describe, it, expect } from 'vitest'
import { locales, defaultLocale } from '../config'
import { getMessages } from '../messages'
import zhMessages from '../messages/zh.json'
import enMessages from '../messages/en.json'
import { EN_TOOLS_META, getEnToolMeta } from '../en-tools-meta'

describe('i18n config', () => {
  it('locales contains zh and en', () => {
    expect(locales).toContain('zh')
    expect(locales).toContain('en')
  })

  it('defaultLocale is zh', () => {
    expect(defaultLocale).toBe('zh')
  })
})

describe('getMessages', () => {
  it('returns zh messages for zh locale', () => {
    const messages = getMessages('zh')
    expect(messages).toBe(zhMessages)
  })

  it('returns en messages for en locale', () => {
    const messages = getMessages('en')
    expect(messages).toBe(enMessages)
  })
})

describe('zh messages structure', () => {
  it('has tools section', () => {
    expect(zhMessages.tools).toBeDefined()
  })

  it('has common section', () => {
    expect(zhMessages.common).toBeDefined()
  })

  it('has leaderboard section', () => {
    expect(zhMessages.leaderboard).toBeDefined()
  })

  it('ping tool has title and description', () => {
    expect(zhMessages.tools.ping.title).toBeTruthy()
    expect(zhMessages.tools.ping.description).toBeTruthy()
  })

  it('common run key is defined', () => {
    expect(zhMessages.common.run).toBe('开始检测')
  })
})

describe('en messages structure', () => {
  it('has tools section', () => {
    expect(enMessages.tools).toBeDefined()
  })

  it('has common section', () => {
    expect(enMessages.common).toBeDefined()
  })

  it('has leaderboard section', () => {
    expect(enMessages.leaderboard).toBeDefined()
  })

  it('ping tool has title and description', () => {
    expect(enMessages.tools.ping.title).toBeTruthy()
    expect(enMessages.tools.ping.description).toBeTruthy()
  })

  it('common run key is English', () => {
    expect(enMessages.common.run).toBe('Run Check')
  })

  it('leaderboard tabs are in English', () => {
    expect(enMessages.leaderboard.tabs.cdn).toBe('CDN Response Speed')
  })
})

describe('zh and en message keys are in sync', () => {
  const zhToolKeys = Object.keys(zhMessages.tools)
  const enToolKeys = Object.keys(enMessages.tools)

  it('both locales have the same tool keys', () => {
    expect(zhToolKeys.sort()).toEqual(enToolKeys.sort())
  })

  it('both locales have the same common keys', () => {
    const zhCommonKeys = Object.keys(zhMessages.common).sort()
    const enCommonKeys = Object.keys(enMessages.common).sort()
    expect(zhCommonKeys).toEqual(enCommonKeys)
  })
})

describe('EN_TOOLS_META', () => {
  it('contains 21 tools', () => {
    expect(EN_TOOLS_META).toHaveLength(21)
  })

  it('all 21 slugs are present', () => {
    const slugs = EN_TOOLS_META.map(t => t.slug)
    const expectedSlugs = [
      'ping',
      'http',
      'dns',
      'traceroute',
      'ssl',
      'ip',
      'whois',
      'icp',
      'diagnose',
      'ipv6-check',
      'base64',
      'cidr-calculator',
      'cron-parser',
      'hash',
      'ipv6-converter',
      'json-formatter',
      'jwt-decoder',
      'qrcode',
      'regex-tester',
      'tcping',
      'timestamp',
    ]
    for (const slug of expectedSlugs) {
      expect(slugs).toContain(slug)
    }
  })

  it('each tool has title, description, and schemaName', () => {
    for (const tool of EN_TOOLS_META) {
      expect(tool.title).toBeTruthy()
      expect(tool.description).toBeTruthy()
      expect(tool.schemaName).toBeTruthy()
    }
  })

  it('getEnToolMeta returns correct entry for ping', () => {
    const tool = getEnToolMeta('ping')
    expect(tool).toBeDefined()
    expect(tool?.slug).toBe('ping')
  })

  it('getEnToolMeta returns undefined for unknown slug', () => {
    expect(getEnToolMeta('nonexistent-tool')).toBeUndefined()
  })

  it('each title includes | idcd suffix', () => {
    for (const tool of EN_TOOLS_META) {
      expect(tool.title).toContain('| idcd')
    }
  })
})
