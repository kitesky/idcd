"use client"

import { useState } from 'react'
import { useTranslations } from 'next-intl'
import {
  Card, CardContent, CardHeader, CardTitle,
  Textarea, Button, Badge, Label, Input,
} from '@/components/ui'
import {
  urlEncode, urlDecode,
  charToCodePoint, codePointToChar, stringToUnicode, unicodeToString,
  decodeJWT,
  numberToAllBases, NUMBER_BASE_KEYS,
  jsonToYaml, formatYAML, formatXML,
  parseURL,
  parseUserAgent,
  formatNumber, formatCurrency,
} from '@/lib/tool-functions'
import type { NumberBaseKey } from '@/lib/tool-functions'
import { translateToolError } from '@/lib/tool-error'

// ── URL 编解码 ───────────────────────────────────────────────────────────────
export function UrlEncodeClient() {
  const tErr = useTranslations('docs.toolFunctions.errors')
  const [input, setInput] = useState('')
  const [mode, setMode] = useState<'encode' | 'decode'>('encode')
  const [error, setError] = useState('')

  const processInput = (value: string) => {
    setInput(value)
    setError('')
  }

  const getOutput = () => {
    if (!input) return ''
    try {
      return mode === 'encode' ? urlEncode(input) : urlDecode(input)
    } catch (e) {
      setError(translateToolError(e, tErr as never, '处理失败'))
      return ''
    }
  }

  const output = getOutput()

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">URL 编解码</h1>
        <p className="text-muted-foreground mt-2">encodeURIComponent / decodeURIComponent</p>
      </div>
      <div className="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle>输入</CardTitle>
              <div className="flex border rounded-lg p-1">
                {(['encode', 'decode'] as const).map(m => (
                  <Button
                    key={m}
                    size="sm"
                    variant={mode === m ? 'default' : 'ghost'}
                    className="h-7 px-3"
                    onClick={() => setMode(m)}
                  >
                    {m === 'encode' ? '编码' : '解码'}
                  </Button>
                ))}
              </div>
            </div>
          </CardHeader>
          <CardContent>
            <Textarea
              placeholder={mode === 'encode' ? 'https://example.com/path?q=中文&foo=bar' : 'https%3A%2F%2Fexample.com%2F'}
              value={input}
              onChange={e => processInput(e.target.value)}
              className="min-h-[200px] font-mono text-sm"
            />
          </CardContent>
        </Card>
        <Card>
          <CardHeader><CardTitle>结果</CardTitle></CardHeader>
          <CardContent className="space-y-2">
            {error && <Badge variant="destructive">{error}</Badge>}
            <Textarea
              value={output}
              readOnly
              className="min-h-[200px] font-mono text-sm bg-muted/50"
              placeholder="结果…"
            />
            {output && (
              <Button variant="outline" size="sm" onClick={() => navigator.clipboard.writeText(output)}>
                复制
              </Button>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}

// ── Unicode 转换 ─────────────────────────────────────────────────────────────
export function UnicodeClient() {
  const tErr = useTranslations('docs.toolFunctions.errors')
  const [char, setChar] = useState('')
  const [codePoint, setCodePoint] = useState('')
  const [text, setText] = useState('')
  const [escaped, setEscaped] = useState('')
  const [charResult, setCharResult] = useState('')
  const [cpResult, setCpResult] = useState('')
  const [cpError, setCpError] = useState('')

  const handleCharInput = (v: string) => {
    setChar(v)
    setCpResult(v ? Array.from(v).map(charToCodePoint).join(' ') : '')
  }

  const handleCpInput = (v: string) => {
    setCodePoint(v)
    setCpError('')
    try {
      setCharResult(v ? codePointToChar(v) : '')
    } catch (e) {
      setCpError(translateToolError(e, tErr as never, '无效'))
      setCharResult('')
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">Unicode 转换</h1>
        <p className="text-muted-foreground mt-2">字符与 Unicode 码点互转</p>
      </div>
      <div className="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader><CardTitle>字符 → 码点</CardTitle></CardHeader>
          <CardContent className="space-y-3">
            <div className="space-y-1">
              <Label>输入字符</Label>
              <Input placeholder="A 或 中 或 😀" value={char} onChange={e => handleCharInput(e.target.value)} />
            </div>
            {cpResult && (
              <div className="space-y-1">
                <Label>码点</Label>
                <code className="block bg-muted p-3 rounded text-sm font-mono">{cpResult}</code>
              </div>
            )}
          </CardContent>
        </Card>
        <Card>
          <CardHeader><CardTitle>码点 → 字符</CardTitle></CardHeader>
          <CardContent className="space-y-3">
            <div className="space-y-1">
              <Label>输入码点（如 U+4E2D 或 4E2D）</Label>
              <Input placeholder="U+4E2D" value={codePoint} onChange={e => handleCpInput(e.target.value)} />
            </div>
            {cpError && <Badge variant="destructive">{cpError}</Badge>}
            {charResult && (
              <div className="text-5xl text-center py-4 border rounded">{charResult}</div>
            )}
          </CardContent>
        </Card>
      </div>
      <Card>
        <CardHeader><CardTitle>文本 ↔ Unicode 转义序列</CardTitle></CardHeader>
        <CardContent className="grid gap-4 lg:grid-cols-2">
          <div className="space-y-2">
            <Label>文本</Label>
            <Textarea
              placeholder="你好，World！"
              value={text}
              onChange={e => { setText(e.target.value); setEscaped(stringToUnicode(e.target.value)) }}
              className="font-mono text-sm min-h-[100px]"
            />
          </div>
          <div className="space-y-2">
            <Label>Unicode 转义</Label>
            <Textarea
              placeholder="\\u{4F60}\\u{597D}"
              value={escaped}
              onChange={e => { setEscaped(e.target.value); try { setText(unicodeToString(e.target.value)) } catch {} }}
              className="font-mono text-sm min-h-[100px]"
            />
          </div>
        </CardContent>
      </Card>
    </div>
  )
}

// ── JWT 解码 ─────────────────────────────────────────────────────────────────
export function JwtDecodeClient() {
  const tErr = useTranslations('docs.toolFunctions.errors')
  const [input, setInput] = useState('')
  const [result, setResult] = useState<ReturnType<typeof decodeJWT> | null>(null)
  const [error, setError] = useState('')

  const handleChange = (v: string) => {
    setInput(v)
    if (!v.trim()) { setResult(null); setError(''); return }
    try {
      setResult(decodeJWT(v))
      setError('')
    } catch (e) {
      setError(translateToolError(e, tErr as never, '解码失败'))
      setResult(null)
    }
  }

  const fmt = (ts: number) => new Date(ts * 1000).toLocaleString('zh-CN')
  // eslint-disable-next-line react-hooks/purity -- 判断 JWT 过期需要当前时间，仅用于展示
  const expired = (exp?: number) => exp ? Date.now() / 1000 > exp : false

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">JWT 解码</h1>
        <p className="text-muted-foreground mt-2">解析 JSON Web Token，查看 Header 和 Payload（不验签）</p>
      </div>
      <Card>
        <CardHeader><CardTitle>JWT Token</CardTitle></CardHeader>
        <CardContent>
          <Textarea
            placeholder="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
            value={input}
            onChange={e => handleChange(e.target.value)}
            className="min-h-[100px] font-mono text-sm"
          />
          {error && <Badge variant="destructive" className="mt-2">{error}</Badge>}
        </CardContent>
      </Card>
      {result && (
        <div className="space-y-4">
          <Card>
            <CardHeader><CardTitle>Header</CardTitle></CardHeader>
            <CardContent>
              <pre className="bg-muted/50 p-4 rounded font-mono text-sm overflow-x-auto">
                {JSON.stringify(result.header, null, 2)}
              </pre>
            </CardContent>
          </Card>
          <Card>
            <CardHeader><CardTitle>Payload</CardTitle></CardHeader>
            <CardContent className="space-y-3">
              <pre className="bg-muted/50 p-4 rounded font-mono text-sm overflow-x-auto">
                {JSON.stringify(result.payload, null, 2)}
              </pre>
              {Boolean(result.payload.iat ?? result.payload.exp ?? result.payload.nbf) && (
                <div className="text-sm space-y-1 border-t pt-3">
                  {result.payload.iat != null && <p><strong>iat（签发）：</strong>{fmt(Number(result.payload.iat))}</p>}
                  {result.payload.exp != null && (
                    <p className="flex items-center gap-2">
                      <strong>exp（过期）：</strong>{fmt(Number(result.payload.exp))}
                      <Badge variant={expired(Number(result.payload.exp)) ? 'destructive' : 'default'}>
                        {expired(Number(result.payload.exp)) ? '已过期' : '有效'}
                      </Badge>
                    </p>
                  )}
                  {result.payload.nbf != null && <p><strong>nbf（生效）：</strong>{fmt(Number(result.payload.nbf))}</p>}
                </div>
              )}
            </CardContent>
          </Card>
          <Card>
            <CardHeader><CardTitle>Signature</CardTitle></CardHeader>
            <CardContent>
              <div className="bg-muted/50 p-4 rounded font-mono text-sm break-all">{result.signature}</div>
              <p className="text-xs text-muted-foreground mt-2">仅解码，不验证签名有效性</p>
            </CardContent>
          </Card>
        </div>
      )}
    </div>
  )
}

// ── 进制转换 ─────────────────────────────────────────────────────────────────
export function NumberConvertClient() {
  const tErr = useTranslations('docs.toolFunctions.errors')
  const tBase = useTranslations('docs.toolFunctions.numberBases')
  const [input, setInput] = useState('')
  const [fromBase, setFromBase] = useState(10)
  const [result, setResult] = useState<Record<NumberBaseKey, string> | null>(null)
  const [error, setError] = useState('')

  const bases: { key: NumberBaseKey; value: number }[] = [
    { key: 'decimal', value: 10 },
    { key: 'hex', value: 16 },
    { key: 'binary', value: 2 },
    { key: 'octal', value: 8 },
  ]

  const handleConvert = () => {
    if (!input.trim()) return
    try {
      setResult(numberToAllBases(input, fromBase))
      setError('')
    } catch (e) {
      setError(translateToolError(e, tErr as never, '转换失败'))
      setResult(null)
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">进制转换</h1>
        <p className="text-muted-foreground mt-2">二进制、八进制、十进制、十六进制互转</p>
      </div>
      <Card>
        <CardHeader><CardTitle>输入数字</CardTitle></CardHeader>
        <CardContent className="space-y-4">
          <div className="flex gap-3 flex-wrap">
            {bases.map(b => (
              <Button
                key={b.value}
                size="sm"
                variant={fromBase === b.value ? 'default' : 'outline'}
                onClick={() => setFromBase(b.value)}
              >
                {tBase(b.key)} ({b.value})
              </Button>
            ))}
          </div>
          <div className="flex gap-2">
            <Input
              placeholder={fromBase === 16 ? 'FF 或 1A2B' : fromBase === 2 ? '1010 1100' : String(fromBase === 8 ? '377' : '255')}
              value={input}
              onChange={e => setInput(e.target.value)}
              className="font-mono"
            />
            <Button onClick={handleConvert}>转换</Button>
          </div>
          {error && <Badge variant="destructive">{error}</Badge>}
        </CardContent>
      </Card>
      {result && (
        <Card>
          <CardHeader><CardTitle>转换结果</CardTitle></CardHeader>
          <CardContent className="space-y-2">
            {NUMBER_BASE_KEYS.map(key => (
              <div key={key} className="flex items-center gap-4 py-2 border-b last:border-0">
                <span className="text-muted-foreground text-sm w-24 shrink-0">{tBase(key)}</span>
                <code className="flex-1 font-mono text-sm">{result[key]}</code>
                <Button variant="outline" size="sm" onClick={() => navigator.clipboard.writeText(result[key])}>复制</Button>
              </div>
            ))}
          </CardContent>
        </Card>
      )}
    </div>
  )
}

// ── JSON ↔ YAML ───────────────────────────────────────────────────────────────
export function JsonToYamlClient() {
  const tErr = useTranslations('docs.toolFunctions.errors')
  const [input, setInput] = useState('')
  const [mode, setMode] = useState<'j2y' | 'y2j'>('j2y')
  const [output, setOutput] = useState('')
  const [error, setError] = useState('')

  const convert = () => {
    setError('')
    try {
      if (mode === 'j2y') {
        const obj = JSON.parse(input)
        setOutput(jsonToYaml(obj))
      } else {
        // YAML → JSON: handle basic cases
        const lines = input.split('\n')
        const obj: Record<string, unknown> = {}
        for (const line of lines) {
          const match = line.match(/^(\w[\w-]*)\s*:\s*(.+)$/)
          if (match) obj[match[1]!] = isNaN(Number(match[2])) ? match[2]! : Number(match[2])
        }
        setOutput(JSON.stringify(obj, null, 2))
      }
    } catch (e) {
      setError(translateToolError(e, tErr as never, '转换失败'))
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">JSON ↔ YAML</h1>
        <p className="text-muted-foreground mt-2">JSON 与 YAML 格式互相转换</p>
      </div>
      <div className="flex border rounded-lg p-1 w-fit">
        <Button size="sm" variant={mode === 'j2y' ? 'default' : 'ghost'} onClick={() => setMode('j2y')}>
          JSON → YAML
        </Button>
        <Button size="sm" variant={mode === 'y2j' ? 'default' : 'ghost'} onClick={() => setMode('y2j')}>
          YAML → JSON
        </Button>
      </div>
      <div className="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader><CardTitle>{mode === 'j2y' ? 'JSON 输入' : 'YAML 输入'}</CardTitle></CardHeader>
          <CardContent className="space-y-3">
            <Textarea
              placeholder={mode === 'j2y' ? '{"name": "idcd", "version": 1}' : 'name: idcd\nversion: 1'}
              value={input}
              onChange={e => setInput(e.target.value)}
              className="min-h-[280px] font-mono text-sm"
            />
            <Button onClick={convert}>转换</Button>
          </CardContent>
        </Card>
        <Card>
          <CardHeader><CardTitle>{mode === 'j2y' ? 'YAML 输出' : 'JSON 输出'}</CardTitle></CardHeader>
          <CardContent className="space-y-2">
            {error && <Badge variant="destructive">{error}</Badge>}
            <Textarea value={output} readOnly className="min-h-[280px] font-mono text-sm bg-muted/50" placeholder="结果…" />
            {output && <Button variant="outline" size="sm" onClick={() => navigator.clipboard.writeText(output)}>复制</Button>}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}

// ── YAML 格式化 ──────────────────────────────────────────────────────────────
export function YamlFormatterClient() {
  const tErr = useTranslations('docs.toolFunctions.errors')
  const [input, setInput] = useState('')
  const [output, setOutput] = useState('')
  const [error, setError] = useState('')

  const format = () => {
    try {
      setOutput(formatYAML(input))
      setError('')
    } catch (e) {
      setError(translateToolError(e, tErr as never, '格式化失败'))
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">YAML 格式化</h1>
        <p className="text-muted-foreground mt-2">YAML 缩进规范化和格式美化</p>
      </div>
      <div className="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader><CardTitle>YAML 输入</CardTitle></CardHeader>
          <CardContent className="space-y-3">
            <Textarea value={input} onChange={e => setInput(e.target.value)} className="min-h-[300px] font-mono text-sm" placeholder="粘贴 YAML 内容…" />
            <Button onClick={format}>格式化</Button>
          </CardContent>
        </Card>
        <Card>
          <CardHeader><CardTitle>格式化结果</CardTitle></CardHeader>
          <CardContent className="space-y-2">
            {error && <Badge variant="destructive">{error}</Badge>}
            <Textarea value={output} readOnly className="min-h-[300px] font-mono text-sm bg-muted/50" />
            {output && <Button variant="outline" size="sm" onClick={() => navigator.clipboard.writeText(output)}>复制</Button>}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}

// ── XML 格式化 ───────────────────────────────────────────────────────────────
export function XmlFormatterClient() {
  const tErr = useTranslations('docs.toolFunctions.errors')
  const [input, setInput] = useState('')
  const [output, setOutput] = useState('')
  const [error, setError] = useState('')
  const [mode, setMode] = useState<'format' | 'minify'>('format')

  const process = () => {
    if (!input.trim()) return
    try {
      if (mode === 'minify') {
        setOutput(input.replace(/>\s+</g, '><').trim())
      } else {
        setOutput(formatXML(input))
      }
      setError('')
    } catch (e) {
      setError(translateToolError(e, tErr as never, '处理失败'))
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">XML 格式化</h1>
        <p className="text-muted-foreground mt-2">XML 格式美化和压缩</p>
      </div>
      <div className="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle>XML 输入</CardTitle>
              <div className="flex border rounded-lg p-1">
                {(['format', 'minify'] as const).map(m => (
                  <Button
                    key={m}
                    size="sm"
                    variant={mode === m ? 'default' : 'ghost'}
                    className="h-7 px-3"
                    onClick={() => setMode(m)}
                  >
                    {m === 'format' ? '美化' : '压缩'}
                  </Button>
                ))}
              </div>
            </div>
          </CardHeader>
          <CardContent className="space-y-3">
            <Textarea value={input} onChange={e => setInput(e.target.value)} className="min-h-[300px] font-mono text-sm" placeholder="<root><item>value</item></root>" />
            <Button onClick={process}>{mode === 'format' ? '美化' : '压缩'}</Button>
          </CardContent>
        </Card>
        <Card>
          <CardHeader><CardTitle>结果</CardTitle></CardHeader>
          <CardContent className="space-y-2">
            {error && <Badge variant="destructive">{error}</Badge>}
            <Textarea value={output} readOnly className="min-h-[300px] font-mono text-sm bg-muted/50" />
            {output && <Button variant="outline" size="sm" onClick={() => navigator.clipboard.writeText(output)}>复制</Button>}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}

// ── URL 解析 ─────────────────────────────────────────────────────────────────
export function UrlParserClient() {
  const tErr = useTranslations('docs.toolFunctions.errors')
  const [url, setUrl] = useState('https://example.com:8080/path/to/page?foo=bar&q=%E4%B8%AD%E6%96%87#section')
  const [result, setResult] = useState<Record<string, string> | null>(null)
  const [error, setError] = useState('')

  const parse = () => {
    try {
      setResult(parseURL(url))
      setError('')
    } catch (e) {
      setError(translateToolError(e, tErr as never, '解析失败，请确认 URL 格式正确'))
      setResult(null)
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">URL 解析</h1>
        <p className="text-muted-foreground mt-2">解析 URL 各组成部分</p>
      </div>
      <Card>
        <CardHeader><CardTitle>输入 URL</CardTitle></CardHeader>
        <CardContent className="space-y-3">
          <Input
            placeholder="https://example.com/path?q=hello#section"
            value={url}
            onChange={e => setUrl(e.target.value)}
            className="font-mono text-sm"
          />
          <Button onClick={parse}>解析</Button>
          {error && <Badge variant="destructive">{error}</Badge>}
        </CardContent>
      </Card>
      {result && (
        <Card>
          <CardHeader><CardTitle>解析结果</CardTitle></CardHeader>
          <CardContent className="space-y-2">
            {Object.entries(result).map(([k, v]) => v ? (
              <div key={k} className="flex gap-3 text-sm py-1.5 border-b last:border-0">
                <span className="text-muted-foreground w-28 shrink-0 font-medium">{k.startsWith('param:') ? `参数: ${k.slice(6)}` : k}</span>
                <code className="flex-1 font-mono break-all">{v}</code>
              </div>
            ) : null)}
          </CardContent>
        </Card>
      )}
    </div>
  )
}

// ── User-Agent 解析 ───────────────────────────────────────────────────────────
export function UserAgentClient() {
  const [ua, setUa] = useState(
    typeof navigator !== 'undefined' ? navigator.userAgent : 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 Chrome/124.0 Safari/537.36'
  )
  const result = ua.trim() ? parseUserAgent(ua) : null

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">UA 解析</h1>
        <p className="text-muted-foreground mt-2">解析 User-Agent 字符串，识别浏览器、操作系统和设备</p>
      </div>
      <Card>
        <CardHeader><CardTitle>User-Agent</CardTitle></CardHeader>
        <CardContent className="space-y-2">
          <Textarea
            value={ua}
            onChange={e => setUa(e.target.value)}
            className="min-h-[80px] font-mono text-sm"
            placeholder="粘贴 User-Agent 字符串…"
          />
          <Button variant="outline" size="sm" onClick={() => typeof navigator !== 'undefined' && setUa(navigator.userAgent)}>
            使用当前浏览器 UA
          </Button>
        </CardContent>
      </Card>
      {result && (
        <Card>
          <CardHeader><CardTitle>解析结果</CardTitle></CardHeader>
          <CardContent className="space-y-2">
            {Object.entries(result).map(([k, v]) => (
              <div key={k} className="flex justify-between py-2 border-b last:border-0">
                <span className="text-muted-foreground text-sm">{k}</span>
                <span className="font-medium text-sm">{v}</span>
              </div>
            ))}
          </CardContent>
        </Card>
      )}
    </div>
  )
}

// ── 数字格式化 ────────────────────────────────────────────────────────────────
export function NumberFormatClient() {
  const [input, setInput] = useState('1234567.89')
  const [_error, setError] = useState('')

  const num = parseFloat(input)
  const valid = !isNaN(num)

  const formats = valid
    ? [
        ['普通千分位', formatNumber(num)],
        ['两位小数', formatNumber(num, 'zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })],
        ['人民币', formatCurrency(num, 'CNY')],
        ['美元', formatCurrency(num, 'USD', 'en-US')],
        ['欧元', formatCurrency(num, 'EUR', 'de-DE')],
        ['科学计数', formatNumber(num, 'zh-CN', { notation: 'scientific' })],
        ['紧凑格式', formatNumber(num, 'zh-CN', { notation: 'compact' })],
        ['百分比', formatNumber(num / 100, 'zh-CN', { style: 'percent', minimumFractionDigits: 2 })],
      ]
    : []

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">数字格式化</h1>
        <p className="text-muted-foreground mt-2">多种数字格式化输出</p>
      </div>
      <Card>
        <CardHeader><CardTitle>输入数字</CardTitle></CardHeader>
        <CardContent>
          <Input
            placeholder="1234567.89"
            value={input}
            onChange={e => { setInput(e.target.value); setError('') }}
            className="font-mono"
          />
          {!valid && input && <p className="text-sm text-destructive mt-1">请输入有效数字</p>}
        </CardContent>
      </Card>
      {formats.length > 0 && (
        <Card>
          <CardHeader><CardTitle>格式化结果</CardTitle></CardHeader>
          <CardContent className="space-y-2">
            {(formats as [string, string][]).map(([label, value]) => (
              <div key={label} className="flex items-center justify-between py-2 border-b last:border-0">
                <span className="text-muted-foreground text-sm">{label}</span>
                <div className="flex items-center gap-2">
                  <code className="font-mono text-sm">{value}</code>
                  <Button variant="outline" size="sm" onClick={() => navigator.clipboard.writeText(value)}>复制</Button>
                </div>
              </div>
            ))}
          </CardContent>
        </Card>
      )}
    </div>
  )
}
