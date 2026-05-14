"use client"

import { useState, useEffect } from "react"
import { Card, CardContent, CardHeader, CardTitle, Input, Button, Badge } from "@/components/ui"

export default function TimestampPage() {
  const [timestamp, setTimestamp] = useState('')
  const [datetime, setDatetime] = useState('')
  const [currentTime, setCurrentTime] = useState<number>(0)
  const [output, setOutput] = useState<{
    iso: string
    local: string
    utc: string
    unix_seconds: string
    unix_milliseconds: string
  } | null>(null)
  const [error, setError] = useState('')

  // Update current time every second
  useEffect(() => {
    const updateCurrentTime = () => {
      setCurrentTime(Date.now())
    }

    updateCurrentTime()
    const interval = setInterval(updateCurrentTime, 1000)
    return () => clearInterval(interval)
  }, [])

  const convertFromTimestamp = () => {
    try {
      let ts = parseInt(timestamp.trim())

      if (isNaN(ts)) {
        throw new Error('无效的时间戳格式')
      }

      // Auto-detect seconds vs milliseconds
      // If timestamp is less than year 2001 in seconds, assume it's in milliseconds
      if (ts < 978307200 && ts > 978307200000) {
        // Actually, let's check if it looks like milliseconds (13 digits) or seconds (10 digits)
      }

      // If it looks like seconds (around 10 digits), convert to milliseconds
      if (ts.toString().length <= 10) {
        ts = ts * 1000
      }

      const date = new Date(ts)

      if (isNaN(date.getTime())) {
        throw new Error('无效的时间戳')
      }

      setOutput({
        iso: date.toISOString(),
        local: date.toLocaleString('zh-CN', {
          timeZone: Intl.DateTimeFormat().resolvedOptions().timeZone,
          year: 'numeric',
          month: '2-digit',
          day: '2-digit',
          hour: '2-digit',
          minute: '2-digit',
          second: '2-digit'
        }),
        utc: date.toUTCString(),
        unix_seconds: Math.floor(date.getTime() / 1000).toString(),
        unix_milliseconds: date.getTime().toString()
      })
      setError('')
    } catch (err) {
      setError(err instanceof Error ? err.message : '转换失败')
      setOutput(null)
    }
  }

  const convertFromDatetime = () => {
    try {
      const date = new Date(datetime.trim())

      if (isNaN(date.getTime())) {
        throw new Error('无效的日期时间格式')
      }

      setOutput({
        iso: date.toISOString(),
        local: date.toLocaleString('zh-CN', {
          timeZone: Intl.DateTimeFormat().resolvedOptions().timeZone,
          year: 'numeric',
          month: '2-digit',
          day: '2-digit',
          hour: '2-digit',
          minute: '2-digit',
          second: '2-digit'
        }),
        utc: date.toUTCString(),
        unix_seconds: Math.floor(date.getTime() / 1000).toString(),
        unix_milliseconds: date.getTime().toString()
      })
      setError('')
    } catch (err) {
      setError(err instanceof Error ? err.message : '转换失败')
      setOutput(null)
    }
  }

  const useCurrentTime = () => {
    const now = Date.now()
    setTimestamp(Math.floor(now / 1000).toString())
    convertFromTimestamp()
  }

  const handleTimestampChange = (value: string) => {
    setTimestamp(value)
    if (error) setError('')
  }

  const handleDatetimeChange = (value: string) => {
    setDatetime(value)
    if (error) setError('')
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">时间戳转换工具</h1>
        <p className="text-muted-foreground mt-2">
          Unix 时间戳与标准日期时间的双向转换工具
        </p>
        <div className="mt-2 text-sm text-muted-foreground font-mono">
          当前时间戳：{Math.floor(currentTime / 1000)} ({new Date(currentTime).toLocaleString('zh-CN')})
        </div>
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        {/* Timestamp Input */}
        <Card>
          <CardHeader>
            <CardTitle>时间戳转换</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <label className="text-sm font-medium">Unix 时间戳（秒或毫秒）</label>
              <Input
                placeholder="例如：1703980800 或 1703980800000"
                value={timestamp}
                onChange={(e) => handleTimestampChange(e.target.value)}
                className="font-mono"
              />
            </div>
            <div className="flex gap-2">
              <Button onClick={convertFromTimestamp}>转换</Button>
              <Button onClick={useCurrentTime} variant="outline">当前时间</Button>
            </div>
          </CardContent>
        </Card>

        {/* Datetime Input */}
        <Card>
          <CardHeader>
            <CardTitle>日期时间转换</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <label className="text-sm font-medium">日期时间字符串</label>
              <Input
                placeholder="例如：2023-12-31 08:00:00 或 2023-12-31T08:00:00Z"
                value={datetime}
                onChange={(e) => handleDatetimeChange(e.target.value)}
                className="font-mono"
              />
            </div>
            <Button onClick={convertFromDatetime}>转换</Button>
          </CardContent>
        </Card>
      </div>

      {/* Output */}
      {(output || error) && (
        <Card>
          <CardHeader>
            <CardTitle>转换结果</CardTitle>
          </CardHeader>
          <CardContent>
            {error && (
              <Badge variant="destructive" className="mb-4">
                错误：{error}
              </Badge>
            )}
            {output && (
              <div className="space-y-3">
                <div className="grid gap-3">
                  <div>
                    <label className="text-sm font-medium text-muted-foreground">ISO 8601 格式</label>
                    <div className="font-mono text-sm bg-muted/50 p-2 rounded border">
                      {output.iso}
                    </div>
                  </div>
                  <div>
                    <label className="text-sm font-medium text-muted-foreground">本地时间</label>
                    <div className="font-mono text-sm bg-muted/50 p-2 rounded border">
                      {output.local}
                    </div>
                  </div>
                  <div>
                    <label className="text-sm font-medium text-muted-foreground">UTC 时间</label>
                    <div className="font-mono text-sm bg-muted/50 p-2 rounded border">
                      {output.utc}
                    </div>
                  </div>
                  <div className="grid gap-3 md:grid-cols-2">
                    <div>
                      <label className="text-sm font-medium text-muted-foreground">Unix 时间戳（秒）</label>
                      <div className="font-mono text-sm bg-muted/50 p-2 rounded border">
                        {output.unix_seconds}
                      </div>
                    </div>
                    <div>
                      <label className="text-sm font-medium text-muted-foreground">Unix 时间戳（毫秒）</label>
                      <div className="font-mono text-sm bg-muted/50 p-2 rounded border">
                        {output.unix_milliseconds}
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            )}
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader>
          <CardTitle>使用说明</CardTitle>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground space-y-2">
          <p>• <strong>时间戳输入</strong>：自动识别秒级（10位）或毫秒级（13位）时间戳</p>
          <p>• <strong>日期时间输入</strong>：支持多种格式，如 "2023-12-31 08:00:00" 或 ISO 8601</p>
          <p>• <strong>当前时间</strong>：快速获取当前 Unix 时间戳</p>
          <p>• 输出包含 ISO 8601、本地时间、UTC 时间以及 Unix 时间戳</p>
        </CardContent>
      </Card>
    </div>
  )
}