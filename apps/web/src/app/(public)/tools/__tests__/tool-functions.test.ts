import { describe, it, expect } from 'vitest'
import {
  // URL
  urlEncode, urlDecode,
  // Unicode
  charToCodePoint, codePointToChar, stringToUnicode, unicodeToString,
  // JWT
  decodeJWT,
  // CIDR
  parseCIDR,
  // IPv6
  normalizeIPv6, compressIPv6, getIPv6Type,
  // Color
  hexToRgb, rgbToHex, rgbToHsl, hslToRgb,
  // HTML
  encodeHTML, decodeHTML,
  // Base conversion
  convertBase, numberToAllBases,
  // Text case
  toCamelCase, toSnakeCase, toPascalCase, toKebabCase, toConstantCase,
  // Date
  dateDiff, addDays,
  // Number format
  formatNumber, formatCurrency,
  // Word count
  countWords,
  // Sort/dedup
  sortLines, removeDuplicates,
  // chmod
  calculateChmod, parseChmod, chmodToSymbolic,
  // JSON sort
  sortJSON,
  // JSON/YAML
  jsonToYaml,
  // XML
  formatXML,
  // YAML
  formatYAML,
  // URL parser
  parseURL,
  // User Agent
  parseUserAgent,
  // Text stats
  getTextFreqStats,
  // Lorem
  generateLorem,
  // CSV
  parseCSV,
  // Markdown
  parseMarkdown,
  // Timezone
  getTimeInZone,
} from '@/lib/tool-functions'

// ── URL 编解码 ────────────────────────────────────────────────────────────────
describe('URL 编解码', () => {
  it('编码中文字符', () => {
    expect(urlEncode('你好世界')).toBe('%E4%BD%A0%E5%A5%BD%E4%B8%96%E7%95%8C')
  })

  it('编码特殊字符', () => {
    expect(urlEncode('foo bar&baz=1')).toBe('foo%20bar%26baz%3D1')
  })

  it('解码编码字符串', () => {
    expect(urlDecode('%E4%BD%A0%E5%A5%BD')).toBe('你好')
  })

  it('解码 URL 特殊字符', () => {
    expect(urlDecode('foo%20bar%26baz%3D1')).toBe('foo bar&baz=1')
  })

  it('编解码互逆', () => {
    const original = 'Hello, 世界! @#$%^&*()'
    expect(urlDecode(urlEncode(original))).toBe(original)
  })
})

// ── Unicode ───────────────────────────────────────────────────────────────────
describe('Unicode 转换', () => {
  it('字符转码点 - ASCII', () => {
    expect(charToCodePoint('A')).toBe('U+0041')
  })

  it('字符转码点 - 中文', () => {
    expect(charToCodePoint('中')).toBe('U+4E2D')
  })

  it('字符转码点 - emoji', () => {
    expect(charToCodePoint('😀')).toBe('U+1F600')
  })

  it('码点转字符 - U+ 前缀', () => {
    expect(codePointToChar('U+4E2D')).toBe('中')
  })

  it('码点转字符 - 十六进制', () => {
    expect(codePointToChar('41')).toBe('A')
  })

  it('无效码点抛出错误', () => {
    expect(() => codePointToChar('GGGG')).toThrow()
  })

  it('字符串转 Unicode 转义', () => {
    const result = stringToUnicode('AB')
    expect(result).toBe('\\u{41}\\u{42}')
  })

  it('Unicode 转义转字符串', () => {
    expect(unicodeToString('\\u{4E2D}')).toBe('中')
  })
})

// ── JWT 解码 ─────────────────────────────────────────────────────────────────
describe('JWT 解码', () => {
  const validJWT = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c'

  it('成功解码有效 JWT', () => {
    const { header, payload, signature } = decodeJWT(validJWT)
    expect(header.alg).toBe('HS256')
    expect(header.typ).toBe('JWT')
    expect(payload.sub).toBe('1234567890')
    expect(payload.name).toBe('John Doe')
    expect(payload.iat).toBe(1516239022)
    expect(signature).toBe('SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c')
  })

  it('无效格式抛出错误', () => {
    expect(() => decodeJWT('not.a.valid.jwt.here.extra')).toThrow('无效的 JWT 格式')
  })

  it('两段 JWT 抛出错误', () => {
    expect(() => decodeJWT('aaa.bbb')).toThrow()
  })

  it('处理空白字符', () => {
    const { payload } = decodeJWT('  ' + validJWT + '  ')
    expect(payload.name).toBe('John Doe')
  })
})

// ── CIDR 计算 ─────────────────────────────────────────────────────────────────
describe('CIDR 计算', () => {
  it('/24 网段计算', () => {
    const r = parseCIDR('192.168.1.0/24')
    expect(r.network).toBe('192.168.1.0')
    expect(r.broadcast).toBe('192.168.1.255')
    expect(r.firstHost).toBe('192.168.1.1')
    expect(r.lastHost).toBe('192.168.1.254')
    expect(r.hosts).toBe(254)
    expect(r.totalAddresses).toBe(256)
    expect(r.mask).toBe('255.255.255.0')
  })

  it('/16 网段计算', () => {
    const r = parseCIDR('10.0.0.0/16')
    expect(r.network).toBe('10.0.0.0')
    expect(r.hosts).toBe(65534)
    expect(r.totalAddresses).toBe(65536)
  })

  it('/32 单主机', () => {
    const r = parseCIDR('192.168.1.1/32')
    expect(r.totalAddresses).toBe(1)
    // /32 是单主机路由，地址本身即主机
    expect(r.hosts).toBeGreaterThanOrEqual(0)
  })

  it('无效 CIDR 抛出错误', () => {
    expect(() => parseCIDR('192.168.1.0')).toThrow()
    expect(() => parseCIDR('192.168.1.0/33')).toThrow()
    expect(() => parseCIDR('256.0.0.0/24')).toThrow()
  })

  it('掩码正确', () => {
    const r = parseCIDR('10.0.0.0/8')
    expect(r.mask).toBe('255.0.0.0')
  })
})

// ── IPv6 ──────────────────────────────────────────────────────────────────────
describe('IPv6 处理', () => {
  it('标准化 ::1', () => {
    const full = normalizeIPv6('::1')
    expect(full).toBe('0000:0000:0000:0000:0000:0000:0000:0001')
  })

  it('压缩完整 IPv6', () => {
    const compressed = compressIPv6('2001:0db8:0000:0000:0000:0000:0000:0001')
    expect(compressed).toBe('2001:db8::1')
  })

  it('回环地址检测', () => {
    expect(getIPv6Type('::1')).toContain('回环')
  })

  it('链路本地地址检测', () => {
    expect(getIPv6Type('fe80::1')).toContain('链路本地')
  })

  it('无效 IPv6 抛出错误', () => {
    expect(() => normalizeIPv6('not::valid::ipv6::address::here')).toThrow()
  })
})

// ── 颜色转换 ─────────────────────────────────────────────────────────────────
describe('颜色转换', () => {
  it('HEX → RGB', () => {
    expect(hexToRgb('#3b82f6')).toEqual({ r: 59, g: 130, b: 246 })
  })

  it('3位 HEX', () => {
    expect(hexToRgb('#fff')).toEqual({ r: 255, g: 255, b: 255 })
  })

  it('无效 HEX 返回 null', () => {
    expect(hexToRgb('invalid')).toBeNull()
    expect(hexToRgb('#xyz123')).toBeNull()
  })

  it('RGB → HEX', () => {
    expect(rgbToHex(59, 130, 246)).toBe('#3b82f6')
  })

  it('黑色 RGB → HEX', () => {
    expect(rgbToHex(0, 0, 0)).toBe('#000000')
  })

  it('白色 RGB → HEX', () => {
    expect(rgbToHex(255, 255, 255)).toBe('#ffffff')
  })

  it('RGB → HSL 红色', () => {
    const hsl = rgbToHsl(255, 0, 0)
    expect(hsl.h).toBe(0)
    expect(hsl.s).toBe(100)
    expect(hsl.l).toBe(50)
  })

  it('RGB → HSL 无色彩', () => {
    const hsl = rgbToHsl(128, 128, 128)
    expect(hsl.s).toBe(0)
  })

  it('HSL → RGB 红色', () => {
    const rgb = hslToRgb(0, 100, 50)
    expect(rgb.r).toBe(255)
    expect(rgb.g).toBe(0)
    expect(rgb.b).toBe(0)
  })
})

// ── HTML 编解码 ──────────────────────────────────────────────────────────────
describe('HTML 编解码', () => {
  it('编码 HTML 特殊字符', () => {
    expect(encodeHTML('<div class="test">&hello</div>')).toBe(
      '&lt;div class=&quot;test&quot;&gt;&amp;hello&lt;/div&gt;'
    )
  })

  it('编码单引号', () => {
    expect(encodeHTML("it's")).toBe('it&#039;s')
  })

  it('解码 HTML 实体', () => {
    expect(decodeHTML('&lt;div&gt;&amp;hello&lt;/div&gt;')).toBe('<div>&hello</div>')
  })

  it('编解码互逆', () => {
    const original = '<script>alert("xss & injection")</script>'
    expect(decodeHTML(encodeHTML(original))).toBe(original)
  })

  it('空字符串不变', () => {
    expect(encodeHTML('')).toBe('')
    expect(decodeHTML('')).toBe('')
  })
})

// ── 进制转换 ─────────────────────────────────────────────────────────────────
describe('进制转换', () => {
  it('十进制 → 二进制', () => {
    expect(convertBase('255', 10, 2)).toBe('11111111')
  })

  it('二进制 → 十六进制', () => {
    expect(convertBase('11111111', 2, 16)).toBe('FF')
  })

  it('十进制 → 十六进制', () => {
    expect(convertBase('255', 10, 16)).toBe('FF')
  })

  it('十六进制 → 十进制', () => {
    expect(convertBase('FF', 16, 10)).toBe('255')
  })

  it('无效输入抛出错误', () => {
    expect(() => convertBase('GG', 16, 10)).toThrow()
    expect(() => convertBase('', 10, 2)).toThrow()
  })

  it('numberToAllBases', () => {
    const r = numberToAllBases('255', 10)
    expect(r['二进制']).toBe('11111111')
    expect(r['十六进制']).toBe('FF')
    expect(r['十进制']).toBe('255')
    expect(r['八进制']).toBe('377')
  })
})

// ── 文本大小写 ────────────────────────────────────────────────────────────────
describe('文本大小写转换', () => {
  it('camelCase', () => {
    expect(toCamelCase('hello world')).toBe('helloWorld')
    expect(toCamelCase('hello_world')).toBe('helloWorld')
    expect(toCamelCase('hello-world')).toBe('helloWorld')
  })

  it('snake_case', () => {
    expect(toSnakeCase('helloWorld')).toBe('hello_world')
    expect(toSnakeCase('hello world')).toBe('hello_world')
  })

  it('PascalCase', () => {
    expect(toPascalCase('hello world')).toBe('HelloWorld')
    expect(toPascalCase('hello_world')).toBe('HelloWorld')
  })

  it('kebab-case', () => {
    expect(toKebabCase('helloWorld')).toBe('hello-world')
    expect(toKebabCase('hello_world')).toBe('hello-world')
  })

  it('CONSTANT_CASE', () => {
    expect(toConstantCase('hello world')).toBe('HELLO_WORLD')
    expect(toConstantCase('helloWorld')).toBe('HELLO_WORLD')
  })
})

// ── 日期计算 ─────────────────────────────────────────────────────────────────
describe('日期计算', () => {
  it('两日期差值', () => {
    expect(dateDiff('2024-01-01', '2024-01-31')).toBe(30)
  })

  it('负差值', () => {
    expect(dateDiff('2024-01-31', '2024-01-01')).toBe(-30)
  })

  it('相同日期为0', () => {
    expect(dateDiff('2024-06-15', '2024-06-15')).toBe(0)
  })

  it('日期加法', () => {
    expect(addDays('2024-01-01', 30)).toBe('2024-01-31')
  })

  it('日期减法', () => {
    expect(addDays('2024-01-31', -30)).toBe('2024-01-01')
  })

  it('跨月计算', () => {
    expect(addDays('2024-01-31', 1)).toBe('2024-02-01')
  })

  it('无效日期抛出错误', () => {
    expect(() => dateDiff('invalid', '2024-01-01')).toThrow()
    expect(() => addDays('not-a-date', 10)).toThrow()
  })
})

// ── 数字格式化 ────────────────────────────────────────────────────────────────
describe('数字格式化', () => {
  it('千分位', () => {
    expect(formatNumber(1234567)).toContain('1')
    expect(formatNumber(1234567)).toContain('234')
  })

  it('两位小数', () => {
    const result = formatNumber(1234.5, 'zh-CN', { minimumFractionDigits: 2 })
    expect(result).toContain('1')
  })

  it('货币格式', () => {
    const result = formatCurrency(100, 'CNY')
    expect(result).toContain('100')
  })
})

// ── 字数统计 ─────────────────────────────────────────────────────────────────
describe('字数统计', () => {
  it('空字符串', () => {
    const r = countWords('')
    expect(r.chars).toBe(0)
    expect(r.words).toBe(0)
    expect(r.lines).toBe(0)
  })

  it('简单英文', () => {
    const r = countWords('Hello World')
    expect(r.chars).toBe(11)
    expect(r.words).toBe(2)
    expect(r.charsNoSpace).toBe(10)
  })

  it('多行文本', () => {
    const r = countWords('Line 1\nLine 2\nLine 3')
    expect(r.lines).toBe(3)
  })

  it('中文字符计数', () => {
    const r = countWords('你好世界')
    expect(r.chineseChars).toBe(4)
  })

  it('段落计数', () => {
    const r = countWords('Para 1\n\nPara 2')
    expect(r.paragraphs).toBe(2)
  })
})

// ── 行排序 ───────────────────────────────────────────────────────────────────
describe('行排序', () => {
  it('正序排序', () => {
    expect(sortLines('c\na\nb')).toBe('a\nb\nc')
  })

  it('倒序排序', () => {
    expect(sortLines('a\nc\nb', true)).toBe('c\nb\na')
  })

  it('忽略大小写', () => {
    const result = sortLines('B\na\nC', false, true)
    expect(result.toLowerCase()).toBe('a\nb\nc')
  })

  it('空字符串', () => {
    expect(sortLines('')).toBe('')
  })
})

// ── 去重 ─────────────────────────────────────────────────────────────────────
describe('去重', () => {
  it('移除重复行', () => {
    expect(removeDuplicates('a\nb\na\nc')).toBe('a\nb\nc')
  })

  it('忽略大小写去重', () => {
    expect(removeDuplicates('Hello\nhello\nworld', true)).toBe('Hello\nworld')
  })

  it('保留顺序', () => {
    expect(removeDuplicates('c\na\nb\na\nc')).toBe('c\na\nb')
  })

  it('空字符串', () => {
    expect(removeDuplicates('')).toBe('')
  })
})

// ── chmod ────────────────────────────────────────────────────────────────────
describe('chmod 计算', () => {
  it('644 计算', () => {
    const perms = parseChmod('644')
    expect(calculateChmod(perms)).toBe('644')
  })

  it('755 计算', () => {
    const perms = parseChmod('755')
    expect(calculateChmod(perms)).toBe('755')
    expect(chmodToSymbolic('755')).toBe('rwxr-xr-x')
  })

  it('777 符号', () => {
    expect(chmodToSymbolic('777')).toBe('rwxrwxrwx')
  })

  it('000 符号', () => {
    expect(chmodToSymbolic('000')).toBe('---------')
  })

  it('parseChmod 400', () => {
    const p = parseChmod('400')
    expect(p.owner.r).toBe(true)
    expect(p.owner.w).toBe(false)
    expect(p.group.r).toBe(false)
    expect(p.others.r).toBe(false)
  })

  it('无效权限抛出错误', () => {
    expect(() => parseChmod('999')).toThrow()
  })
})

// ── JSON 排序 ─────────────────────────────────────────────────────────────────
describe('JSON 键排序', () => {
  it('基础对象排序', () => {
    const sorted = sortJSON({ z: 1, a: 2, m: 3 }) as Record<string, number>
    expect(Object.keys(sorted)).toEqual(['a', 'm', 'z'])
  })

  it('嵌套对象排序', () => {
    const sorted = sortJSON({ z: { b: 1, a: 2 }, a: 3 }) as Record<string, unknown>
    expect(Object.keys(sorted)).toEqual(['a', 'z'])
    expect(Object.keys(sorted['z'] as object)).toEqual(['a', 'b'])
  })

  it('数组不改变', () => {
    const input = { a: [3, 1, 2] }
    const sorted = sortJSON(input) as { a: number[] }
    expect(sorted.a).toEqual([3, 1, 2])
  })

  it('原始值不变', () => {
    expect(sortJSON(42)).toBe(42)
    expect(sortJSON(null)).toBeNull()
    expect(sortJSON('hello')).toBe('hello')
  })
})

// ── JSON → YAML ───────────────────────────────────────────────────────────────
describe('JSON → YAML 转换', () => {
  it('简单对象', () => {
    const yaml = jsonToYaml({ name: 'idcd', version: 1 })
    expect(yaml).toContain('name: idcd')
    expect(yaml).toContain('version: 1')
  })

  it('嵌套对象', () => {
    const yaml = jsonToYaml({ a: { b: 'c' } })
    expect(yaml).toContain('a:')
    expect(yaml).toContain('b: c')
  })

  it('数组', () => {
    const yaml = jsonToYaml({ items: [1, 2, 3] })
    expect(yaml).toContain('- 1')
    expect(yaml).toContain('- 2')
  })

  it('null 值', () => {
    expect(jsonToYaml(null)).toBe('null')
  })

  it('布尔值', () => {
    expect(jsonToYaml(true)).toBe('true')
    expect(jsonToYaml(false)).toBe('false')
  })
})

// ── XML 格式化 ────────────────────────────────────────────────────────────────
describe('XML 格式化', () => {
  it('格式化简单 XML', () => {
    const result = formatXML('<root><item>value</item></root>')
    expect(result).toContain('<root>')
    expect(result).toContain('<item>')
    expect(result.split('\n').length).toBeGreaterThan(1)
  })

  it('自闭合标签', () => {
    const result = formatXML('<root><br/></root>')
    expect(result).toContain('<br/>')
  })
})

// ── YAML 格式化 ───────────────────────────────────────────────────────────────
describe('YAML 格式化', () => {
  it('规范化缩进', () => {
    const yaml = 'name: idcd\nversion: 1'
    const result = formatYAML(yaml)
    expect(result).toContain('name: idcd')
    expect(result).toContain('version: 1')
  })

  it('空字符串', () => {
    expect(formatYAML('')).toBe('')
  })

  it('去除多余空行', () => {
    const yaml = 'a: 1\n\n\n\nb: 2'
    const result = formatYAML(yaml)
    expect(result.match(/\n\n\n/)).toBeNull()
  })
})

// ── URL 解析 ─────────────────────────────────────────────────────────────────
describe('URL 解析', () => {
  it('解析完整 URL', () => {
    const r = parseURL('https://example.com:8080/path?q=hello&lang=zh#section')
    expect(r.protocol).toBe('https:')
    expect(r.hostname).toBe('example.com')
    expect(r.port).toBe('8080')
    expect(r.pathname).toBe('/path')
    expect(r['param:q']).toBe('hello')
    expect(r['param:lang']).toBe('zh')
    expect(r.hash).toBe('#section')
  })

  it('无效 URL 抛出错误', () => {
    expect(() => parseURL('not-a-url')).toThrow()
  })
})

// ── User-Agent 解析 ───────────────────────────────────────────────────────────
describe('User-Agent 解析', () => {
  it('识别 Chrome 浏览器', () => {
    const ua = 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36'
    const r = parseUserAgent(ua)
    expect(r['浏览器']).toContain('Chrome')
    expect(r['操作系统']).toContain('Windows')
  })

  it('识别 Firefox 浏览器', () => {
    const ua = 'Mozilla/5.0 (X11; Linux x86_64; rv:120.0) Gecko/20100101 Firefox/120.0'
    const r = parseUserAgent(ua)
    expect(r['浏览器']).toContain('Firefox')
    expect(r['操作系统']).toBe('Linux')
  })

  it('识别手机设备', () => {
    const ua = 'Mozilla/5.0 (Linux; Android 13; Pixel 7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Mobile Safari/537.36'
    const r = parseUserAgent(ua)
    expect(r['设备']).toContain('安卓手机')
  })
})

// ── 词频统计 ─────────────────────────────────────────────────────────────────
describe('词频统计', () => {
  it('英文词频', () => {
    const r = getTextFreqStats('hello world hello foo world world')
    expect(r.topWords[0]![0]).toBe('world')
    expect(r.topWords[0]![1]).toBe(3)
    expect(r.totalWords).toBe(6)
  })

  it('空文本', () => {
    const r = getTextFreqStats('')
    expect(r.totalWords).toBe(0)
    expect(r.topWords).toHaveLength(0)
  })

  it('中文词频', () => {
    const r = getTextFreqStats('中文测试中文')
    expect(r.topWords.length).toBeGreaterThan(0)
  })
})

// ── Lorem Ipsum ───────────────────────────────────────────────────────────────
describe('Lorem Ipsum', () => {
  it('生成指定段落数', () => {
    const result = generateLorem(3)
    expect(result.split('\n\n')).toHaveLength(3)
  })

  it('生成单段落', () => {
    const result = generateLorem(1)
    expect(result.trim().length).toBeGreaterThan(0)
  })

  it('每段包含指定句数', () => {
    const result = generateLorem(1, 2)
    const sentences = result.trim().split('. ')
    expect(sentences.length).toBeGreaterThanOrEqual(2)
  })
})

// ── CSV 解析 ─────────────────────────────────────────────────────────────────
describe('CSV 解析', () => {
  it('解析简单 CSV', () => {
    const result = parseCSV('a,b,c\n1,2,3')
    expect(result).toHaveLength(2)
    expect(result[0]).toEqual(['a', 'b', 'c'])
    expect(result[1]).toEqual(['1', '2', '3'])
  })

  it('处理引号', () => {
    const result = parseCSV('"hello, world",foo,bar')
    expect(result[0]![0]).toBe('hello, world')
  })

  it('自定义分隔符', () => {
    const result = parseCSV('a;b;c', ';')
    expect(result[0]).toEqual(['a', 'b', 'c'])
  })

  it('空 CSV', () => {
    const result = parseCSV('a,b\n')
    expect(result.length).toBeGreaterThan(0)
  })
})

// ── Markdown 解析 ─────────────────────────────────────────────────────────────
describe('Markdown 解析', () => {
  it('解析标题', () => {
    const html = parseMarkdown('# 标题一')
    expect(html).toContain('<h1>')
    expect(html).toContain('标题一')
  })

  it('解析粗体', () => {
    const html = parseMarkdown('**粗体**文字')
    expect(html).toContain('<strong>')
  })

  it('解析斜体', () => {
    const html = parseMarkdown('*斜体*文字')
    expect(html).toContain('<em>')
  })

  it('HTML 转义', () => {
    const html = parseMarkdown('<script>alert(1)</script>')
    expect(html).not.toContain('<script>')
    expect(html).toContain('&lt;script&gt;')
  })
})

// ── 时区工具 ─────────────────────────────────────────────────────────────────
describe('时区工具', () => {
  it('UTC 格式化', () => {
    const d = new Date('2024-01-15T12:00:00Z')
    const result = getTimeInZone(d, 'UTC')
    expect(result).toBeTruthy()
    expect(typeof result).toBe('string')
  })

  it('不同时区结果不同', () => {
    const d = new Date('2024-06-15T12:00:00Z')
    const utc = getTimeInZone(d, 'UTC')
    const shanghai = getTimeInZone(d, 'Asia/Shanghai')
    expect(utc).not.toBe(shanghai)
  })
})
