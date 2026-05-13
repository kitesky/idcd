"use client"

import { useState } from "react"
import { Card, CardContent, CardHeader, CardTitle, Input, Badge } from "@idcd/ui"

interface CIDRResult {
  networkAddress: string
  broadcastAddress: string
  subnetMask: string
  wildcardMask: string
  firstHost: string
  lastHost: string
  totalHosts: number
  usableHosts: number
  subnetBits: number
  hostBits: number
  ipClass: string
}

export default function CIDRCalculatorPage() {
  const [input, setInput] = useState('')
  const [result, setResult] = useState<CIDRResult | null>(null)
  const [error, setError] = useState('')

  const ipToInt = (ip: string): number => {
    const parts = ip.split('.').map(Number)
    return (parts[0] << 24) | (parts[1] << 16) | (parts[2] << 8) | parts[3]
  }

  const intToIp = (int: number): string => {
    return [
      (int >>> 24) & 255,
      (int >>> 16) & 255,
      (int >>> 8) & 255,
      int & 255
    ].join('.')
  }

  const getIPClass = (firstOctet: number): string => {
    if (firstOctet >= 1 && firstOctet <= 126) return 'A'
    if (firstOctet >= 128 && firstOctet <= 191) return 'B'
    if (firstOctet >= 192 && firstOctet <= 223) return 'C'
    if (firstOctet >= 224 && firstOctet <= 239) return 'D (组播)'
    if (firstOctet >= 240 && firstOctet <= 255) return 'E (保留)'
    return '无效'
  }

  const calculateCIDR = (cidr: string): CIDRResult => {
    const [ipStr, prefixStr] = cidr.split('/')

    if (!ipStr || !prefixStr) {
      throw new Error('无效的 CIDR 格式，应为 IP/前缀长度')
    }

    const prefix = parseInt(prefixStr)
    if (isNaN(prefix) || prefix < 0 || prefix > 32) {
      throw new Error('前缀长度必须在 0-32 之间')
    }

    // Validate IP address
    const ipParts = ipStr.split('.')
    if (ipParts.length !== 4) {
      throw new Error('无效的 IP 地址格式')
    }

    const ipNumbers = ipParts.map(Number)
    if (ipNumbers.some(num => isNaN(num) || num < 0 || num > 255)) {
      throw new Error('IP 地址的每个八位组必须在 0-255 之间')
    }

    const ip = ipToInt(ipStr)
    const subnetBits = prefix
    const hostBits = 32 - prefix

    // Calculate subnet mask
    const subnetMask = (~0 << hostBits) >>> 0
    const wildcardMask = ~subnetMask >>> 0

    // Calculate network and broadcast addresses
    const networkAddress = ip & subnetMask
    const broadcastAddress = networkAddress | wildcardMask

    // Calculate host range
    const firstHost = networkAddress + 1
    const lastHost = broadcastAddress - 1

    const totalHosts = Math.pow(2, hostBits)
    const usableHosts = Math.max(0, totalHosts - 2) // Subtract network and broadcast

    const firstOctet = parseInt(ipStr.split('.')[0])
    const ipClass = getIPClass(firstOctet)

    return {
      networkAddress: intToIp(networkAddress),
      broadcastAddress: intToIp(broadcastAddress),
      subnetMask: intToIp(subnetMask),
      wildcardMask: intToIp(wildcardMask),
      firstHost: intToIp(firstHost),
      lastHost: intToIp(lastHost),
      totalHosts,
      usableHosts,
      subnetBits,
      hostBits,
      ipClass
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
      const calculated = calculateCIDR(trimmed)
      setResult(calculated)
      setError('')
    } catch (err) {
      setError(err instanceof Error ? err.message : '计算失败')
      setResult(null)
    }
  }

  const examples = [
    { name: '常见家庭网络', cidr: '192.168.1.0/24' },
    { name: '小型办公网络', cidr: '192.168.0.0/22' },
    { name: '大型企业网络', cidr: '10.0.0.0/16' },
    { name: '超大网络', cidr: '10.0.0.0/8' },
    { name: '单主机', cidr: '192.168.1.100/32' },
    { name: '点对点链路', cidr: '192.168.1.0/30' }
  ]

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">CIDR 计算器</h1>
        <p className="text-muted-foreground mt-2">
          IP 网段计算工具，计算网络地址、广播地址、子网掩码、可用 IP 范围
        </p>
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        {/* Input section */}
        <Card>
          <CardHeader>
            <CardTitle>CIDR 输入</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <label className="text-sm font-medium">CIDR 表示法</label>
              <Input
                placeholder="例如：192.168.1.0/24"
                value={input}
                onChange={(e) => handleInputChange(e.target.value)}
                className="font-mono"
              />
              <p className="text-xs text-muted-foreground">
                格式：IP地址/前缀长度（如 192.168.1.0/24）
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
                    onClick={() => setInput(example.cidr)}
                    className="block w-full text-left px-2 py-1 text-xs bg-muted/50 hover:bg-muted rounded"
                  >
                    <span className="font-medium">{example.name}</span>: <code>{example.cidr}</code>
                  </button>
                ))}
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Basic Info */}
        {result && (
          <Card>
            <CardHeader>
              <CardTitle>基本信息</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              <div className="grid gap-3">
                <div>
                  <label className="text-xs font-medium text-muted-foreground">IP 类别</label>
                  <div className="font-mono text-sm bg-muted/50 p-2 rounded border">
                    {result.ipClass}
                  </div>
                </div>

                <div className="grid gap-3 md:grid-cols-2">
                  <div>
                    <label className="text-xs font-medium text-muted-foreground">子网位数</label>
                    <div className="font-mono text-sm bg-muted/50 p-2 rounded border">
                      {result.subnetBits} 位
                    </div>
                  </div>
                  <div>
                    <label className="text-xs font-medium text-muted-foreground">主机位数</label>
                    <div className="font-mono text-sm bg-muted/50 p-2 rounded border">
                      {result.hostBits} 位
                    </div>
                  </div>
                </div>

                <div className="grid gap-3 md:grid-cols-2">
                  <div>
                    <label className="text-xs font-medium text-muted-foreground">总 IP 数</label>
                    <div className="font-mono text-sm bg-muted/50 p-2 rounded border">
                      {result.totalHosts.toLocaleString()}
                    </div>
                  </div>
                  <div>
                    <label className="text-xs font-medium text-muted-foreground">可用 IP 数</label>
                    <div className="font-mono text-sm bg-muted/50 p-2 rounded border">
                      {result.usableHosts.toLocaleString()}
                    </div>
                  </div>
                </div>
              </div>
            </CardContent>
          </Card>
        )}
      </div>

      {/* Detailed Results */}
      {result && (
        <Card>
          <CardHeader>
            <CardTitle>网络详情</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="grid gap-4 md:grid-cols-2">
              <div className="space-y-3">
                <div>
                  <label className="text-sm font-medium text-muted-foreground">网络地址</label>
                  <div className="font-mono text-sm bg-muted/50 p-3 rounded border">
                    {result.networkAddress}
                  </div>
                </div>

                <div>
                  <label className="text-sm font-medium text-muted-foreground">广播地址</label>
                  <div className="font-mono text-sm bg-muted/50 p-3 rounded border">
                    {result.broadcastAddress}
                  </div>
                </div>

                <div>
                  <label className="text-sm font-medium text-muted-foreground">子网掩码</label>
                  <div className="font-mono text-sm bg-muted/50 p-3 rounded border">
                    {result.subnetMask}
                  </div>
                </div>
              </div>

              <div className="space-y-3">
                <div>
                  <label className="text-sm font-medium text-muted-foreground">通配符掩码</label>
                  <div className="font-mono text-sm bg-muted/50 p-3 rounded border">
                    {result.wildcardMask}
                  </div>
                </div>

                <div>
                  <label className="text-sm font-medium text-muted-foreground">第一个可用 IP</label>
                  <div className="font-mono text-sm bg-muted/50 p-3 rounded border">
                    {result.firstHost}
                  </div>
                </div>

                <div>
                  <label className="text-sm font-medium text-muted-foreground">最后一个可用 IP</label>
                  <div className="font-mono text-sm bg-muted/50 p-3 rounded border">
                    {result.lastHost}
                  </div>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader>
          <CardTitle>CIDR 说明</CardTitle>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground space-y-3">
          <div>
            <p><strong className="text-foreground">CIDR 表示法</strong>：Classless Inter-Domain Routing，无类别域间路由</p>
          </div>

          <div>
            <p><strong className="text-foreground">前缀长度</strong>：表示网络部分的位数</p>
            <p className="ml-4">• /8 = 255.0.0.0 (A类默认)</p>
            <p className="ml-4">• /16 = 255.255.0.0 (B类默认)</p>
            <p className="ml-4">• /24 = 255.255.255.0 (C类默认)</p>
            <p className="ml-4">• /30 = 255.255.255.252 (点对点链路)</p>
          </div>

          <div>
            <p><strong className="text-foreground">特殊地址</strong>：</p>
            <p className="ml-4">• 网络地址：不能分配给主机</p>
            <p className="ml-4">• 广播地址：用于广播通信</p>
            <p className="ml-4">• 可用 IP = 总 IP - 2（网络地址 + 广播地址）</p>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}