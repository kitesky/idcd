"use client"

import { useState, useEffect } from 'react'
import { useTranslations } from 'next-intl'
import {
  Card, CardContent, CardHeader, CardTitle,
  Button, Badge, Label, Input, Textarea,
} from '@/components/ui'
import { Cron } from 'croner'
import {
  parseCIDR,
  checkIPv6,
  HTTP_STATUS_CODES, MIME_TYPES,
  TIMEZONES, getTimeInZone,
  dateDiff, addDays,
  parseCSV,
} from '@/lib/tool-functions'
import { translateToolError } from '@/lib/tool-error'

// ── 正则表达式测试 ────────────────────────────────────────────────────────────
export function RegexClient() {
  const tErr = useTranslations('docs.toolFunctions.errors')
  const [pattern, setPattern] = useState('(\\w+)@(\\w+\\.\\w+)')
  const [flags, setFlags] = useState('g')
  const [testStr, setTestStr] = useState('联系我们：hello@example.com 或 support@idcd.com')
  const [error, setError] = useState('')

  let regex: RegExp | null = null
  let matches: RegExpExecArray[] = []

  try {
    regex = new RegExp(pattern, flags)
    const allFlags = flags.includes('g') ? flags : flags + 'g'
    const re = new RegExp(pattern, allFlags)
    let m: RegExpExecArray | null
    while ((m = re.exec(testStr)) !== null) {
      matches.push(m)
      if (!allFlags.includes('g')) break
    }
    if (error) setError('')
  } catch (e) {
    if (!error) setError(translateToolError(e, tErr as never, '无效的正则表达式'))
  }

  const flagOptions = ['g', 'i', 'm', 's', 'u']

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">正则表达式测试</h1>
        <p className="text-muted-foreground mt-2">实时测试正则表达式，高亮显示匹配结果</p>
      </div>
      <Card>
        <CardHeader><CardTitle>正则表达式</CardTitle></CardHeader>
        <CardContent className="space-y-3">
          <div className="flex items-center gap-2">
            <span className="text-muted-foreground">/</span>
            <Input
              value={pattern}
              onChange={e => setPattern(e.target.value)}
              className="font-mono flex-1"
              placeholder="正则表达式"
            />
            <span className="text-muted-foreground">/</span>
            <Input
              value={flags}
              onChange={e => setFlags(e.target.value.replace(/[^gimsuy]/g, ''))}
              className="font-mono w-20"
              placeholder="flags"
            />
          </div>
          <div className="flex gap-2 flex-wrap">
            {flagOptions.map(f => (
              <button
                key={f}
                onClick={() => setFlags(prev => prev.includes(f) ? prev.replace(f, '') : prev + f)}
                className={`px-2 py-0.5 text-xs rounded border font-mono transition-colors ${flags.includes(f) ? 'bg-primary text-primary-foreground border-primary' : 'border-border'}`}
              >
                {f}
              </button>
            ))}
          </div>
          {error && <Badge variant="destructive">{error}</Badge>}
        </CardContent>
      </Card>
      <Card>
        <CardHeader><CardTitle>测试文本</CardTitle></CardHeader>
        <CardContent>
          <Textarea
            value={testStr}
            onChange={e => setTestStr(e.target.value)}
            className="min-h-[120px] font-mono text-sm"
            placeholder="在此输入要测试的文本…"
          />
        </CardContent>
      </Card>
      {regex && !error && (
        <Card>
          <CardHeader>
            <div className="flex items-center gap-2">
              <CardTitle>匹配结果</CardTitle>
              <Badge variant={matches.length > 0 ? 'default' : 'secondary'}>
                {matches.length} 个匹配
              </Badge>
            </div>
          </CardHeader>
          <CardContent className="space-y-3">
            {matches.length === 0 ? (
              <p className="text-muted-foreground text-sm">无匹配</p>
            ) : (
              matches.map((m, i) => (
                <div key={i} className="border rounded p-3 space-y-1">
                  <div className="flex gap-3 text-sm">
                    <span className="text-muted-foreground w-20">完整匹配</span>
                    <code className="font-mono bg-muted/50 px-1 rounded">{m[0]}</code>
                    <span className="text-muted-foreground">位置 {m.index}</span>
                  </div>
                  {m.slice(1).map((group, gi) => group !== undefined && (
                    <div key={gi} className="flex gap-3 text-sm">
                      <span className="text-muted-foreground w-20">分组 {gi + 1}</span>
                      <code className="font-mono bg-muted/50 px-1 rounded">{group}</code>
                    </div>
                  ))}
                </div>
              ))
            )}
          </CardContent>
        </Card>
      )}
    </div>
  )
}

// ── Cron 可视化 ───────────────────────────────────────────────────────────────
export function CronVizClient() {
  const tErr = useTranslations('docs.toolFunctions.errors')
  const [expr, setExpr] = useState('*/5 * * * *')
  const [error, setError] = useState('')
  const [nextTimes, setNextTimes] = useState<string[]>([])

  const parse = () => {
    try {
      const cron = new Cron(expr, { maxRuns: 10 })
      const times: string[] = []
      for (let i = 0; i < 10; i++) {
        const next = cron.nextRun()
        if (!next) break
        times.push(next.toLocaleString('zh-CN', {
          year: 'numeric', month: '2-digit', day: '2-digit',
          hour: '2-digit', minute: '2-digit', second: '2-digit',
        }))
      }
      setNextTimes(times)
      setError('')
    } catch (e) {
      setError(translateToolError(e, tErr as never, '无效的 Cron 表达式'))
      setNextTimes([])
    }
  }

  const presets = [
    { label: '每分钟', expr: '* * * * *' },
    { label: '每5分钟', expr: '*/5 * * * *' },
    { label: '每小时', expr: '0 * * * *' },
    { label: '每天午夜', expr: '0 0 * * *' },
    { label: '每周一', expr: '0 0 * * 1' },
    { label: '每月1日', expr: '0 0 1 * *' },
  ]

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">Cron 可视化</h1>
        <p className="text-muted-foreground mt-2">解析 Cron 表达式，查看下次执行时间</p>
      </div>
      <Card>
        <CardHeader><CardTitle>Cron 表达式</CardTitle></CardHeader>
        <CardContent className="space-y-3">
          <div className="flex gap-2">
            <Input
              value={expr}
              onChange={e => setExpr(e.target.value)}
              className="font-mono flex-1"
              placeholder="* * * * *"
            />
            <Button onClick={parse}>解析</Button>
          </div>
          <div className="text-xs text-muted-foreground font-mono">
            分&nbsp;&nbsp;&nbsp;&nbsp;时&nbsp;&nbsp;&nbsp;&nbsp;日&nbsp;&nbsp;&nbsp;&nbsp;月&nbsp;&nbsp;&nbsp;&nbsp;星期
          </div>
          <div className="flex flex-wrap gap-2">
            {presets.map(p => (
              <button
                key={p.expr}
                onClick={() => setExpr(p.expr)}
                className="px-2 py-1 text-xs rounded border hover:bg-muted transition-colors font-mono"
              >
                {p.label} ({p.expr})
              </button>
            ))}
          </div>
          {error && <Badge variant="destructive">{error}</Badge>}
        </CardContent>
      </Card>
      {nextTimes.length > 0 && (
        <Card>
          <CardHeader><CardTitle>未来 10 次执行时间</CardTitle></CardHeader>
          <CardContent className="space-y-1">
            {nextTimes.map((t, i) => (
              <div key={i} className="flex gap-3 text-sm py-1.5 border-b last:border-0">
                <span className="text-muted-foreground w-6">{i + 1}</span>
                <code className="font-mono">{t}</code>
              </div>
            ))}
          </CardContent>
        </Card>
      )}
    </div>
  )
}

// ── CIDR 计算器 ───────────────────────────────────────────────────────────────
export function CidrCalcClient() {
  const tErr = useTranslations('docs.toolFunctions.errors')
  const [input, setInput] = useState('192.168.1.0/24')
  const [result, setResult] = useState<ReturnType<typeof parseCIDR> | null>(null)
  const [error, setError] = useState('')

  const calculate = () => {
    try {
      setResult(parseCIDR(input))
      setError('')
    } catch (e) {
      setError(translateToolError(e, tErr as never, '计算失败'))
      setResult(null)
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">CIDR 计算器</h1>
        <p className="text-muted-foreground mt-2">计算 CIDR 子网信息</p>
      </div>
      <Card>
        <CardHeader><CardTitle>输入 CIDR</CardTitle></CardHeader>
        <CardContent className="space-y-3">
          <div className="flex gap-2">
            <Input value={input} onChange={e => setInput(e.target.value)} className="font-mono" placeholder="192.168.1.0/24" />
            <Button onClick={calculate}>计算</Button>
          </div>
          {error && <Badge variant="destructive">{error}</Badge>}
        </CardContent>
      </Card>
      {result && (
        <Card>
          <CardHeader><CardTitle>计算结果</CardTitle></CardHeader>
          <CardContent className="space-y-2">
            {[
              ['网络地址', result.network],
              ['广播地址', result.broadcast],
              ['第一个主机', result.firstHost],
              ['最后一个主机', result.lastHost],
              ['主机数量', result.hosts.toLocaleString()],
              ['总地址数', result.totalAddresses.toLocaleString()],
              ['子网掩码', result.mask],
              ['前缀长度', `/${result.cidr}`],
              ['IP 类别', result.ipClass],
            ].map(([label, value]) => (
              <div key={label} className="flex gap-4 py-1.5 border-b last:border-0 text-sm">
                <span className="text-muted-foreground w-32 shrink-0">{label}</span>
                <code className="font-mono">{value}</code>
              </div>
            ))}
          </CardContent>
        </Card>
      )}
    </div>
  )
}

// ── IPv6 检测 ─────────────────────────────────────────────────────────────────
export function Ipv6CheckClient() {
  const [input, setInput] = useState('2001:db8::1')
  const [result, setResult] = useState<ReturnType<typeof checkIPv6> | null>(null)

  const check = () => {
    const r = checkIPv6(input)
    setResult(r)
  }

  const rows = result && result.valid
    ? [
        ['完整展开', result.expanded],
        ['压缩格式', result.compressed],
        ['地址类型', result.type],
        ['IPv4 映射', result.isIPv4Mapped ? '是' : '否'],
      ]
    : []

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">IPv6 检测</h1>
        <p className="text-muted-foreground mt-2">验证 IPv6 地址格式，扩展/压缩格式互转，类型识别</p>
      </div>
      <Card>
        <CardHeader><CardTitle>输入 IPv6 地址</CardTitle></CardHeader>
        <CardContent className="space-y-3">
          <div className="flex gap-2">
            <Input value={input} onChange={e => setInput(e.target.value)} className="font-mono" placeholder="2001:db8::1 或 ::1 或 ::ffff:1.2.3.4" />
            <Button onClick={check}>检测</Button>
          </div>
          {result && (
            <Badge variant={result.valid ? 'default' : 'destructive'}>
              {result.valid ? '有效 IPv6 地址' : '无效的 IPv6 地址'}
            </Badge>
          )}
        </CardContent>
      </Card>
      {result && result.valid && (
        <Card>
          <CardHeader><CardTitle>检测结果</CardTitle></CardHeader>
          <CardContent className="space-y-2">
            {(rows as [string, string][]).map(([label, value]) => (
              <div key={label} className="flex gap-4 py-2 border-b last:border-0 text-sm">
                <span className="text-muted-foreground w-24 shrink-0">{label}</span>
                <code className="font-mono flex-1">{value}</code>
                {label !== '地址类型' && label !== 'IPv4 映射' && (
                  <Button variant="ghost" size="sm" onClick={() => navigator.clipboard.writeText(value)}>复制</Button>
                )}
              </div>
            ))}
          </CardContent>
        </Card>
      )}
    </div>
  )
}

// ── HTTP 状态码 ───────────────────────────────────────────────────────────────
export function HttpStatusClient() {
  const [query, setQuery] = useState('')

  const filtered = query
    ? HTTP_STATUS_CODES.filter(
        s => String(s.code).includes(query) ||
          s.reason.toLowerCase().includes(query.toLowerCase()) ||
          s.description.includes(query)
      )
    : HTTP_STATUS_CODES

  const groups = filtered.reduce((acc, s) => {
    if (!acc[s.category]) acc[s.category] = []
    acc[s.category]!.push(s)
    return acc
  }, {} as Record<string, typeof HTTP_STATUS_CODES>)

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">HTTP 状态码</h1>
        <p className="text-muted-foreground mt-2">HTTP 响应状态码速查</p>
      </div>
      <Card>
        <CardContent className="pt-4">
          <Input
            placeholder="搜索状态码、描述…（如 404、Not Found）"
            value={query}
            onChange={e => setQuery(e.target.value)}
          />
        </CardContent>
      </Card>
      {Object.entries(groups).map(([category, codes]) => (
        <Card key={category}>
          <CardHeader><CardTitle>{category}</CardTitle></CardHeader>
          <CardContent className="space-y-1">
            {codes.map(s => (
              <div key={s.code} className="flex items-start gap-3 py-2 border-b last:border-0 text-sm">
                <Badge variant="outline" className="font-mono w-14 justify-center shrink-0">{s.code}</Badge>
                <div>
                  <span className="font-medium">{s.reason}</span>
                  <span className="text-muted-foreground ml-2">— {s.description}</span>
                </div>
              </div>
            ))}
          </CardContent>
        </Card>
      ))}
    </div>
  )
}

// ── MIME 类型 ─────────────────────────────────────────────────────────────────
export function MimeTypeClient() {
  const [query, setQuery] = useState('')

  const filtered = query
    ? MIME_TYPES.filter(
        m => m.ext.includes(query.toLowerCase()) ||
          m.mime.includes(query.toLowerCase()) ||
          m.description.includes(query)
      )
    : MIME_TYPES

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">MIME 类型</h1>
        <p className="text-muted-foreground mt-2">文件扩展名与 MIME 类型查询</p>
      </div>
      <Card>
        <CardContent className="pt-4">
          <Input
            placeholder="搜索扩展名或 MIME 类型…（如 .jpg、image/jpeg）"
            value={query}
            onChange={e => setQuery(e.target.value)}
          />
        </CardContent>
      </Card>
      <Card>
        <CardContent className="pt-4">
          <div className="space-y-1">
            {filtered.map(m => (
              <div key={m.ext} className="flex items-center gap-3 py-2 border-b last:border-0 text-sm">
                <code className="font-mono w-16 text-muted-foreground">{m.ext}</code>
                <code className="font-mono flex-1">{m.mime}</code>
                <span className="text-muted-foreground text-xs hidden sm:block">{m.description}</span>
              </div>
            ))}
          </div>
        </CardContent>
      </Card>
    </div>
  )
}

// ── 时区转换 ─────────────────────────────────────────────────────────────────
export function TimezoneClient() {
  const [now, setNow] = useState(new Date())

  useEffect(() => {
    const id = setInterval(() => setNow(new Date()), 1000)
    return () => clearInterval(id)
  }, [])

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">时区转换</h1>
        <p className="text-muted-foreground mt-2">全球主要时区当前时间</p>
      </div>
      <Card>
        <CardContent className="pt-4 space-y-1">
          {TIMEZONES.map(tz => (
            <div key={tz.zone} className="flex items-center justify-between py-2 border-b last:border-0 text-sm">
              <span className="text-muted-foreground w-40 shrink-0">{tz.label}</span>
              <code className="font-mono">{getTimeInZone(now, tz.zone)}</code>
            </div>
          ))}
        </CardContent>
      </Card>
    </div>
  )
}

// ── 日期计算 ─────────────────────────────────────────────────────────────────
export function DateCalcClient() {
  const tErr = useTranslations('docs.toolFunctions.errors')
  const today = new Date().toISOString().split('T')[0]!
  const [date1, setDate1] = useState(today)
  const [date2, setDate2] = useState(today)
  const [addInput, setAddInput] = useState(today)
  const [days, setDays] = useState(30)
  const [diffResult, setDiffResult] = useState<number | null>(null)
  const [addResult, setAddResult] = useState('')
  const [errors, setErrors] = useState({ diff: '', add: '' })

  const calcDiff = () => {
    try {
      setDiffResult(dateDiff(date1, date2))
      setErrors(prev => ({ ...prev, diff: '' }))
    } catch (e) {
      setErrors(prev => ({ ...prev, diff: translateToolError(e, tErr as never, '错误') }))
    }
  }

  const calcAdd = () => {
    try {
      setAddResult(addDays(addInput, days))
      setErrors(prev => ({ ...prev, add: '' }))
    } catch (e) {
      setErrors(prev => ({ ...prev, add: translateToolError(e, tErr as never, '错误') }))
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">日期计算</h1>
        <p className="text-muted-foreground mt-2">日期差值和加减计算</p>
      </div>
      <Card>
        <CardHeader><CardTitle>日期差值</CardTitle></CardHeader>
        <CardContent className="space-y-3">
          <div className="flex items-center gap-3 flex-wrap">
            <div className="space-y-1">
              <Label>开始日期</Label>
              <Input type="date" value={date1} onChange={e => setDate1(e.target.value)} className="font-mono" />
            </div>
            <div className="space-y-1">
              <Label>结束日期</Label>
              <Input type="date" value={date2} onChange={e => setDate2(e.target.value)} className="font-mono" />
            </div>
            <Button className="mt-5" onClick={calcDiff}>计算</Button>
          </div>
          {errors.diff && <Badge variant="destructive">{errors.diff}</Badge>}
          {diffResult !== null && (
            <p className="text-lg font-semibold">
              相差 <span className="text-primary">{Math.abs(diffResult)}</span> 天
              {diffResult < 0 ? '（第一个日期在后）' : ''}
            </p>
          )}
        </CardContent>
      </Card>
      <Card>
        <CardHeader><CardTitle>日期加减</CardTitle></CardHeader>
        <CardContent className="space-y-3">
          <div className="flex items-center gap-3 flex-wrap">
            <div className="space-y-1">
              <Label>基准日期</Label>
              <Input type="date" value={addInput} onChange={e => setAddInput(e.target.value)} className="font-mono" />
            </div>
            <div className="space-y-1">
              <Label>加减天数（负数为减）</Label>
              <Input type="number" value={days} onChange={e => setDays(Number(e.target.value))} className="w-32 font-mono" />
            </div>
            <Button className="mt-5" onClick={calcAdd}>计算</Button>
          </div>
          {errors.add && <Badge variant="destructive">{errors.add}</Badge>}
          {addResult && (
            <p className="text-lg font-semibold">
              结果日期：<span className="text-primary font-mono">{addResult}</span>
            </p>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

// ── CSV 格式化 ─────────────────────────────────────────────────────────────────
export function CsvFormatterClient() {
  const tErr = useTranslations('docs.toolFunctions.errors')
  const [input, setInput] = useState('姓名,年龄,城市\n张三,25,北京\n李四,30,上海\n王五,28,广州')
  const [delimiter, setDelimiter] = useState(',')
  const [table, setTable] = useState<string[][] | null>(null)
  const [error, setError] = useState('')

  const parse = () => {
    try {
      setTable(parseCSV(input, delimiter || ','))
      setError('')
    } catch (e) {
      setError(translateToolError(e, tErr as never, '解析失败'))
      setTable(null)
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">CSV 格式化</h1>
        <p className="text-muted-foreground mt-2">将 CSV 数据解析为表格展示</p>
      </div>
      <Card>
        <CardHeader><CardTitle>CSV 输入</CardTitle></CardHeader>
        <CardContent className="space-y-3">
          <div className="flex items-center gap-3">
            <Label>分隔符</Label>
            <Input
              value={delimiter}
              onChange={e => setDelimiter(e.target.value.slice(0, 1))}
              className="w-16 font-mono"
              placeholder=","
            />
          </div>
          <Textarea
            value={input}
            onChange={e => setInput(e.target.value)}
            className="min-h-[150px] font-mono text-sm"
            placeholder="CSV 数据…"
          />
          <Button onClick={parse}>解析</Button>
          {error && <Badge variant="destructive">{error}</Badge>}
        </CardContent>
      </Card>
      {table && table.length > 0 && (
        <Card>
          <CardHeader><CardTitle>表格视图</CardTitle></CardHeader>
          <CardContent className="overflow-x-auto">
            <table className="w-full text-sm border-collapse">
              <thead>
                <tr className="border-b">
                  {table[0]!.map((cell, i) => (
                    <th key={i} className="text-left py-2 px-3 font-medium bg-muted/30">{cell}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {table.slice(1).map((row, ri) => (
                  <tr key={ri} className="border-b hover:bg-muted/20">
                    {row.map((cell, ci) => (
                      <td key={ci} className="py-2 px-3">{cell}</td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
            <p className="text-muted-foreground text-xs mt-2">{table.length - 1} 行数据，{table[0]!.length} 列</p>
          </CardContent>
        </Card>
      )}
    </div>
  )
}
