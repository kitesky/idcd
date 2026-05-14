"use client"

import { useState, useMemo } from "react"
import { Card, CardContent, CardHeader, CardTitle, Input, Textarea, Badge } from "@/components/ui"
import { encodeHTML } from "@/lib/tool-functions"

interface MatchResult {
  match: string
  index: number
  groups: string[]
}

export default function RegexTesterPage() {
  const [pattern, setPattern] = useState('')
  const [flags, setFlags] = useState('g')
  const [testText, setTestText] = useState('')
  const [error, setError] = useState('')

  // Real-time regex testing
  const results = useMemo(() => {
    if (!pattern || !testText) {
      return { matches: [], highlightedText: testText }
    }

    try {
      const regex = new RegExp(pattern, flags)
      const matches: MatchResult[] = []
      let highlightedText = testText
      let match
      let offset = 0

      // Reset regex lastIndex for global matching
      regex.lastIndex = 0

      if (flags.includes('g')) {
        while ((match = regex.exec(testText)) !== null) {
          matches.push({
            match: match[0],
            index: match.index,
            groups: match.slice(1)
          })

          // Prevent infinite loop
          if (match[0].length === 0) {
            regex.lastIndex++
          }
        }
      } else {
        match = regex.exec(testText)
        if (match) {
          matches.push({
            match: match[0],
            index: match.index,
            groups: match.slice(1)
          })
        }
      }

      // Build highlighted text by iterating matches in order, escaping each segment.
      if (matches.length > 0) {
        const sortedMatches = [...matches].sort((a, b) => a.index - b.index)
        let result = ''
        let lastIndex = 0
        sortedMatches.forEach(m => {
          result += encodeHTML(testText.slice(lastIndex, m.index))
          result += `<mark class="bg-primary/20 text-primary-foreground rounded px-1">${encodeHTML(m.match)}</mark>`
          lastIndex = m.index + m.match.length
        })
        result += encodeHTML(testText.slice(lastIndex))
        highlightedText = result
      }

      setError('')
      return { matches, highlightedText }
    } catch (err) {
      setError(err instanceof Error ? err.message : '正则表达式错误')
      return { matches: [], highlightedText: testText }
    }
  }, [pattern, flags, testText])

  const flagOptions = [
    { key: 'g', label: 'g', description: '全局匹配' },
    { key: 'i', label: 'i', description: '忽略大小写' },
    { key: 'm', label: 'm', description: '多行模式' },
    { key: 's', label: 's', description: 'dotAll 模式（. 匹配换行符）' }
  ]

  const toggleFlag = (flag: string) => {
    setFlags(prev =>
      prev.includes(flag)
        ? prev.replace(flag, '')
        : prev + flag
    )
  }

  const commonPatterns = [
    { name: '邮箱地址', pattern: '[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}' },
    { name: '手机号码', pattern: '1[3-9]\\d{9}' },
    { name: 'IPv4 地址', pattern: '(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)' },
    { name: 'URL', pattern: 'https?://[^\\s/$.?#].[^\\s]*' },
    { name: '身份证号', pattern: '[1-9]\\d{5}(19|20)\\d{2}((0[1-9])|(1[0-2]))(([0-2][1-9])|10|20|30|31)\\d{3}[0-9Xx]' },
    { name: '日期格式', pattern: '\\d{4}-\\d{2}-\\d{2}' }
  ]

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">正则表达式测试工具</h1>
        <p className="text-muted-foreground mt-2">
          在线正则表达式测试器，实时匹配高亮显示，支持捕获组查看
        </p>
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        {/* Pattern Input */}
        <Card>
          <CardHeader>
            <CardTitle>正则表达式</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <label className="text-sm font-medium">模式</label>
              <Input
                placeholder="输入正则表达式..."
                value={pattern}
                onChange={(e) => setPattern(e.target.value)}
                className="font-mono"
              />
            </div>

            <div className="space-y-2">
              <label className="text-sm font-medium">标志</label>
              <div className="flex flex-wrap gap-2">
                {flagOptions.map(option => (
                  <button
                    key={option.key}
                    onClick={() => toggleFlag(option.key)}
                    className={`px-3 py-1 text-sm rounded border transition-colors ${
                      flags.includes(option.key)
                        ? 'bg-primary text-primary-foreground border-primary'
                        : 'bg-background border-input hover:bg-muted'
                    }`}
                    title={option.description}
                  >
                    {option.label}
                  </button>
                ))}
              </div>
              <p className="text-xs text-muted-foreground">
                当前标志：/{pattern}/{flags}
              </p>
            </div>

            {error && (
              <Badge variant="destructive">
                错误：{error}
              </Badge>
            )}

            <div className="space-y-2">
              <label className="text-sm font-medium">常用模式</label>
              <div className="space-y-1">
                {commonPatterns.map((item, index) => (
                  <button
                    key={index}
                    onClick={() => setPattern(item.pattern)}
                    className="block w-full text-left px-2 py-1 text-xs bg-muted/50 hover:bg-muted rounded"
                  >
                    <span className="font-medium">{item.name}</span>: {item.pattern}
                  </button>
                ))}
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Test Text Input */}
        <Card>
          <CardHeader>
            <CardTitle>测试文本</CardTitle>
          </CardHeader>
          <CardContent>
            <Textarea
              placeholder="在此输入要测试的文本..."
              value={testText}
              onChange={(e) => setTestText(e.target.value)}
              className="min-h-[300px] font-mono text-sm"
            />
          </CardContent>
        </Card>
      </div>

      {/* Results */}
      {(results.matches.length > 0 || (pattern && testText)) && (
        <>
          {/* Match Results */}
          <Card>
            <CardHeader>
              <CardTitle>
                匹配结果
                <Badge variant="outline" className="ml-2">
                  {results.matches.length} 个匹配
                </Badge>
              </CardTitle>
            </CardHeader>
            <CardContent>
              {results.matches.length > 0 ? (
                <div className="space-y-2">
                  {results.matches.map((match, index) => (
                    <div key={index} className="p-3 bg-muted/50 rounded border">
                      <div className="flex items-center gap-4 text-sm">
                        <span className="font-medium">匹配 {index + 1}:</span>
                        <code className="bg-background px-2 py-1 rounded border">
                          {match.match}
                        </code>
                        <span className="text-muted-foreground">
                          位置: {match.index}-{match.index + match.match.length - 1}
                        </span>
                      </div>
                      {match.groups.length > 0 && (
                        <div className="mt-2 pt-2 border-t text-sm">
                          <span className="font-medium text-muted-foreground">捕获组:</span>
                          <div className="flex flex-wrap gap-2 mt-1">
                            {match.groups.map((group, groupIndex) => (
                              <code key={groupIndex} className="bg-background px-2 py-1 rounded border text-xs">
                                ${groupIndex + 1}: {group || '(空)'}
                              </code>
                            ))}
                          </div>
                        </div>
                      )}
                    </div>
                  ))}
                </div>
              ) : (
                <p className="text-muted-foreground">无匹配结果</p>
              )}
            </CardContent>
          </Card>

          {/* Highlighted Text */}
          <Card>
            <CardHeader>
              <CardTitle>高亮显示</CardTitle>
            </CardHeader>
            <CardContent>
              <div
                className="font-mono text-sm bg-muted/50 p-4 rounded border min-h-[100px] whitespace-pre-wrap"
                dangerouslySetInnerHTML={{ __html: results.highlightedText }}
              />
            </CardContent>
          </Card>
        </>
      )}

      <Card>
        <CardHeader>
          <CardTitle>使用说明</CardTitle>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground space-y-2">
          <p>• <strong>实时匹配</strong>：输入正则表达式和测试文本后实时显示匹配结果</p>
          <p>• <strong>标志说明</strong>：g=全局匹配，i=忽略大小写，m=多行模式，s=dotAll模式</p>
          <p>• <strong>捕获组</strong>：使用括号 () 创建捕获组，查看匹配的子字符串</p>
          <p>• <strong>高亮显示</strong>：匹配的文本会被高亮标记</p>
        </CardContent>
      </Card>
    </div>
  )
}