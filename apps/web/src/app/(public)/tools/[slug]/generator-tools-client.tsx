"use client"

import { useState, useRef } from 'react'
import { useTranslations } from 'next-intl'
import {
  Card, CardContent, CardHeader, CardTitle,
  Button, Badge, Label, Input, Textarea,
} from '@/components/ui'
import {
  generatePassword,
  generateUUID,
  generateLorem,
  calculateChmod, parseChmod, chmodToSymbolic,
  sortJSON,
  hexToRgb, rgbToHex, rgbToHsl,
} from '@/lib/tool-functions'
import type { ChmodPerms } from '@/lib/tool-functions'
import { translateToolError } from '@/lib/tool-error'

// ── 密码生成器 ───────────────────────────────────────────────────────────────
export function PasswordGenClient() {
  const [length, setLength] = useState(20)
  const [opts, setOpts] = useState({ uppercase: true, lowercase: true, numbers: true, symbols: true })
  const [passwords, setPasswords] = useState<string[]>([])
  const [count, setCount] = useState(5)

  const generate = () => {
    const list: string[] = []
    for (let i = 0; i < count; i++) list.push(generatePassword(length, opts))
    setPasswords(list)
  }

  const toggle = (key: keyof typeof opts) =>
    setOpts(prev => ({ ...prev, [key]: !prev[key] }))

  const strength = () => {
    const score =
      (opts.uppercase ? 26 : 0) +
      (opts.lowercase ? 26 : 0) +
      (opts.numbers ? 10 : 0) +
      (opts.symbols ? 30 : 0)
    const entropy = Math.round(length * Math.log2(score || 1))
    if (entropy >= 80) return { label: '非常强', variant: 'default' as const }
    if (entropy >= 60) return { label: '强', variant: 'default' as const }
    if (entropy >= 40) return { label: '中等', variant: 'secondary' as const }
    return { label: '弱', variant: 'destructive' as const }
  }

  const s = strength()

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">密码生成器</h1>
        <p className="text-muted-foreground mt-2">生成高强度随机密码</p>
      </div>
      <Card>
        <CardHeader><CardTitle>生成设置</CardTitle></CardHeader>
        <CardContent className="space-y-5">
          <div className="space-y-2">
            <div className="flex justify-between">
              <Label>密码长度：{length}</Label>
              <Badge variant={s.variant}>{s.label}</Badge>
            </div>
            <input
              type="range" min={8} max={64} value={length}
              onChange={e => setLength(Number(e.target.value))}
              className="w-full"
            />
          </div>
          <div className="flex flex-wrap gap-4">
            {([
              ['uppercase', '大写字母 (A-Z)'],
              ['lowercase', '小写字母 (a-z)'],
              ['numbers', '数字 (0-9)'],
              ['symbols', '特殊符号'],
            ] as const).map(([key, label]) => (
              <label key={key} className="flex items-center gap-2 text-sm cursor-pointer">
                <input type="checkbox" checked={opts[key]} onChange={() => toggle(key)} className="rounded" />
                {label}
              </label>
            ))}
          </div>
          <div className="flex items-center gap-3">
            <Label>数量</Label>
            <Input
              type="number" min={1} max={20} value={count}
              onChange={e => setCount(Number(e.target.value))}
              className="w-24"
            />
            <Button onClick={generate}>生成</Button>
          </div>
        </CardContent>
      </Card>
      {passwords.length > 0 && (
        <Card>
          <CardHeader><CardTitle>生成结果</CardTitle></CardHeader>
          <CardContent className="space-y-2">
            {passwords.map((pwd, i) => (
              <div key={i} className="flex items-center gap-3 py-2 border-b last:border-0">
                <code className="flex-1 font-mono text-sm break-all">{pwd}</code>
                <Button variant="outline" size="sm" onClick={() => navigator.clipboard.writeText(pwd)}>复制</Button>
              </div>
            ))}
          </CardContent>
        </Card>
      )}
    </div>
  )
}

// ── UUID 生成器 ───────────────────────────────────────────────────────────────
export function UuidGenClient() {
  const [uuids, setUuids] = useState<string[]>([])
  const [count, setCount] = useState(5)
  const [upper, setUpper] = useState(false)
  const [noDash, setNoDash] = useState(false)

  const generate = () => {
    const list: string[] = []
    for (let i = 0; i < count; i++) {
      let u = generateUUID()
      if (noDash) u = u.replace(/-/g, '')
      if (upper) u = u.toUpperCase()
      list.push(u)
    }
    setUuids(list)
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">UUID 生成器</h1>
        <p className="text-muted-foreground mt-2">生成 UUID v4 唯一标识符</p>
      </div>
      <Card>
        <CardHeader><CardTitle>设置</CardTitle></CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center gap-4 flex-wrap">
            <div className="flex items-center gap-2">
              <Label>数量</Label>
              <Input type="number" min={1} max={50} value={count} onChange={e => setCount(Number(e.target.value))} className="w-24" />
            </div>
            <label className="flex items-center gap-2 text-sm cursor-pointer">
              <input type="checkbox" checked={upper} onChange={e => setUpper(e.target.checked)} className="rounded" />
              大写
            </label>
            <label className="flex items-center gap-2 text-sm cursor-pointer">
              <input type="checkbox" checked={noDash} onChange={e => setNoDash(e.target.checked)} className="rounded" />
              去除连字符
            </label>
            <Button onClick={generate}>生成</Button>
          </div>
        </CardContent>
      </Card>
      {uuids.length > 0 && (
        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle>结果</CardTitle>
              <Button variant="outline" size="sm" onClick={() => navigator.clipboard.writeText(uuids.join('\n'))}>
                全部复制
              </Button>
            </div>
          </CardHeader>
          <CardContent className="space-y-1">
            {uuids.map((uuid, i) => (
              <div key={i} className="flex items-center gap-3 py-1.5 border-b last:border-0">
                <code className="flex-1 font-mono text-sm">{uuid}</code>
                <Button variant="ghost" size="sm" onClick={() => navigator.clipboard.writeText(uuid)}>复制</Button>
              </div>
            ))}
          </CardContent>
        </Card>
      )}
    </div>
  )
}

// ── Lorem Ipsum ───────────────────────────────────────────────────────────────
export function LoremClient() {
  const [paragraphs, setParagraphs] = useState(3)
  const [sentences, setSentences] = useState(4)
  const [result, setResult] = useState('')

  const generate = () => setResult(generateLorem(paragraphs, sentences))

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">Lorem Ipsum</h1>
        <p className="text-muted-foreground mt-2">生成 Lorem Ipsum 占位文本</p>
      </div>
      <Card>
        <CardHeader><CardTitle>设置</CardTitle></CardHeader>
        <CardContent className="flex items-center gap-4 flex-wrap">
          <div className="flex items-center gap-2">
            <Label>段落数</Label>
            <Input type="number" min={1} max={20} value={paragraphs} onChange={e => setParagraphs(Number(e.target.value))} className="w-24" />
          </div>
          <div className="flex items-center gap-2">
            <Label>每段句数</Label>
            <Input type="number" min={1} max={10} value={sentences} onChange={e => setSentences(Number(e.target.value))} className="w-24" />
          </div>
          <Button onClick={generate}>生成</Button>
        </CardContent>
      </Card>
      {result && (
        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle>生成结果</CardTitle>
              <Button variant="outline" size="sm" onClick={() => navigator.clipboard.writeText(result)}>复制</Button>
            </div>
          </CardHeader>
          <CardContent>
            {result.split('\n\n').map((para, i) => (
              <p key={i} className="text-sm text-muted-foreground mb-3 last:mb-0">{para}</p>
            ))}
          </CardContent>
        </Card>
      )}
    </div>
  )
}

// ── chmod 计算器 ──────────────────────────────────────────────────────────────
export function ChmodCalcClient() {
  const defaultPerms: ChmodPerms = {
    owner: { r: true, w: true, x: false },
    group: { r: true, w: false, x: false },
    others: { r: true, w: false, x: false },
  }

  const [perms, setPerms] = useState<ChmodPerms>(defaultPerms)
  const [octalInput, setOctalInput] = useState('')

  const octal = calculateChmod(perms)
  const symbolic = chmodToSymbolic(octal)

  const toggle = (who: keyof ChmodPerms, bit: 'r' | 'w' | 'x') => {
    setPerms(prev => ({
      ...prev,
      [who]: { ...prev[who], [bit]: !prev[who][bit] },
    }))
  }

  const handleOctalInput = (v: string) => {
    setOctalInput(v)
    if (/^[0-7]{3}$/.test(v)) {
      try { setPerms(parseChmod(v)) } catch {}
    }
  }

  const roles: { key: keyof ChmodPerms; label: string }[] = [
    { key: 'owner', label: '所有者' },
    { key: 'group', label: '所属组' },
    { key: 'others', label: '其他人' },
  ]

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">chmod 计算器</h1>
        <p className="text-muted-foreground mt-2">可视化计算 Unix/Linux 文件权限</p>
      </div>
      <Card>
        <CardHeader><CardTitle>权限矩阵</CardTitle></CardHeader>
        <CardContent>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b">
                  <th className="text-left py-2 pr-4 font-medium text-muted-foreground">角色</th>
                  <th className="text-center py-2 px-4 font-medium">读 (r)</th>
                  <th className="text-center py-2 px-4 font-medium">写 (w)</th>
                  <th className="text-center py-2 px-4 font-medium">执行 (x)</th>
                  <th className="text-center py-2 px-4 font-medium text-muted-foreground">八进制</th>
                </tr>
              </thead>
              <tbody>
                {roles.map(({ key, label }) => (
                  <tr key={key} className="border-b last:border-0">
                    <td className="py-3 pr-4 font-medium">{label}</td>
                    {(['r', 'w', 'x'] as const).map(bit => (
                      <td key={bit} className="text-center py-3 px-4">
                        <input
                          type="checkbox"
                          checked={perms[key][bit]}
                          onChange={() => toggle(key, bit)}
                          className="w-4 h-4 cursor-pointer"
                        />
                      </td>
                    ))}
                    <td className="text-center py-3 px-4 font-mono text-muted-foreground">
                      {(perms[key].r ? 4 : 0) + (perms[key].w ? 2 : 0) + (perms[key].x ? 1 : 0)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>
      <div className="grid gap-4 lg:grid-cols-3">
        <Card>
          <CardContent className="pt-6">
            <p className="text-sm text-muted-foreground mb-1">chmod 命令</p>
            <code className="text-2xl font-mono font-bold">chmod {octal}</code>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="pt-6">
            <p className="text-sm text-muted-foreground mb-1">符号表示</p>
            <code className="text-2xl font-mono font-bold">{symbolic}</code>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="pt-6">
            <p className="text-sm text-muted-foreground mb-1">八进制输入</p>
            <Input
              placeholder="644"
              value={octalInput}
              onChange={e => handleOctalInput(e.target.value)}
              className="font-mono text-lg"
              maxLength={3}
            />
          </CardContent>
        </Card>
      </div>
      <Card>
        <CardHeader><CardTitle>常用权限参考</CardTitle></CardHeader>
        <CardContent className="text-sm space-y-1">
          {[
            ['644', 'rw-r--r--', '普通文件（owner 可写，其他只读）'],
            ['755', 'rwxr-xr-x', '可执行文件/目录（owner 全权，其他执行）'],
            ['600', 'rw-------', '私有文件（仅 owner 读写）'],
            ['777', 'rwxrwxrwx', '全部可读写执行（不推荐用于生产）'],
            ['400', 'r--------', '只读文件（密钥文件常用）'],
          ].map(([oct, sym, desc]) => (
            <div key={oct} className="flex gap-3 py-1.5 border-b last:border-0">
              <code className="w-12 font-mono">{oct}</code>
              <code className="w-28 font-mono text-muted-foreground">{sym}</code>
              <span className="text-muted-foreground">{desc}</span>
            </div>
          ))}
        </CardContent>
      </Card>
    </div>
  )
}

// ── JSON 键排序 ───────────────────────────────────────────────────────────────
export function SortJsonClient() {
  const tErr = useTranslations('docs.toolFunctions.errors')
  const [input, setInput] = useState('')
  const [output, setOutput] = useState('')
  const [error, setError] = useState('')
  const [indent, setIndent] = useState(2)

  const sort = () => {
    try {
      const obj = JSON.parse(input)
      setOutput(JSON.stringify(sortJSON(obj), null, indent))
      setError('')
    } catch (e) {
      setError(translateToolError(e, tErr as never, 'JSON 解析失败'))
      setOutput('')
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">JSON 键排序</h1>
        <p className="text-muted-foreground mt-2">将 JSON 对象的键按字母顺序排序</p>
      </div>
      <div className="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader><CardTitle>输入 JSON</CardTitle></CardHeader>
          <CardContent className="space-y-3">
            <Textarea value={input} onChange={e => setInput(e.target.value)} className="min-h-[300px] font-mono text-sm" placeholder='{"z": 1, "a": 2, "m": {"x": 3, "b": 4}}' />
            <div className="flex items-center gap-3">
              <Label>缩进</Label>
              <Input type="number" min={0} max={8} value={indent} onChange={e => setIndent(Number(e.target.value))} className="w-20" />
              <Button onClick={sort}>排序</Button>
            </div>
            {error && <Badge variant="destructive">{error}</Badge>}
          </CardContent>
        </Card>
        <Card>
          <CardHeader><CardTitle>排序后</CardTitle></CardHeader>
          <CardContent className="space-y-2">
            <Textarea value={output} readOnly className="min-h-[300px] font-mono text-sm bg-muted/50" />
            {output && <Button variant="outline" size="sm" onClick={() => navigator.clipboard.writeText(output)}>复制</Button>}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}

// ── 颜色格式转换 ──────────────────────────────────────────────────────────────
export function ColorPickerClient() {
  const [hex, setHex] = useState('#3b82f6')
  const [error, setError] = useState('')

  const rgb = hexToRgb(hex)
  const hsl = rgb ? rgbToHsl(rgb.r, rgb.g, rgb.b) : null

  const handleHexChange = (v: string) => {
    setHex(v)
    setError('')
    if (v && !hexToRgb(v)) setError('无效的十六进制颜色')
  }

  const handleRgbChange = (r: number, g: number, b: number) => {
    setError('')
    setHex(rgbToHex(r, g, b))
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">颜色格式转换</h1>
        <p className="text-muted-foreground mt-2">HEX / RGB / HSL 颜色格式互转</p>
      </div>
      <Card>
        <CardContent className="pt-6">
          <div className="flex items-center gap-4 flex-wrap">
            <div className="flex items-center gap-3">
              <input type="color" value={hex.length === 7 ? hex : '#000000'} onChange={e => handleHexChange(e.target.value)} className="w-12 h-10 rounded cursor-pointer border-0 bg-transparent" />
              <div
                className="w-20 h-10 rounded border"
                style={{ background: rgb ? `rgb(${rgb.r},${rgb.g},${rgb.b})` : 'transparent' }}
              />
            </div>
          </div>
        </CardContent>
      </Card>
      <div className="grid gap-4 lg:grid-cols-3">
        <Card>
          <CardHeader><CardTitle>HEX</CardTitle></CardHeader>
          <CardContent className="space-y-2">
            <Input
              value={hex}
              onChange={e => handleHexChange(e.target.value)}
              className="font-mono"
              placeholder="#3b82f6"
            />
            {error && <p className="text-sm text-destructive">{error}</p>}
            <Button variant="outline" size="sm" onClick={() => navigator.clipboard.writeText(hex)}>复制</Button>
          </CardContent>
        </Card>
        <Card>
          <CardHeader><CardTitle>RGB</CardTitle></CardHeader>
          <CardContent className="space-y-2">
            {rgb ? (
              <>
                {(['r', 'g', 'b'] as const).map(channel => (
                  <div key={channel} className="flex items-center gap-2">
                    <Label className="w-4">{channel.toUpperCase()}</Label>
                    <Input
                      type="number" min={0} max={255}
                      value={rgb[channel]}
                      onChange={e => {
                        const val = Math.max(0, Math.min(255, Number(e.target.value)))
                        handleRgbChange(
                          channel === 'r' ? val : rgb.r,
                          channel === 'g' ? val : rgb.g,
                          channel === 'b' ? val : rgb.b,
                        )
                      }}
                    />
                  </div>
                ))}
                <Button variant="outline" size="sm" onClick={() => navigator.clipboard.writeText(`rgb(${rgb.r}, ${rgb.g}, ${rgb.b})`)}>复制</Button>
              </>
            ) : <p className="text-muted-foreground text-sm">输入有效的 HEX 颜色</p>}
          </CardContent>
        </Card>
        <Card>
          <CardHeader><CardTitle>HSL</CardTitle></CardHeader>
          <CardContent className="space-y-2">
            {hsl ? (
              <>
                {([['H', hsl.h, '°'], ['S', hsl.s, '%'], ['L', hsl.l, '%']] as [string, number, string][]).map(([label, val, unit]) => (
                  <div key={label} className="flex justify-between text-sm py-1">
                    <span className="text-muted-foreground">{label}</span>
                    <code className="font-mono">{val}{unit}</code>
                  </div>
                ))}
                <Button variant="outline" size="sm" onClick={() => navigator.clipboard.writeText(`hsl(${hsl.h}, ${hsl.s}%, ${hsl.l}%)`)}>复制</Button>
              </>
            ) : <p className="text-muted-foreground text-sm">输入有效的 HEX 颜色</p>}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}

// ── 图片 Base64 ────────────────────────────────────────────────────────────────
export function ImageBase64Client() {
  const [result, setResult] = useState('')
  const [fileName, setFileName] = useState('')
  const [fileSize, setFileSize] = useState(0)
  const inputRef = useRef<HTMLInputElement>(null)

  const handleFile = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    setFileName(file.name)
    setFileSize(file.size)

    const reader = new FileReader()
    reader.onload = ev => setResult(ev.target?.result as string ?? '')
    reader.readAsDataURL(file)
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">图片 Base64</h1>
        <p className="text-muted-foreground mt-2">将图片转换为 Base64 Data URL</p>
      </div>
      <Card>
        <CardHeader><CardTitle>选择图片</CardTitle></CardHeader>
        <CardContent className="space-y-4">
          <input ref={inputRef} type="file" accept="image/*" onChange={handleFile} className="hidden" />
          <Button onClick={() => inputRef.current?.click()}>选择图片文件</Button>
          {fileName && (
            <p className="text-sm text-muted-foreground">
              {fileName} ({(fileSize / 1024).toFixed(1)} KB)
            </p>
          )}
        </CardContent>
      </Card>
      {result && (
        <div className="space-y-4">
          <Card>
            <CardHeader><CardTitle>预览</CardTitle></CardHeader>
            <CardContent>
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img src={result} alt="预览" className="max-h-64 rounded border" />
            </CardContent>
          </Card>
          <Card>
            <CardHeader>
              <div className="flex items-center justify-between">
                <CardTitle>Base64 Data URL</CardTitle>
                <Button variant="outline" size="sm" onClick={() => navigator.clipboard.writeText(result)}>
                  复制
                </Button>
              </div>
            </CardHeader>
            <CardContent>
              <textarea
                readOnly
                value={result}
                className="w-full h-32 font-mono text-xs bg-muted/50 border rounded p-3 resize-none"
              />
              <p className="text-sm text-muted-foreground mt-2">
                Base64 大小：{(result.length / 1024).toFixed(1)} KB（原始 {(fileSize / 1024).toFixed(1)} KB，约增加 33%）
              </p>
            </CardContent>
          </Card>
        </div>
      )}
    </div>
  )
}
