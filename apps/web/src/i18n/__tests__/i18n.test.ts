import { describe, it, expect } from 'vitest'
import { locales, defaultLocale, isValidLocale } from '../routing'

import cnTools from '../messages/cn/tools.json'
import enTools from '../messages/en/tools.json'
import cnCommon from '../messages/cn/common.json'
import enCommon from '../messages/en/common.json'
import cnErrors from '../messages/cn/errors.json'
import enErrors from '../messages/en/errors.json'
import cnLeaderboard from '../messages/cn/leaderboard.json'
import enLeaderboard from '../messages/en/leaderboard.json'

describe('routing config', () => {
  it('locales contains cn and en', () => {
    expect(locales).toContain('cn')
    expect(locales).toContain('en')
  })

  it('defaultLocale is cn', () => {
    expect(defaultLocale).toBe('cn')
  })

  it('isValidLocale returns true for cn and en', () => {
    expect(isValidLocale('cn')).toBe(true)
    expect(isValidLocale('en')).toBe(true)
  })

  it('isValidLocale returns false for unknown locales', () => {
    expect(isValidLocale('fr')).toBe(false)
    expect(isValidLocale('')).toBe(false)
    expect(isValidLocale('zh')).toBe(false)
  })
})

describe('tools namespace', () => {
  it('cn and en have the same top-level tool keys', () => {
    const cnKeys = Object.keys(cnTools).sort()
    const enKeys = Object.keys(enTools).sort()
    expect(cnKeys).toEqual(enKeys)
  })

  it('each core probe tool has title and description in cn', () => {
    const probeTools = ['ping', 'http', 'dns', 'ssl', 'traceroute', 'ip', 'whois', 'icp', 'diagnose']
    for (const slug of probeTools) {
      const tool = (cnTools as Record<string, { title?: string; description?: string }>)[slug]
      expect(tool?.title, `cn.tools.${slug}.title`).toBeTruthy()
      expect(tool?.description, `cn.tools.${slug}.description`).toBeTruthy()
    }
  })

  it('each core probe tool has title and description in en', () => {
    const probeTools = ['ping', 'http', 'dns', 'ssl', 'traceroute', 'ip', 'whois', 'icp', 'diagnose']
    for (const slug of probeTools) {
      const tool = (enTools as Record<string, { title?: string; description?: string }>)[slug]
      expect(tool?.title, `en.tools.${slug}.title`).toBeTruthy()
      expect(tool?.description, `en.tools.${slug}.description`).toBeTruthy()
    }
  })

  it('_ui.run is correct in cn', () => {
    expect((cnTools as unknown as Record<string, Record<string, string>>)._ui?.run).toBe('开始检测')
  })

  it('_ui.run is correct in en', () => {
    expect((enTools as unknown as Record<string, Record<string, string>>)._ui?.run).toBe('Run Check')
  })
})

describe('common namespace', () => {
  it('cn and en have the same keys', () => {
    expect(Object.keys(cnCommon).sort()).toEqual(Object.keys(enCommon).sort())
  })

  it('cn save is correct', () => {
    expect((cnCommon as Record<string, string>).save).toBe('保存')
  })

  it('en save is correct', () => {
    expect((enCommon as Record<string, string>).save).toBe('Save')
  })
})

describe('errors namespace', () => {
  const knownCodes = [
    'NOT_FOUND', 'DUPLICATE', 'CONFLICT', 'VALIDATION',
    'UNAUTHORIZED', 'FORBIDDEN', 'RATE_LIMIT', 'INTERNAL', 'UNAVAILABLE',
  ]

  it('cn and en have the same error codes', () => {
    expect(Object.keys(cnErrors).sort()).toEqual(Object.keys(enErrors).sort())
  })

  it('all known apperr codes are present in cn', () => {
    for (const code of knownCodes) {
      expect((cnErrors as Record<string, string>)[code], `cn.errors.${code}`).toBeTruthy()
    }
  })

  it('all known apperr codes are present in en', () => {
    for (const code of knownCodes) {
      expect((enErrors as Record<string, string>)[code], `en.errors.${code}`).toBeTruthy()
    }
  })
})

describe('leaderboard namespace', () => {
  it('has title and tabs in cn', () => {
    expect(cnLeaderboard.title).toBeTruthy()
    expect(cnLeaderboard.tabs.cdn).toBeTruthy()
  })

  it('has title and tabs in en', () => {
    expect(enLeaderboard.title).toBeTruthy()
    expect(enLeaderboard.tabs.cdn).toBeTruthy()
  })
})

describe('tools meta (en)', () => {
  // The legacy `en-tools-meta.ts` data table has been migrated into the
  // `tools.<slug>.meta` translation keys. These assertions guard that the
  // canonical English tool metadata still lives in tools.json (covers the SEO
  // `<head>` titles served by /en/tools/[slug]).
  const requiredMetaSlugs = [
    'ping', 'http', 'dns', 'traceroute', 'ssl', 'ip', 'whois', 'icp',
    'diagnose', 'ipv6-check', 'base64', 'cidr-calculator', 'cron-parser',
    'hash', 'ipv6-converter', 'json-formatter', 'jwt-decoder', 'qrcode',
    'regex-tester', 'tcping', 'timestamp',
  ]

  it('every required slug ships en meta.title + meta.description', () => {
    type Tool = { meta?: { title?: string; description?: string } }
    const tools = enTools as Record<string, Tool>
    for (const slug of requiredMetaSlugs) {
      const meta = tools[slug]?.meta
      expect(meta?.title, `en.tools.${slug}.meta.title`).toBeTruthy()
      expect(meta?.description, `en.tools.${slug}.meta.description`).toBeTruthy()
    }
  })

  it('every en meta.title carries the | idcd brand suffix', () => {
    type Tool = { meta?: { title?: string } }
    const tools = enTools as Record<string, Tool>
    for (const slug of requiredMetaSlugs) {
      const title = tools[slug]?.meta?.title
      expect(title, `en.tools.${slug}.meta.title`).toContain('| idcd')
    }
  })
})
