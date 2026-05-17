"use client"

import { useState, useRef, useEffect } from "react"
import { Card, CardContent, CardHeader, CardTitle, Textarea, Button, Badge, Input, Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui"
import QRCode from "qrcode"

interface QROptions {
  errorCorrectionLevel: 'L' | 'M' | 'Q' | 'H'
  type: 'text' | 'url' | 'wifi' | 'sms' | 'tel'
  size: number
  margin: number
  color: {
    dark: string
    light: string
  }
}

export default function QRCodePage() {
  const [input, setInput] = useState('')
  const [qrOptions, setQROptions] = useState<QROptions>({
    errorCorrectionLevel: 'M',
    type: 'text',
    size: 256,
    margin: 4,
    color: {
      dark: '#000000',
      light: '#ffffff'
    }
  })
  const [qrDataUrl, setQrDataUrl] = useState('')
  const [error, setError] = useState('')
  const canvasRef = useRef<HTMLCanvasElement>(null)

  // WiFi specific fields
  const [wifiSsid, setWifiSsid] = useState('')
  const [wifiPassword, setWifiPassword] = useState('')
  const [wifiSecurity, setWifiSecurity] = useState('WPA')

  const generateQRCode = async () => {
    try {
      let text = input.trim()

      if (!text) {
        if (qrOptions.type === 'wifi') {
          if (!wifiSsid) {
            setError('请输入 WiFi 网络名称')
            return
          }
          // Generate WiFi QR format: WIFI:T:WPA;S:SSID;P:password;;
          text = `WIFI:T:${wifiSecurity};S:${wifiSsid};P:${wifiPassword};;`
        } else {
          setError('请输入内容')
          return
        }
      }

      // Validate URL format for URL type
      if (qrOptions.type === 'url' && text) {
        try {
          new URL(text)
        } catch {
          // Try adding protocol if missing
          if (!text.startsWith('http://') && !text.startsWith('https://')) {
            text = 'https://' + text
          }
        }
      }

      // Format for SMS and Tel
      if (qrOptions.type === 'sms' && text) {
        text = `sms:${text}`
      }
      if (qrOptions.type === 'tel' && text) {
        text = `tel:${text}`
      }

      const canvas = canvasRef.current
      if (!canvas) return

      await QRCode.toCanvas(canvas, text, {
        errorCorrectionLevel: qrOptions.errorCorrectionLevel,
        width: qrOptions.size,
        margin: qrOptions.margin,
        color: qrOptions.color
      })

      // Create data URL for download
      const dataUrl = canvas.toDataURL('image/png')
      setQrDataUrl(dataUrl)
      setError('')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'QR 码生成失败')
      setQrDataUrl('')
    }
  }

  const downloadQRCode = () => {
    if (!qrDataUrl) return

    const link = document.createElement('a')
    link.download = `qrcode-${Date.now()}.png`
    link.href = qrDataUrl
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
  }

  // Auto-generate on input change
  useEffect(() => {
    if (input || (qrOptions.type === 'wifi' && wifiSsid)) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- 异步生成 QR 码后必须 setState，输入变化即触发
      void generateQRCode()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps -- generateQRCode 依赖全部 input/qrOptions/wifi*，已显式列出
  }, [input, qrOptions, wifiSsid, wifiPassword, wifiSecurity])

  const handleInputChange = (value: string) => {
    setInput(value)
    if (error) setError('')
  }

  const presets = {
    text: [
      { name: '示例文本', value: 'Hello World!' },
      { name: '联系信息', value: '姓名：张三\n电话：138xxxx8888\n邮箱：zhang@example.com' }
    ],
    url: [
      { name: '网站首页', value: 'https://example.com' },
      { name: 'GitHub', value: 'https://github.com' }
    ],
    sms: [
      { name: '发送短信', value: '138xxxx8888' }
    ],
    tel: [
      { name: '拨打电话', value: '138xxxx8888' }
    ]
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">二维码生成工具</h1>
        <p className="text-muted-foreground mt-2">
          在线生成高质量二维码，支持文本、URL、WiFi、电话等多种类型
        </p>
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        {/* Input section */}
        <Card>
          <CardHeader>
            <CardTitle>内容设置</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <label className="text-sm font-medium">类型</label>
              <Select
                value={qrOptions.type}
                onValueChange={(value: any) =>
                  setQROptions(prev => ({ ...prev, type: value }))
                }
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="text">文本</SelectItem>
                  <SelectItem value="url">URL 链接</SelectItem>
                  <SelectItem value="wifi">WiFi 连接</SelectItem>
                  <SelectItem value="sms">短信</SelectItem>
                  <SelectItem value="tel">电话</SelectItem>
                </SelectContent>
              </Select>
            </div>

            {qrOptions.type === 'wifi' ? (
              <div className="space-y-3">
                <div className="space-y-2">
                  <label className="text-sm font-medium">网络名称 (SSID)</label>
                  <Input
                    placeholder="输入 WiFi 网络名称"
                    value={wifiSsid}
                    onChange={(e) => setWifiSsid(e.target.value)}
                  />
                </div>
                <div className="space-y-2">
                  <label className="text-sm font-medium">密码</label>
                  <Input
                    placeholder="输入 WiFi 密码"
                    value={wifiPassword}
                    onChange={(e) => setWifiPassword(e.target.value)}
                    type="password"
                  />
                </div>
                <div className="space-y-2">
                  <label className="text-sm font-medium">安全类型</label>
                  <Select value={wifiSecurity} onValueChange={setWifiSecurity}>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="WPA">WPA/WPA2</SelectItem>
                      <SelectItem value="WEP">WEP</SelectItem>
                      <SelectItem value="nopass">无密码</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              </div>
            ) : (
              <div className="space-y-2">
                <label className="text-sm font-medium">
                  {qrOptions.type === 'url' && 'URL 地址'}
                  {qrOptions.type === 'text' && '文本内容'}
                  {qrOptions.type === 'sms' && '手机号码'}
                  {qrOptions.type === 'tel' && '电话号码'}
                </label>
                <Textarea
                  placeholder={
                    qrOptions.type === 'url'
                      ? '输入网址，如 https://example.com'
                      : qrOptions.type === 'sms'
                      ? '输入手机号码'
                      : qrOptions.type === 'tel'
                      ? '输入电话号码'
                      : '输入文本内容...'
                  }
                  value={input}
                  onChange={(e) => handleInputChange(e.target.value)}
                  className="min-h-[100px]"
                />
              </div>
            )}

            {(presets as any)[qrOptions.type] && (
              <div className="space-y-2">
                <label className="text-sm font-medium">快捷示例</label>
                <div className="space-y-1">
                  {(presets as any)[qrOptions.type].map((preset: any, index: number) => (
                    <button
                      key={index}
                      onClick={() => setInput(preset.value)}
                      className="block w-full text-left px-2 py-1 text-xs bg-muted/50 hover:bg-muted rounded"
                    >
                      {preset.name}: {preset.value}
                    </button>
                  ))}
                </div>
              </div>
            )}

            {error && (
              <Badge variant="destructive">
                错误：{error}
              </Badge>
            )}
          </CardContent>
        </Card>

        {/* QR Code Display */}
        <Card>
          <CardHeader>
            <CardTitle>生成的二维码</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex justify-center p-6 bg-muted/30 rounded border">
              <canvas
                ref={canvasRef}
                className="max-w-full h-auto border"
                style={{ imageRendering: 'pixelated' }}
              />
            </div>

            {qrDataUrl && (
              <div className="flex gap-2">
                <Button onClick={downloadQRCode}>下载 PNG</Button>
                <Button
                  variant="outline"
                  onClick={() => navigator.clipboard.writeText(qrDataUrl)}
                >
                  复制图片数据
                </Button>
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Options */}
      <Card>
        <CardHeader>
          <CardTitle>高级选项</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
            <div className="space-y-2">
              <label className="text-sm font-medium">尺寸</label>
              <Select
                value={qrOptions.size.toString()}
                onValueChange={(value) =>
                  setQROptions(prev => ({ ...prev, size: parseInt(value) }))
                }
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="128">128x128</SelectItem>
                  <SelectItem value="256">256x256</SelectItem>
                  <SelectItem value="512">512x512</SelectItem>
                  <SelectItem value="1024">1024x1024</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <label className="text-sm font-medium">纠错级别</label>
              <Select
                value={qrOptions.errorCorrectionLevel}
                onValueChange={(value: any) =>
                  setQROptions(prev => ({ ...prev, errorCorrectionLevel: value }))
                }
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="L">L (低)</SelectItem>
                  <SelectItem value="M">M (中)</SelectItem>
                  <SelectItem value="Q">Q (四分位)</SelectItem>
                  <SelectItem value="H">H (高)</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <label className="text-sm font-medium">前景色</label>
              <Input
                type="color"
                value={qrOptions.color.dark}
                onChange={(e) =>
                  setQROptions(prev => ({
                    ...prev,
                    color: { ...prev.color, dark: e.target.value }
                  }))
                }
                className="h-10"
              />
            </div>

            <div className="space-y-2">
              <label className="text-sm font-medium">背景色</label>
              <Input
                type="color"
                value={qrOptions.color.light}
                onChange={(e) =>
                  setQROptions(prev => ({
                    ...prev,
                    color: { ...prev.color, light: e.target.value }
                  }))
                }
                className="h-10"
              />
            </div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>使用说明</CardTitle>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground space-y-2">
          <p>• <strong>纠错级别</strong>：L(7%)、M(15%)、Q(25%)、H(30%) - 更高级别能容忍更多损坏</p>
          <p>• <strong>WiFi 二维码</strong>：扫描后可直接连接到指定 WiFi 网络</p>
          <p>• <strong>URL 链接</strong>：自动添加 https:// 协议（如果缺失）</p>
          <p>• <strong>下载</strong>：生成的二维码为 PNG 格式，支持透明背景</p>
        </CardContent>
      </Card>
    </div>
  )
}