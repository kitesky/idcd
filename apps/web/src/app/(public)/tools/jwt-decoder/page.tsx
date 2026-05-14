"use client"

import { useState } from "react"
import { Card, CardContent, CardHeader, CardTitle, Textarea, Badge } from "@/components/ui"

interface JWTData {
  header: any
  payload: any
  signature: string
}

export default function JwtDecoderPage() {
  const [input, setInput] = useState('')
  const [output, setOutput] = useState<JWTData | null>(null)
  const [error, setError] = useState('')

  const decodeJWT = (token: string): JWTData => {
    const parts = token.split('.')

    if (parts.length !== 3) {
      throw new Error('无效的 JWT 格式，应包含 3 个部分（Header.Payload.Signature）')
    }

    const [headerB64, payloadB64, signature] = parts

    // Decode header
    let header: any
    try {
      const headerJson = atob(headerB64.replace(/-/g, '+').replace(/_/g, '/'))
      header = JSON.parse(headerJson)
    } catch (err) {
      throw new Error('无法解码 JWT Header')
    }

    // Decode payload
    let payload: any
    try {
      const payloadJson = atob(payloadB64.replace(/-/g, '+').replace(/_/g, '/'))
      payload = JSON.parse(payloadJson)
    } catch (err) {
      throw new Error('无法解码 JWT Payload')
    }

    return { header, payload, signature }
  }

  const handleInputChange = (value: string) => {
    setInput(value)

    const trimmedValue = value.trim()
    if (!trimmedValue) {
      setOutput(null)
      setError('')
      return
    }

    try {
      const decoded = decodeJWT(trimmedValue)
      setOutput(decoded)
      setError('')
    } catch (err) {
      setError(err instanceof Error ? err.message : '解码失败')
      setOutput(null)
    }
  }

  const isExpired = (exp?: number): boolean => {
    if (!exp) return false
    return Date.now() / 1000 > exp
  }

  const formatTimestamp = (timestamp: number): string => {
    return new Date(timestamp * 1000).toLocaleString('zh-CN')
  }

  const getExpiryBadge = (exp?: number) => {
    if (!exp) return null

    const expired = isExpired(exp)
    return (
      <Badge variant={expired ? "destructive" : "default"} className="ml-2">
        {expired ? "已过期" : "有效"}
      </Badge>
    )
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">JWT 解码工具</h1>
        <p className="text-muted-foreground mt-2">
          在线解析 JSON Web Token，查看 Header 和 Payload 内容（不验证签名）
        </p>
      </div>

      <div className="grid gap-6">
        {/* Input section */}
        <Card>
          <CardHeader>
            <CardTitle>JWT Token 输入</CardTitle>
          </CardHeader>
          <CardContent>
            <Textarea
              placeholder="在此粘贴 JWT token...例如：eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
              value={input}
              onChange={(e) => handleInputChange(e.target.value)}
              className="min-h-[120px] font-mono text-sm"
            />
          </CardContent>
        </Card>

        {/* Error display */}
        {error && (
          <Card>
            <CardContent className="pt-6">
              <Badge variant="destructive">
                错误：{error}
              </Badge>
            </CardContent>
          </Card>
        )}

        {/* Output sections */}
        {output && (
          <>
            {/* Header */}
            <Card>
              <CardHeader>
                <CardTitle>Header</CardTitle>
              </CardHeader>
              <CardContent>
                <pre className="bg-muted/50 p-4 rounded border font-mono text-sm overflow-x-auto">
                  {JSON.stringify(output.header, null, 2)}
                </pre>
              </CardContent>
            </Card>

            {/* Payload */}
            <Card>
              <CardHeader>
                <CardTitle>Payload</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <pre className="bg-muted/50 p-4 rounded border font-mono text-sm overflow-x-auto">
                  {JSON.stringify(output.payload, null, 2)}
                </pre>

                {/* Common fields explanation */}
                {(output.payload.exp || output.payload.iat || output.payload.nbf) && (
                  <div className="space-y-2 pt-4 border-t">
                    <h4 className="text-sm font-medium">时间字段说明</h4>
                    <div className="text-sm text-muted-foreground space-y-1">
                      {output.payload.iat && (
                        <div>
                          <strong>iat (签发时间)</strong>: {formatTimestamp(output.payload.iat)}
                        </div>
                      )}
                      {output.payload.exp && (
                        <div className="flex items-center">
                          <strong>exp (过期时间)</strong>: {formatTimestamp(output.payload.exp)}
                          {getExpiryBadge(output.payload.exp)}
                        </div>
                      )}
                      {output.payload.nbf && (
                        <div>
                          <strong>nbf (生效时间)</strong>: {formatTimestamp(output.payload.nbf)}
                        </div>
                      )}
                    </div>
                  </div>
                )}
              </CardContent>
            </Card>

            {/* Signature */}
            <Card>
              <CardHeader>
                <CardTitle>Signature</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="bg-muted/50 p-4 rounded border font-mono text-sm break-all">
                  {output.signature}
                </div>
                <p className="text-sm text-muted-foreground mt-2">
                  注意：此工具仅解码 JWT，不验证签名的有效性
                </p>
              </CardContent>
            </Card>
          </>
        )}
      </div>

      <Card>
        <CardHeader>
          <CardTitle>JWT 格式说明</CardTitle>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground space-y-2">
          <p>• <strong>JWT 结构</strong>：Header.Payload.Signature（三个部分用点号分隔）</p>
          <p>• <strong>Header</strong>：包含算法和 token 类型信息</p>
          <p>• <strong>Payload</strong>：包含声明（claims），如用户信息、过期时间等</p>
          <p>• <strong>Signature</strong>：用于验证 token 完整性（本工具不验证签名）</p>
          <p>• <strong>安全提示</strong>：请勿在此输入包含敏感信息的生产环境 JWT</p>
        </CardContent>
      </Card>
    </div>
  )
}