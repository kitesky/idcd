"use client"

import { useState } from "react"
import { Card, CardContent, CardHeader, CardTitle, Textarea, Button, Badge } from "@/components/ui"

type HashAlgorithm = 'MD5' | 'SHA1' | 'SHA256' | 'SHA512'

// MD5 is not available in Web Crypto API (considered insecure). This is a pure-JS
// implementation used only for the legacy hash-tool display — not for any security purpose.
function md5(input: string): string {
  const rotateLeft = (value: number, shift: number) =>
    (value << shift) | (value >>> (32 - shift))

  const addUnsigned = (x: number, y: number) => {
    const x8 = x & 0x80000000
    const y8 = y & 0x80000000
    const x4 = x & 0x40000000
    const y4 = y & 0x40000000
    const t = (x & 0x3fffffff) + (y & 0x3fffffff)
    if (x4 & y4) return t ^ 0x80000000 ^ x8 ^ y8
    if (x4 | y4) {
      if (t & 0x40000000) return t ^ 0xc0000000 ^ x8 ^ y8
      return t ^ 0x40000000 ^ x8 ^ y8
    }
    return t ^ x8 ^ y8
  }

  const F = (x: number, y: number, z: number) => (x & y) | (~x & z)
  const G = (x: number, y: number, z: number) => (x & z) | (y & ~z)
  const H = (x: number, y: number, z: number) => x ^ y ^ z
  const I = (x: number, y: number, z: number) => y ^ (x | ~z)

  const FF = (a: number, b: number, c: number, d: number, x: number, s: number, ac: number) =>
    addUnsigned(rotateLeft(addUnsigned(addUnsigned(addUnsigned(a, F(b, c, d)), x), ac), s), b)
  const GG = (a: number, b: number, c: number, d: number, x: number, s: number, ac: number) =>
    addUnsigned(rotateLeft(addUnsigned(addUnsigned(addUnsigned(a, G(b, c, d)), x), ac), s), b)
  const HH = (a: number, b: number, c: number, d: number, x: number, s: number, ac: number) =>
    addUnsigned(rotateLeft(addUnsigned(addUnsigned(addUnsigned(a, H(b, c, d)), x), ac), s), b)
  const II = (a: number, b: number, c: number, d: number, x: number, s: number, ac: number) =>
    addUnsigned(rotateLeft(addUnsigned(addUnsigned(addUnsigned(a, I(b, c, d)), x), ac), s), b)

  const convertToWordArray = (str: string) => {
    const bytes = new TextEncoder().encode(str)
    const nBytes = bytes.length
    const nWords = ((nBytes + 8) >> 6) + 1
    const words = new Array(nWords * 16).fill(0)
    for (let i = 0; i < nBytes; i++) {
      words[i >> 2] |= bytes[i]! << ((i % 4) * 8)
    }
    words[nBytes >> 2] |= 0x80 << ((nBytes % 4) * 8)
    words[nWords * 16 - 2] = nBytes * 8
    return words
  }

  const wordToHex = (value: number) => {
    let hex = ''
    for (let i = 0; i <= 3; i++) {
      hex += ('0' + ((value >> (i * 8)) & 0xff).toString(16)).slice(-2)
    }
    return hex
  }

  const x = convertToWordArray(input)
  let [a, b, c, d] = [0x67452301, 0xefcdab89, 0x98badcfe, 0x10325476]

  for (let i = 0; i < x.length; i += 16) {
    const [aa, bb, cc, dd] = [a, b, c, d]
    a = FF(a,b,c,d, x[i+0],  7, 0xd76aa478); d = FF(d,a,b,c, x[i+1], 12, 0xe8c7b756)
    c = FF(c,d,a,b, x[i+2], 17, 0x242070db); b = FF(b,c,d,a, x[i+3], 22, 0xc1bdceee)
    a = FF(a,b,c,d, x[i+4],  7, 0xf57c0faf); d = FF(d,a,b,c, x[i+5], 12, 0x4787c62a)
    c = FF(c,d,a,b, x[i+6], 17, 0xa8304613); b = FF(b,c,d,a, x[i+7], 22, 0xfd469501)
    a = FF(a,b,c,d, x[i+8],  7, 0x698098d8); d = FF(d,a,b,c, x[i+9], 12, 0x8b44f7af)
    c = FF(c,d,a,b, x[i+10],17, 0xffff5bb1); b = FF(b,c,d,a, x[i+11],22, 0x895cd7be)
    a = FF(a,b,c,d, x[i+12], 7, 0x6b901122); d = FF(d,a,b,c, x[i+13],12, 0xfd987193)
    c = FF(c,d,a,b, x[i+14],17, 0xa679438e); b = FF(b,c,d,a, x[i+15],22, 0x49b40821)
    a = GG(a,b,c,d, x[i+1],  5, 0xf61e2562); d = GG(d,a,b,c, x[i+6],  9, 0xc040b340)
    c = GG(c,d,a,b, x[i+11],14, 0x265e5a51); b = GG(b,c,d,a, x[i+0], 20, 0xe9b6c7aa)
    a = GG(a,b,c,d, x[i+5],  5, 0xd62f105d); d = GG(d,a,b,c, x[i+10], 9, 0x02441453)
    c = GG(c,d,a,b, x[i+15],14, 0xd8a1e681); b = GG(b,c,d,a, x[i+4], 20, 0xe7d3fbc8)
    a = GG(a,b,c,d, x[i+9],  5, 0x21e1cde6); d = GG(d,a,b,c, x[i+14], 9, 0xc33707d6)
    c = GG(c,d,a,b, x[i+3], 14, 0xf4d50d87); b = GG(b,c,d,a, x[i+8], 20, 0x455a14ed)
    a = GG(a,b,c,d, x[i+13], 5, 0xa9e3e905); d = GG(d,a,b,c, x[i+2],  9, 0xfcefa3f8)
    c = GG(c,d,a,b, x[i+7], 14, 0x676f02d9); b = GG(b,c,d,a, x[i+12],20, 0x8d2a4c8a)
    a = HH(a,b,c,d, x[i+5],  4, 0xfffa3942); d = HH(d,a,b,c, x[i+8], 11, 0x8771f681)
    c = HH(c,d,a,b, x[i+11],16, 0x6d9d6122); b = HH(b,c,d,a, x[i+14],23, 0xfde5380c)
    a = HH(a,b,c,d, x[i+1],  4, 0xa4beea44); d = HH(d,a,b,c, x[i+4], 11, 0x4bdecfa9)
    c = HH(c,d,a,b, x[i+7], 16, 0xf6bb4b60); b = HH(b,c,d,a, x[i+10],23, 0xbebfbc70)
    a = HH(a,b,c,d, x[i+13], 4, 0x289b7ec6); d = HH(d,a,b,c, x[i+0], 11, 0xeaa127fa)
    c = HH(c,d,a,b, x[i+3], 16, 0xd4ef3085); b = HH(b,c,d,a, x[i+6], 23, 0x04881d05)
    a = HH(a,b,c,d, x[i+9],  4, 0xd9d4d039); d = HH(d,a,b,c, x[i+12],11, 0xe6db99e5)
    c = HH(c,d,a,b, x[i+15],16, 0x1fa27cf8); b = HH(b,c,d,a, x[i+2], 23, 0xc4ac5665)
    a = II(a,b,c,d, x[i+0],  6, 0xf4292244); d = II(d,a,b,c, x[i+7], 10, 0x432aff97)
    c = II(c,d,a,b, x[i+14],15, 0xab9423a7); b = II(b,c,d,a, x[i+5], 21, 0xfc93a039)
    a = II(a,b,c,d, x[i+12], 6, 0x655b59c3); d = II(d,a,b,c, x[i+3], 10, 0x8f0ccc92)
    c = II(c,d,a,b, x[i+10],15, 0xffeff47d); b = II(b,c,d,a, x[i+1], 21, 0x85845dd1)
    a = II(a,b,c,d, x[i+8],  6, 0x6fa87e4f); d = II(d,a,b,c, x[i+15],10, 0xfe2ce6e0)
    c = II(c,d,a,b, x[i+6], 15, 0xa3014314); b = II(b,c,d,a, x[i+13],21, 0x4e0811a1)
    a = II(a,b,c,d, x[i+4],  6, 0xf7537e82); d = II(d,a,b,c, x[i+11],10, 0xbd3af235)
    c = II(c,d,a,b, x[i+2], 15, 0x2ad7d2bb); b = II(b,c,d,a, x[i+9], 21, 0xeb86d391)
    a = addUnsigned(a, aa); b = addUnsigned(b, bb)
    c = addUnsigned(c, cc); d = addUnsigned(d, dd)
  }

  return wordToHex(a) + wordToHex(b) + wordToHex(c) + wordToHex(d)
}

async function subtleCryptoHash(algorithm: 'SHA-1' | 'SHA-256' | 'SHA-512', text: string): Promise<string> {
  const encoded = new TextEncoder().encode(text)
  const hashBuffer = await crypto.subtle.digest(algorithm, encoded)
  return Array.from(new Uint8Array(hashBuffer))
    .map(b => b.toString(16).padStart(2, '0'))
    .join('')
}

export default function HashPage() {
  const [input, setInput] = useState('')
  const [results, setResults] = useState<Record<HashAlgorithm, string>>({
    MD5: '',
    SHA1: '',
    SHA256: '',
    SHA512: ''
  })
  const [error, setError] = useState('')

  const calculateHash = async (algorithm: HashAlgorithm, text: string): Promise<string> => {
    switch (algorithm) {
      case 'MD5':   return md5(text)
      case 'SHA1':  return subtleCryptoHash('SHA-1', text)
      case 'SHA256':return subtleCryptoHash('SHA-256', text)
      case 'SHA512':return subtleCryptoHash('SHA-512', text)
      default:      throw new Error(`不支持的哈希算法: ${algorithm}`)
    }
  }

  const calculateAllHashes = async () => {
    try {
      const text = input.trim()
      if (!text) { setError('请输入要计算哈希的文本'); return }
      const [md5Hash, sha1Hash, sha256Hash, sha512Hash] = await Promise.all([
        calculateHash('MD5', text),
        calculateHash('SHA1', text),
        calculateHash('SHA256', text),
        calculateHash('SHA512', text),
      ])
      setResults({ MD5: md5Hash, SHA1: sha1Hash, SHA256: sha256Hash, SHA512: sha512Hash })
      setError('')
    } catch (err) {
      setError(err instanceof Error ? err.message : '计算哈希失败')
      setResults({ MD5: '', SHA1: '', SHA256: '', SHA512: '' })
    }
  }

  const calculateSingleHash = async (algorithm: HashAlgorithm) => {
    try {
      const text = input.trim()
      if (!text) { setError('请输入要计算哈希的文本'); return }
      const hash = await calculateHash(algorithm, text)
      setResults(prev => ({ ...prev, [algorithm]: hash }))
      setError('')
    } catch (err) {
      setError(err instanceof Error ? err.message : '计算哈希失败')
    }
  }

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text).catch(() => {})
  }

  const algorithms: { key: HashAlgorithm; name: string; description: string }[] = [
    { key: 'MD5',    name: 'MD5',    description: '128位哈希（已不安全，仅用于校验）' },
    { key: 'SHA1',   name: 'SHA-1',  description: '160位哈希（已不安全）' },
    { key: 'SHA256', name: 'SHA-256',description: '256位哈希（推荐）' },
    { key: 'SHA512', name: 'SHA-512',description: '512位哈希（高安全性）' },
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
        <Card>
          <CardHeader><CardTitle>文本输入</CardTitle></CardHeader>
          <CardContent className="space-y-4">
            <Textarea
              placeholder="在此输入要计算哈希的文本..."
              value={input}
              onChange={(e) => { setInput(e.target.value); if (error) setError('') }}
              className="min-h-[200px] font-mono text-sm"
            />
            <div className="flex flex-wrap gap-2">
              <Button onClick={calculateAllHashes} className="flex-1 min-w-[120px]">
                计算所有哈希
              </Button>
              {algorithms.map((algo) => (
                <Button key={algo.key} onClick={() => calculateSingleHash(algo.key)} variant="outline" size="sm">
                  {algo.name}
                </Button>
              ))}
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader><CardTitle>哈希结果</CardTitle></CardHeader>
          <CardContent className="space-y-4">
            {error && <Badge variant="destructive" className="mb-2">错误：{error}</Badge>}
            <div className="space-y-4">
              {algorithms.map((algo) => (
                <div key={algo.key} className="space-y-2">
                  <div className="flex items-center justify-between">
                    <label className="text-sm font-medium">{algo.name}</label>
                    {results[algo.key] && (
                      <Button variant="ghost" size="sm" onClick={() => copyToClipboard(results[algo.key])}
                        className="h-6 px-2 text-xs">复制</Button>
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
        <CardHeader><CardTitle>算法说明</CardTitle></CardHeader>
        <CardContent className="text-sm text-muted-foreground space-y-3">
          <p><strong className="text-foreground">MD5</strong>：128位哈希算法，速度快但已不安全，仅适用于文件校验</p>
          <p><strong className="text-foreground">SHA-1</strong>：160位哈希算法，已被发现碰撞攻击，不推荐用于安全敏感场景</p>
          <p><strong className="text-foreground">SHA-256</strong>：256位哈希算法，SHA-2 系列，当前主流推荐算法</p>
          <p><strong className="text-foreground">SHA-512</strong>：512位哈希算法，SHA-2 系列，提供更高安全性但输出更长</p>
          <div className="pt-2 border-t">
            <p className="text-xs"><strong>注意</strong>：哈希计算在客户端进行，输入的文本不会上传到服务器</p>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
