"use client"

import { useState } from "react"
import { Card, CardContent, CardHeader, CardTitle, Input, Badge } from "@/components/ui"

interface IPv6Result {
  originalInput: string
  isValid: boolean
  fullExpanded: string
  compressed: string
  addressType: string
  addressScope: string
  prefixLength?: string
}

export default function IPv6ConverterPage() {
  const [input, setInput] = useState('')
  const [result, setResult] = useState<IPv6Result | null>(null)
  const [error, setError] = useState('')

  const expandIPv6 = (ipv6: string): string => {
    // Remove prefix if present
    const addr = ipv6.includes('/') ? ipv6.split('/')[0] : ipv6

    // Handle :: notation
    let expanded = addr

    if (expanded.includes('::')) {
      const parts = expanded.split('::')
      const left = parts[0] ? parts[0].split(':') : []
      const right = parts[1] ? parts[1].split(':') : []

      const totalGroups = 8
      const missingGroups = totalGroups - left.length - right.length

      const middle = Array(missingGroups).fill('0000')
      const allGroups = [...left, ...middle, ...right]

      expanded = allGroups.map(group => group.padStart(4, '0')).join(':')
    } else {
      // Just pad each group
      expanded = expanded.split(':').map(group => group.padStart(4, '0')).join(':')
    }

    return expanded.toLowerCase()
  }

  const compressIPv6 = (fullIPv6: string): string => {
    let groups = fullIPv6.split(':')

    // Remove leading zeros from each group
    groups = groups.map(group => group.replace(/^0+/, '') || '0')

    let compressed = groups.join(':')

    // Find longest sequence of consecutive zeros
    let longestZeroSeq = ''
    let currentZeroSeq = ''
    let inZeroSeq = false

    const temp = compressed.split(':')
    let sequences: { start: number, length: number }[] = []
    let currentSeq = { start: -1, length: 0 }

    for (let i = 0; i < temp.length; i++) {
      if (temp[i] === '0') {
        if (currentSeq.start === -1) {
          currentSeq.start = i
          currentSeq.length = 1
        } else {
          currentSeq.length++
        }
      } else {
        if (currentSeq.start !== -1 && currentSeq.length > 1) {
          sequences.push({ ...currentSeq })
        }
        currentSeq = { start: -1, length: 0 }
      }
    }

    // Check the last sequence
    if (currentSeq.start !== -1 && currentSeq.length > 1) {
      sequences.push({ ...currentSeq })
    }

    // Find longest sequence
    if (sequences.length > 0) {
      const longest = sequences.reduce((a, b) => a.length > b.length ? a : b)

      if (longest.length >= 2) {
        const before = temp.slice(0, longest.start).join(':')
        const after = temp.slice(longest.start + longest.length).join(':')

        if (before && after) {
          compressed = `${before}::${after}`
        } else if (before) {
          compressed = `${before}::`
        } else if (after) {
          compressed = `::${after}`
        } else {
          compressed = '::'
        }
      }
    }

    return compressed
  }

  const getAddressType = (ipv6: string): { type: string, scope: string } => {
    const expanded = expandIPv6(ipv6)
    const firstHex = expanded.substring(0, 4)
    const firstOctet = parseInt(firstHex, 16)

    // Loopback
    if (expanded === '0000:0000:0000:0000:0000:0000:0000:0001') {
      return { type: 'Loopback', scope: 'Node-local' }
    }

    // Unspecified
    if (expanded === '0000:0000:0000:0000:0000:0000:0000:0000') {
      return { type: 'Unspecified', scope: 'N/A' }
    }

    // Link-local
    if (firstHex >= 'fe80' && firstHex <= 'febf') {
      return { type: 'Link-local Unicast', scope: 'Link-local' }
    }

    // Site-local (deprecated)
    if (firstHex >= 'fec0' && firstHex <= 'feff') {
      return { type: 'Site-local Unicast (deprecated)', scope: 'Site-local' }
    }

    // Unique local
    if (firstHex >= 'fc00' && firstHex <= 'fdff') {
      return { type: 'Unique Local Unicast', scope: 'Global' }
    }

    // Multicast
    if (expanded.startsWith('ff')) {
      const scopeNibble = parseInt(expanded.charAt(3), 16)
      const scopes = {
        0: 'Reserved',
        1: 'Interface-local',
        2: 'Link-local',
        4: 'Admin-local',
        5: 'Site-local',
        8: 'Organization-local',
        14: 'Global'
      }
      return {
        type: 'Multicast',
        scope: scopes[scopeNibble as keyof typeof scopes] || 'Unknown scope'
      }
    }

    // Global unicast
    if (firstOctet >= 0x2000 && firstOctet <= 0x3fff) {
      return { type: 'Global Unicast', scope: 'Global' }
    }

    // Documentation prefix
    if (expanded.startsWith('2001:0db8')) {
      return { type: 'Documentation', scope: 'Reserved' }
    }

    return { type: 'Unknown/Reserved', scope: 'Unknown' }
  }

  const validateIPv6 = (ipv6: string): boolean => {
    try {
      // Remove prefix if present
      const addr = ipv6.includes('/') ? ipv6.split('/')[0] : ipv6

      // Basic pattern check
      const ipv6Pattern = /^([0-9a-fA-F]{1,4}:){7}[0-9a-fA-F]{1,4}$|^([0-9a-fA-F]{1,4}:)*::([0-9a-fA-F]{1,4}:)*[0-9a-fA-F]{1,4}$|^::([0-9a-fA-F]{1,4}:)*[0-9a-fA-F]{1,4}$|^([0-9a-fA-F]{1,4}:)+::$|^::$/

      if (!ipv6Pattern.test(addr)) {
        return false
      }

      // Try to expand - if it throws, it's invalid
      expandIPv6(addr)
      return true
    } catch {
      return false
    }
  }

  const handleInputChange = (value: string) => {
    setInput(value)

    const trimmed = value.trim()
    if (!trimmed) {
      setResult(null)
      setError('')
      return
    }

    try {
      const isValid = validateIPv6(trimmed)

      if (!isValid) {
        setError('无效的 IPv6 地址格式')
        setResult(null)
        return
      }

      const addr = trimmed.includes('/') ? trimmed.split('/')[0] : trimmed
      const prefixLength = trimmed.includes('/') ? trimmed.split('/')[1] : undefined

      const fullExpanded = expandIPv6(addr)
      const compressed = compressIPv6(fullExpanded)
      const { type, scope } = getAddressType(addr)

      setResult({
        originalInput: trimmed,
        isValid: true,
        fullExpanded,
        compressed,
        addressType: type,
        addressScope: scope,
        prefixLength
      })
      setError('')
    } catch (err) {
      setError(err instanceof Error ? err.message : '转换失败')
      setResult(null)
    }
  }

  const examples = [
    { name: '压缩格式', ipv6: '2001:db8::1' },
    { name: '完整格式', ipv6: '2001:0db8:0000:0000:0000:0000:0000:0001' },
    { name: '环回地址', ipv6: '::1' },
    { name: '未指定地址', ipv6: '::' },
    { name: 'Link-local', ipv6: 'fe80::1' },
    { name: '组播地址', ipv6: 'ff02::1' },
    { name: '带前缀长度', ipv6: '2001:db8::/32' }
  ]

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">IPv6 转换器</h1>
        <p className="text-muted-foreground mt-2">
          IPv6 地址格式转换工具，支持完整展开、压缩格式转换，地址类型识别
        </p>
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        {/* Input section */}
        <Card>
          <CardHeader>
            <CardTitle>IPv6 地址输入</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <label className="text-sm font-medium">IPv6 地址</label>
              <Input
                placeholder="例如：2001:db8::1 或 fe80::1/64"
                value={input}
                onChange={(e) => handleInputChange(e.target.value)}
                className="font-mono"
              />
              <p className="text-xs text-muted-foreground">
                支持压缩格式、完整格式，可选前缀长度（/数字）
              </p>
            </div>

            {error && (
              <Badge variant="destructive">
                错误：{error}
              </Badge>
            )}

            <div className="space-y-2">
              <label className="text-sm font-medium">常用示例</label>
              <div className="space-y-1">
                {examples.map((example, index) => (
                  <button
                    key={index}
                    onClick={() => setInput(example.ipv6)}
                    className="block w-full text-left px-2 py-1 text-xs bg-muted/50 hover:bg-muted rounded"
                  >
                    <span className="font-medium">{example.name}</span>: <code>{example.ipv6}</code>
                  </button>
                ))}
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Address Info */}
        {result && (
          <Card>
            <CardHeader>
              <CardTitle>地址信息</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              <div className="grid gap-3">
                <div>
                  <label className="text-xs font-medium text-muted-foreground">地址类型</label>
                  <div className="text-sm bg-muted/50 p-2 rounded border">
                    {result.addressType}
                  </div>
                </div>

                <div>
                  <label className="text-xs font-medium text-muted-foreground">地址范围</label>
                  <div className="text-sm bg-muted/50 p-2 rounded border">
                    {result.addressScope}
                  </div>
                </div>

                {result.prefixLength && (
                  <div>
                    <label className="text-xs font-medium text-muted-foreground">前缀长度</label>
                    <div className="font-mono text-sm bg-muted/50 p-2 rounded border">
                      /{result.prefixLength}
                    </div>
                  </div>
                )}

                <div>
                  <label className="text-xs font-medium text-muted-foreground">
                    状态
                    <Badge variant={result.isValid ? "default" : "destructive"} className="ml-2 text-xs">
                      {result.isValid ? "有效" : "无效"}
                    </Badge>
                  </label>
                </div>
              </div>
            </CardContent>
          </Card>
        )}
      </div>

      {/* Conversion Results */}
      {result && (
        <Card>
          <CardHeader>
            <CardTitle>转换结果</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid gap-4">
              <div>
                <label className="text-sm font-medium text-muted-foreground">原始输入</label>
                <div className="font-mono text-sm bg-muted/50 p-3 rounded border break-all">
                  {result.originalInput}
                </div>
              </div>

              <div>
                <label className="text-sm font-medium text-muted-foreground">
                  完整展开格式
                  <button
                    onClick={() => navigator.clipboard.writeText(result.fullExpanded)}
                    className="ml-2 text-xs text-primary hover:underline"
                  >
                    复制
                  </button>
                </label>
                <div className="font-mono text-sm bg-muted/50 p-3 rounded border break-all">
                  {result.fullExpanded}
                </div>
              </div>

              <div>
                <label className="text-sm font-medium text-muted-foreground">
                  压缩格式
                  <button
                    onClick={() => navigator.clipboard.writeText(result.compressed)}
                    className="ml-2 text-xs text-primary hover:underline"
                  >
                    复制
                  </button>
                </label>
                <div className="font-mono text-sm bg-muted/50 p-3 rounded border break-all">
                  {result.compressed}
                </div>
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader>
          <CardTitle>IPv6 地址类型说明</CardTitle>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground space-y-3">
          <div>
            <p><strong className="text-foreground">Global Unicast</strong> (2000::/3)：全球单播地址，类似 IPv4 公网地址</p>
          </div>

          <div>
            <p><strong className="text-foreground">Link-local</strong> (fe80::/10)：链路本地地址，仅在本地链路有效</p>
          </div>

          <div>
            <p><strong className="text-foreground">Unique Local</strong> (fc00::/7)：唯一本地地址，类似 IPv4 私网地址</p>
          </div>

          <div>
            <p><strong className="text-foreground">Multicast</strong> (ff00::/8)：组播地址，用于一对多通信</p>
          </div>

          <div>
            <p><strong className="text-foreground">Loopback</strong> (::1)：环回地址，类似 IPv4 的 127.0.0.1</p>
          </div>

          <div>
            <p><strong className="text-foreground">特殊符号</strong>：</p>
            <p className="ml-4">• <code>::</code> 表示连续的零组（只能出现一次）</p>
            <p className="ml-4">• 前导零可省略（001a 写作 1a）</p>
            <p className="ml-4">• 大小写不敏感</p>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}