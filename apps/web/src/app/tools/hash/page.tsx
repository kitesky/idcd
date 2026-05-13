"use client"

import { useState } from "react"
import { Card, CardContent, CardHeader, CardTitle, Textarea, Button, Badge } from "@idcd/ui"
import CryptoJS from "crypto-js"

type HashAlgorithm = 'MD5' | 'SHA1' | 'SHA256' | 'SHA512'

export default function HashPage() {
  const [input, setInput] = useState('')
  const [results, setResults] = useState<Record<HashAlgorithm, string>>({
    MD5: '',
    SHA1: '',
    SHA256: '',
    SHA512: ''
  })
  const [selectedAlgorithm, setSelectedAlgorithm] = useState<HashAlgorithm>('SHA256')
  const [error, setError] = useState('')

  const calculateHash = (algorithm: HashAlgorithm, text: string): string => {
    switch (algorithm) {
      case 'MD5':
        return CryptoJS.MD5(text).toString()
      case 'SHA1':
        return CryptoJS.SHA1(text).toString()
      case 'SHA256':
        return CryptoJS.SHA256(text).toString()
      case 'SHA512':
        return CryptoJS.SHA512(text).toString()
      default:
        throw new Error(`不支持的哈希算法: ${algorithm}`)
    }
  }

  const calculateAllHashes = () => {
    try {
      const text = input.trim()
      if (!text) {
        setError('请输入要计算哈希的文本')
        return
      }

      const newResults: Record<HashAlgorithm, string> = {
        MD5: calculateHash('MD5', text),
        SHA1: calculateHash('SHA1', text),
        SHA256: calculateHash('SHA256', text),
        SHA512: calculateHash('SHA512', text)
      }

      setResults(newResults)
      setError('')
    } catch (err) {
      setError(err instanceof Error ? err.message : '计算哈希失败')
      setResults({ MD5: '', SHA1: '', SHA256: '', SHA512: '' })
    }
  }

  const calculateSingleHash = (algorithm: HashAlgorithm) => {
    try {
      const text = input.trim()
      if (!text) {
        setError('请输入要计算哈希的文本')
        return
      }

      const hash = calculateHash(algorithm, text)
      setResults(prev => ({ ...prev, [algorithm]: hash }))
      setError('')
    } catch (err) {
      setError(err instanceof Error ? err.message : '计算哈希失败')
    }
  }

  const handleInputChange = (value: string) => {
    setInput(value)
    if (error) setError('')
  }

  const copyToClipboard = (text: string, algorithm: string) => {
    navigator.clipboard.writeText(text)
      .then(() => {
        // Could add a toast notification here
      })
      .catch(() => {
        // Handle copy error
      })
  }

  const algorithms: { key: HashAlgorithm; name: string; description: string }[] = [
    { key: 'MD5', name: 'MD5', description: '128位哈希（已不安全，仅用于校验）' },
    { key: 'SHA1', name: 'SHA-1', description: '160位哈希（已不安全）' },
    { key: 'SHA256', name: 'SHA-256', description: '256位哈希（推荐）' },
    { key: 'SHA512', name: 'SHA-512', description: '512位哈希（高安全性）' }
  ]

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">哈希计算工具</h1>
        <p className="text-muted-foreground mt-2">
          支持 MD5、SHA-1、SHA-256、SHA-512 等多种哈希算法的在线计算工具
        </p>
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        {/* Input section */}
        <Card>
          <CardHeader>
            <CardTitle>文本输入</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <Textarea
              placeholder="在此输入要计算哈希的文本..."
              value={input}
              onChange={(e) => handleInputChange(e.target.value)}
              className="min-h-[200px] font-mono text-sm"
            />
            <div className="flex flex-wrap gap-2">
              <Button onClick={calculateAllHashes} className="flex-1 min-w-[120px]">
                计算所有哈希
              </Button>
              {algorithms.map((algo) => (
                <Button
                  key={algo.key}
                  onClick={() => calculateSingleHash(algo.key)}
                  variant="outline"
                  size="sm"
                >
                  {algo.name}
                </Button>
              ))}
            </div>
          </CardContent>
        </Card>

        {/* Output section */}
        <Card>
          <CardHeader>
            <CardTitle>哈希结果</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            {error && (
              <Badge variant="destructive" className="mb-2">
                错误：{error}
              </Badge>
            )}

            <div className="space-y-4">
              {algorithms.map((algo) => (
                <div key={algo.key} className="space-y-2">
                  <div className="flex items-center justify-between">
                    <label className="text-sm font-medium">{algo.name}</label>
                    {results[algo.key] && (
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => copyToClipboard(results[algo.key], algo.name)}
                        className="h-6 px-2 text-xs"
                      >
                        复制
                      </Button>
                    )}
                  </div>
                  <div className="font-mono text-xs bg-muted/50 p-3 rounded border break-all min-h-[2.5rem] flex items-center">
                    {results[algo.key] || '等待计算...'}
                  </div>
                  <p className="text-xs text-muted-foreground">{algo.description}</p>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>算法说明</CardTitle>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground space-y-3">
          <div>
            <p><strong className="text-foreground">MD5</strong>：128位哈希算法，速度快但已不安全，仅适用于文件校验</p>
          </div>
          <div>
            <p><strong className="text-foreground">SHA-1</strong>：160位哈希算法，已被发现碰撞攻击，不推荐用于安全敏感场景</p>
          </div>
          <div>
            <p><strong className="text-foreground">SHA-256</strong>：256位哈希算法，SHA-2 系列，当前主流推荐算法</p>
          </div>
          <div>
            <p><strong className="text-foreground">SHA-512</strong>：512位哈希算法，SHA-2 系列，提供更高安全性但输出更长</p>
          </div>
          <div className="pt-2 border-t">
            <p className="text-xs">
              <strong>注意</strong>：哈希计算在客户端进行，输入的文本不会上传到服务器
            </p>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}