"use client"

import { useState } from "react"
import { Card, CardContent, CardHeader, CardTitle, Textarea, Button, Badge, cn } from "@idcd/ui"

type TabType = 'encode' | 'decode'

export default function Base64Page() {
  const [activeTab, setActiveTab] = useState<TabType>('encode')
  const [input, setInput] = useState('')
  const [output, setOutput] = useState('')
  const [urlSafe, setUrlSafe] = useState(false)
  const [error, setError] = useState('')

  const encodeBase64 = () => {
    try {
      const encoder = new TextEncoder()
      const data = encoder.encode(input)
      let result = btoa(String.fromCharCode(...data))

      if (urlSafe) {
        result = result.replace(/\+/g, '-').replace(/\//g, '_').replace(/=/g, '')
      }

      setOutput(result)
      setError('')
    } catch (err) {
      setError(err instanceof Error ? err.message : '编码失败')
      setOutput('')
    }
  }

  const decodeBase64 = () => {
    try {
      let base64 = input.trim()

      // Handle URL-safe Base64
      if (urlSafe || base64.includes('-') || base64.includes('_')) {
        base64 = base64.replace(/-/g, '+').replace(/_/g, '/')
        // Add padding if missing
        while (base64.length % 4) {
          base64 += '='
        }
      }

      const decoded = atob(base64)
      const decoder = new TextDecoder('utf-8')
      const uint8Array = new Uint8Array(decoded.length)

      for (let i = 0; i < decoded.length; i++) {
        uint8Array[i] = decoded.charCodeAt(i)
      }

      const result = decoder.decode(uint8Array)
      setOutput(result)
      setError('')
    } catch (err) {
      setError(err instanceof Error ? err.message : '解码失败，请检查 Base64 格式')
      setOutput('')
    }
  }

  const handleInputChange = (value: string) => {
    setInput(value)
    if (error) setError('')
  }

  const processData = () => {
    if (activeTab === 'encode') {
      encodeBase64()
    } else {
      decodeBase64()
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">Base64 编解码工具</h1>
        <p className="text-muted-foreground mt-2">
          在线 Base64 编码解码工具，支持 URL-safe Base64 格式
        </p>
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        {/* Input section */}
        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle>
                {activeTab === 'encode' ? '文本输入' : 'Base64 输入'}
              </CardTitle>
              <div className="flex border rounded-lg p-1">
                <button
                  className={cn(
                    "px-3 py-1 text-sm rounded transition-colors",
                    activeTab === 'encode' && "bg-primary text-primary-foreground"
                  )}
                  onClick={() => setActiveTab('encode')}
                >
                  编码
                </button>
                <button
                  className={cn(
                    "px-3 py-1 text-sm rounded transition-colors",
                    activeTab === 'decode' && "bg-primary text-primary-foreground"
                  )}
                  onClick={() => setActiveTab('decode')}
                >
                  解码
                </button>
              </div>
            </div>
          </CardHeader>
          <CardContent className="space-y-4">
            <Textarea
              placeholder={
                activeTab === 'encode'
                  ? "在此输入要编码的文本..."
                  : "在此输入要解码的 Base64..."
              }
              value={input}
              onChange={(e) => handleInputChange(e.target.value)}
              className="min-h-[300px] font-mono text-sm"
            />
            <div className="flex items-center gap-4">
              <div className="flex items-center space-x-2">
                <input
                  type="checkbox"
                  id="url-safe"
                  checked={urlSafe}
                  onChange={(e) => setUrlSafe(e.target.checked)}
                  className="rounded"
                />
                <label htmlFor="url-safe" className="text-sm">
                  URL-safe Base64
                </label>
              </div>
              <Button onClick={processData}>
                {activeTab === 'encode' ? '编码' : '解码'}
              </Button>
            </div>
          </CardContent>
        </Card>

        {/* Output section */}
        <Card>
          <CardHeader>
            <CardTitle>
              {activeTab === 'encode' ? 'Base64 输出' : '文本输出'}
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            {error && (
              <Badge variant="destructive" className="mb-2">
                错误：{error}
              </Badge>
            )}
            <Textarea
              placeholder="结果将显示在这里..."
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
          <p>• <strong>标准 Base64</strong>：使用 A-Z, a-z, 0-9, +, / 字符</p>
          <p>• <strong>URL-safe Base64</strong>：用 - 和 _ 替换 + 和 /，适合在 URL 中传输</p>
          <p>• 自动检测并处理 URL-safe Base64 格式</p>
          <p>• 支持 UTF-8 文本的完整编解码</p>
        </CardContent>
      </Card>
    </div>
  )
}