import { describe, it, expect } from 'vitest'
import { locales, defaultLocale, isValidLocale } from '../routing'
import { EN_TOOLS_META, getEnToolMeta } from '../en-tools-meta'

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

describe('EN_TOOLS_META', () => {
  it('contains 21 tools', () => {
    expect(EN_TOOLS_META).toHaveLength(21)
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
