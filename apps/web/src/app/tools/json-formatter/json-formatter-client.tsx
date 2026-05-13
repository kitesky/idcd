"use client"

import { useState } from "react"
import { Card, CardContent, CardHeader, CardTitle, Textarea, Button, Badge } from "@idcd/ui"

export default function JsonFormatterClient() {
  const [input, setInput] = useState('')
  const [output, setOutput] = useState('')
  const [error, setError] = useState('')

  const formatJson = () => {
    try {
      const parsed = JSON.parse(input)
      const formatted = JSON.stringify(parsed, null, 2)
      setOutput(formatted)
      setError('')
    } catch (err) {
      setError(err instanceof Error ? err.message : '无效的 JSON 格式')
      setOutput('')
    }
  }

  const minifyJson = () => {
    try {
      const parsed = JSON.parse(input)
      const minified = JSON.stringify(parsed)
      setOutput(minified)
      setError('')
    } catch (err) {
      setError(err instanceof Error ? err.message : '无效的 JSON 格式')
      setOutput('')
    }
  }

  const handleInputChange = (value: string) => {
    setInput(value)
    // Clear previous error when input changes
    if (error) setError('')
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">JSON 格式化工具</h1>
        <p className="text-muted-foreground mt-2">
          在线 JSON 格式化、美化、压缩工具，支持语法检查
        </p>
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        {/* Input section */}
        <Card>
          <CardHeader>
            <CardTitle>输入 JSON</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <Textarea
              placeholder="在此粘贴或输入 JSON 数据..."
              value={input}
              onChange={(e) => handleInputChange(e.target.value)}
              className="min-h-[300px] font-mono text-sm"
            />
            <div className="flex gap-2">
              <Button onClick={formatJson}>格式化</Button>
              <Button onClick={minifyJson} variant="outline">压缩</Button>
            </div>
          </CardContent>
        </Card>

        {/* Output section */}
        <Card>
          <CardHeader>
            <CardTitle>输出结果</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            {error && (
              <Badge variant="destructive" className="mb-2">
                错误：{error}
              </Badge>
            )}
            <Textarea
              placeholder="格式化结果将显示在这里..."
              value={output}
              readOnly
              className="min-h-[300px] font-mono text-sm bg-muted/50"
            />
            {output && (
              <Button
                variant="outline"
                size="sm"
                onClick={() => navigator.clipboard.writeText(output)}
              >
                复制结果
              </Button>
            )}
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>使用说明</CardTitle>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground space-y-2">
          <p>• <strong>格式化</strong>：将压缩的 JSON 转换为易读的缩进格式</p>
          <p>• <strong>压缩</strong>：移除 JSON 中的空格和换行符，生成最小化版本</p>
          <p>• 自动检测 JSON 语法错误并显示详细错误信息</p>
        </CardContent>
      </Card>
    </div>
  )
}