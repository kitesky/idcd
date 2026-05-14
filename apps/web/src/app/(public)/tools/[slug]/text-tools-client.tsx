"use client"

import { useState } from 'react'
import {
  Card, CardContent, CardHeader, CardTitle,
  Textarea, Button, Badge, Label, Input,
} from '@/components/ui'
import {
  countWords,
  sortLines,
  removeDuplicates,
  toCamelCase, toSnakeCase, toPascalCase, toKebabCase, toConstantCase,
  encodeHTML, decodeHTML,
  getTextFreqStats,
  parseMarkdown,
} from '@/lib/tool-functions'

// ── 字数统计 ────────────────────────────────────────────────────────────────
export function WordCounterClient() {
  const [text, setText] = useState('')
  const stats = countWords(text)

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">字数统计</h1>
        <p className="text-muted-foreground mt-2">统计文本的字数、字符数、行数和段落数</p>
      </div>
      <div className="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader><CardTitle>输入文本</CardTitle></CardHeader>
          <CardContent>
            <Textarea
              placeholder="在此粘贴或输入文本…"
              value={text}
              onChange={e => setText(e.target.value)}
              className="min-h-[300px] font-mono text-sm"
            />
          </CardContent>
        </Card>
        <Card>
          <CardHeader><CardTitle>统计结果</CardTitle></CardHeader>
          <CardContent className="space-y-3">
            {([
              ['总字符数', stats.chars],
              ['字符数（不含空白）', stats.charsNoSpace],
              ['单词数', stats.words],
              ['行数', stats.lines],
              ['段落数', stats.paragraphs],
              ['句子数', stats.sentences],
              ['中文字符数', stats.chineseChars],
            ] as [string, number][]).map(([label, value]) => (
              <div key={label} className="flex justify-between items-center py-2 border-b last:border-0">
                <span className="text-muted-foreground text-sm">{label}</span>
                <Badge variant="secondary">{value.toLocaleString()}</Badge>
              </div>
            ))}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}

// ── 行排序 ──────────────────────────────────────────────────────────────────
export function LineSortClient() {
  const [input, setInput] = useState('')
  const [output, setOutput] = useState('')
  const [reverse, setReverse] = useState(false)
  const [caseInsensitive, setCaseInsensitive] = useState(false)

  const handleSort = () => setOutput(sortLines(input, reverse, caseInsensitive))

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">行排序</h1>
        <p className="text-muted-foreground mt-2">对文本按行排序</p>
      </div>
      <div className="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader><CardTitle>输入</CardTitle></CardHeader>
          <CardContent className="space-y-4">
            <Textarea
              placeholder="每行一条数据…"
              value={input}
              onChange={e => setInput(e.target.value)}
              className="min-h-[300px] font-mono text-sm"
            />
            <div className="flex flex-wrap items-center gap-4">
              <label className="flex items-center gap-2 text-sm cursor-pointer">
                <input type="checkbox" checked={reverse} onChange={e => setReverse(e.target.checked)} className="rounded" />
                倒序
              </label>
              <label className="flex items-center gap-2 text-sm cursor-pointer">
                <input type="checkbox" checked={caseInsensitive} onChange={e => setCaseInsensitive(e.target.checked)} className="rounded" />
                忽略大小写
              </label>
              <Button onClick={handleSort}>排序</Button>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader><CardTitle>结果</CardTitle></CardHeader>
          <CardContent className="space-y-2">
            <Textarea value={output} readOnly className="min-h-[300px] font-mono text-sm bg-muted/50" placeholder="排序结果…" />
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

// ── 去重工具 ────────────────────────────────────────────────────────────────
export function DuplicateRemoverClient() {
  const [input, setInput] = useState('')
  const [caseInsensitive, setCaseInsensitive] = useState(false)

  const output = input ? removeDuplicates(input, caseInsensitive) : ''
  const removed = input.split('\n').length - output.split('\n').length

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">去重工具</h1>
        <p className="text-muted-foreground mt-2">删除文本中的重复行，保留唯一行</p>
      </div>
      <div className="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader><CardTitle>输入</CardTitle></CardHeader>
          <CardContent className="space-y-4">
            <Textarea
              placeholder="每行一条数据…"
              value={input}
              onChange={e => setInput(e.target.value)}
              className="min-h-[300px] font-mono text-sm"
            />
            <label className="flex items-center gap-2 text-sm cursor-pointer">
              <input type="checkbox" checked={caseInsensitive} onChange={e => setCaseInsensitive(e.target.checked)} className="rounded" />
              忽略大小写
            </label>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <div className="flex items-center gap-2">
              <CardTitle>结果</CardTitle>
              {input && removed > 0 && (
                <Badge variant="secondary">已删除 {removed} 行</Badge>
              )}
            </div>
          </CardHeader>
          <CardContent className="space-y-2">
            <Textarea value={output} readOnly className="min-h-[300px] font-mono text-sm bg-muted/50" placeholder="去重结果…" />
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

// ── 大小写转换 ───────────────────────────────────────────────────────────────
export function TextCaseClient() {
  const [input, setInput] = useState('')

  const results = input
    ? [
        ['camelCase', toCamelCase(input)],
        ['PascalCase', toPascalCase(input)],
        ['snake_case', toSnakeCase(input)],
        ['kebab-case', toKebabCase(input)],
        ['CONSTANT_CASE', toConstantCase(input)],
        ['UPPER CASE', input.toUpperCase()],
        ['lower case', input.toLowerCase()],
      ]
    : []

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">大小写转换</h1>
        <p className="text-muted-foreground mt-2">将文本转换为不同的命名格式</p>
      </div>
      <Card>
        <CardHeader><CardTitle>输入文本</CardTitle></CardHeader>
        <CardContent>
          <Input
            placeholder="如：hello world, myVariable, MY_CONST…"
            value={input}
            onChange={e => setInput(e.target.value)}
          />
        </CardContent>
      </Card>
      {results.length > 0 && (
        <Card>
          <CardHeader><CardTitle>转换结果</CardTitle></CardHeader>
          <CardContent className="space-y-2">
            {results.map(([label, value]) => (
              <div key={label} className="flex items-center justify-between gap-4 py-2 border-b last:border-0">
                <span className="text-muted-foreground text-sm w-36 shrink-0">{label}</span>
                <code className="flex-1 font-mono text-sm break-all">{value}</code>
                <Button variant="outline" size="sm" onClick={() => navigator.clipboard.writeText(value)}>
                  复制
                </Button>
              </div>
            ))}
          </CardContent>
        </Card>
      )}
    </div>
  )
}

// ── HTML 实体编码 ────────────────────────────────────────────────────────────
export function HtmlEncodeClient() {
  const [input, setInput] = useState('')
  const [mode, setMode] = useState<'encode' | 'decode'>('encode')

  const output = input
    ? (mode === 'encode' ? encodeHTML(input) : decodeHTML(input))
    : ''

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">HTML 实体编码</h1>
        <p className="text-muted-foreground mt-2">HTML 特殊字符实体编码与解码</p>
      </div>
      <div className="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle>输入</CardTitle>
              <div className="flex border rounded-lg p-1">
                {(['encode', 'decode'] as const).map(m => (
                  <button
                    key={m}
                    className={`px-3 py-1 text-sm rounded transition-colors ${mode === m ? 'bg-primary text-primary-foreground' : ''}`}
                    onClick={() => setMode(m)}
                  >
                    {m === 'encode' ? '编码' : '解码'}
                  </button>
                ))}
              </div>
            </div>
          </CardHeader>
          <CardContent>
            <Textarea
              placeholder={mode === 'encode' ? '<div class="example">Hello & World</div>' : '&lt;div&gt;&amp;lt;/div&gt;'}
              value={input}
              onChange={e => setInput(e.target.value)}
              className="min-h-[250px] font-mono text-sm"
            />
          </CardContent>
        </Card>
        <Card>
          <CardHeader><CardTitle>结果</CardTitle></CardHeader>
          <CardContent className="space-y-2">
            <Textarea value={output} readOnly className="min-h-[250px] font-mono text-sm bg-muted/50" placeholder="结果…" />
            {output && (
              <Button variant="outline" size="sm" onClick={() => navigator.clipboard.writeText(output)}>
                复制
              </Button>
            )}
          </CardContent>
        </Card>
      </div>
      <Card>
        <CardHeader><CardTitle>常用实体对照</CardTitle></CardHeader>
        <CardContent className="text-sm font-mono space-y-1">
          {[['<', '&lt;'], ['>', '&gt;'], ['&', '&amp;'], ['"', '&quot;'], ["'", '&#039;'], [' ', '&nbsp;']].map(([char, entity]) => (
            <div key={char} className="flex gap-8">
              <span className="w-8">{char}</span>
              <span className="text-muted-foreground">{entity}</span>
            </div>
          ))}
        </CardContent>
      </Card>
    </div>
  )
}

// ── HTML 转义（同 html-encode 但强调安全）────────────────────────────────────
export function EscapeHtmlClient() {
  const [input, setInput] = useState('')
  const output = input ? encodeHTML(input) : ''

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">HTML 转义</h1>
        <p className="text-muted-foreground mt-2">将 HTML 内容转义为安全的文本，防止 XSS 注入</p>
      </div>
      <div className="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader><CardTitle>原始 HTML</CardTitle></CardHeader>
          <CardContent>
            <Textarea
              placeholder={'<script>alert("xss")</script>\n<img src=x onerror=alert(1)>'}
              value={input}
              onChange={e => setInput(e.target.value)}
              className="min-h-[250px] font-mono text-sm"
            />
          </CardContent>
        </Card>
        <Card>
          <CardHeader><CardTitle>转义后（安全文本）</CardTitle></CardHeader>
          <CardContent className="space-y-2">
            <Textarea value={output} readOnly className="min-h-[250px] font-mono text-sm bg-muted/50" placeholder="转义结果…" />
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

// ── 文本统计（词频）──────────────────────────────────────────────────────────
export function TextStatsClient() {
  const [text, setText] = useState('')
  const stats = text.trim() ? getTextFreqStats(text) : null

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">文本统计</h1>
        <p className="text-muted-foreground mt-2">词频统计与文本分析</p>
      </div>
      <Card>
        <CardHeader><CardTitle>输入文本</CardTitle></CardHeader>
        <CardContent>
          <Textarea
            placeholder="在此输入文本进行分析…"
            value={text}
            onChange={e => setText(e.target.value)}
            className="min-h-[200px] font-mono text-sm"
          />
        </CardContent>
      </Card>
      {stats && (
        <div className="grid gap-6 lg:grid-cols-2">
          <Card>
            <CardHeader><CardTitle>总览</CardTitle></CardHeader>
            <CardContent className="space-y-2">
              {[
                ['总词数', stats.totalWords],
                ['唯一词数', stats.uniqueWords],
              ].map(([label, value]) => (
                <div key={String(label)} className="flex justify-between py-2 border-b last:border-0">
                  <span className="text-muted-foreground text-sm">{label}</span>
                  <Badge variant="secondary">{String(value)}</Badge>
                </div>
              ))}
            </CardContent>
          </Card>
          <Card>
            <CardHeader><CardTitle>Top 20 高频词</CardTitle></CardHeader>
            <CardContent>
              <div className="space-y-1">
                {stats.topWords.map(([word, count]) => (
                  <div key={word} className="flex items-center gap-2 text-sm">
                    <span className="font-mono w-24 truncate">{word}</span>
                    <div className="flex-1 bg-muted rounded-full h-2">
                      <div
                        className="bg-primary h-2 rounded-full"
                        style={{ width: `${(count / (stats.topWords[0]?.[1] ?? 1)) * 100}%` }}
                      />
                    </div>
                    <span className="text-muted-foreground w-8 text-right">{count}</span>
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>
        </div>
      )}
    </div>
  )
}

// ── 文本对比 ────────────────────────────────────────────────────────────────
type DiffLine = { type: 'same' | 'added' | 'removed'; line: string }

function computeDiff(a: string, b: string): DiffLine[] {
  const lines1 = a.split('\n')
  const lines2 = b.split('\n')
  const result: DiffLine[] = []
  const max = Math.max(lines1.length, lines2.length)

  for (let i = 0; i < max; i++) {
    if (i < lines1.length && i < lines2.length) {
      if (lines1[i] === lines2[i]) {
        result.push({ type: 'same', line: lines1[i] })
      } else {
        result.push({ type: 'removed', line: lines1[i] })
        result.push({ type: 'added', line: lines2[i] })
      }
    } else if (i < lines1.length) {
      result.push({ type: 'removed', line: lines1[i] })
    } else {
      result.push({ type: 'added', line: lines2[i] })
    }
  }

  return result
}

export function DiffClient() {
  const [text1, setText1] = useState('')
  const [text2, setText2] = useState('')
  const [diffResult, setDiffResult] = useState<DiffLine[] | null>(null)

  const handleDiff = () => setDiffResult(computeDiff(text1, text2))

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">文本对比</h1>
        <p className="text-muted-foreground mt-2">逐行对比两段文本的差异</p>
      </div>
      <div className="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader><CardTitle>原始文本</CardTitle></CardHeader>
          <CardContent>
            <Textarea value={text1} onChange={e => setText1(e.target.value)} className="min-h-[250px] font-mono text-sm" placeholder="输入原始文本…" />
          </CardContent>
        </Card>
        <Card>
          <CardHeader><CardTitle>修改后文本</CardTitle></CardHeader>
          <CardContent>
            <Textarea value={text2} onChange={e => setText2(e.target.value)} className="min-h-[250px] font-mono text-sm" placeholder="输入修改后文本…" />
          </CardContent>
        </Card>
      </div>
      <Button onClick={handleDiff}>对比</Button>
      {diffResult && (
        <Card>
          <CardHeader>
            <div className="flex items-center gap-4">
              <CardTitle>差异结果</CardTitle>
              <Badge variant="destructive">{diffResult.filter(d => d.type === 'removed').length} 删除</Badge>
              <Badge className="bg-green-600 text-white">{diffResult.filter(d => d.type === 'added').length} 新增</Badge>
            </div>
          </CardHeader>
          <CardContent>
            <div className="font-mono text-sm rounded border overflow-x-auto">
              {diffResult.map((d, i) => (
                <div
                  key={i}
                  className={`px-4 py-0.5 ${
                    d.type === 'added' ? 'bg-green-950/40 text-green-400' :
                    d.type === 'removed' ? 'bg-red-950/40 text-red-400' :
                    'text-muted-foreground'
                  }`}
                >
                  <span className="select-none mr-2 opacity-50">
                    {d.type === 'added' ? '+' : d.type === 'removed' ? '-' : ' '}
                  </span>
                  {d.line || <span className="opacity-30">（空行）</span>}
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  )
}

// ── Markdown 预览 ────────────────────────────────────────────────────────────
export function MarkdownClient() {
  const [input, setInput] = useState(`# 标题一

## 标题二

这是**粗体**和*斜体*文本。

- 列表项 1
- 列表项 2

> 引用文字

\`\`\`
代码块示例
\`\`\`

[链接示例](https://idcd.com)
`)

  const html = parseMarkdown(input)

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">Markdown 预览</h1>
        <p className="text-muted-foreground mt-2">Markdown 编辑与实时预览</p>
      </div>
      <div className="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader><CardTitle>Markdown 输入</CardTitle></CardHeader>
          <CardContent>
            <Textarea
              value={input}
              onChange={e => setInput(e.target.value)}
              className="min-h-[400px] font-mono text-sm"
              placeholder="输入 Markdown 文本…"
            />
          </CardContent>
        </Card>
        <Card>
          <CardHeader><CardTitle>预览</CardTitle></CardHeader>
          <CardContent>
            <div
              className="prose prose-invert max-w-none min-h-[400px] text-sm [&_h1]:text-2xl [&_h1]:font-bold [&_h1]:mb-4 [&_h2]:text-xl [&_h2]:font-bold [&_h2]:mb-3 [&_h3]:text-lg [&_h3]:font-bold [&_h3]:mb-2 [&_p]:mb-3 [&_ul]:pl-4 [&_li]:list-disc [&_li]:ml-2 [&_li]:mb-1 [&_blockquote]:border-l-4 [&_blockquote]:border-muted-foreground [&_blockquote]:pl-4 [&_blockquote]:italic [&_code]:bg-muted [&_code]:px-1 [&_code]:rounded [&_pre]:bg-muted [&_pre]:p-4 [&_pre]:rounded [&_pre]:overflow-x-auto [&_a]:text-primary [&_a]:underline [&_hr]:border-muted"
              dangerouslySetInnerHTML={{ __html: html }}
            />
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
