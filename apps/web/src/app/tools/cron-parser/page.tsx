"use client"

import { useState } from "react"
import { Card, CardContent, CardHeader, CardTitle, Input, Badge } from "@idcd/ui"
import { Cron } from "croner"

export default function CronParserPage() {
  const [expression, setExpression] = useState('')
  const [description, setDescription] = useState('')
  const [nextRuns, setNextRuns] = useState<string[]>([])
  const [error, setError] = useState('')

  const parseCron = (cronExpression: string) => {
    try {
      const trimmed = cronExpression.trim()
      if (!trimmed) {
        setDescription('')
        setNextRuns([])
        setError('')
        return
      }

      const cron = new Cron(trimmed, { maxRuns: 5 })

      // Generate human readable description
      const desc = generateDescription(trimmed)
      setDescription(desc)

      // Get next 5 execution times
      const times: string[] = []
      const now = new Date()

      for (let i = 0; i < 5; i++) {
        const next = cron.nextRun()
        if (next) {
          times.push(next.toLocaleString('zh-CN', {
            year: 'numeric',
            month: '2-digit',
            day: '2-digit',
            hour: '2-digit',
            minute: '2-digit',
            second: '2-digit',
            weekday: 'short'
          }))
        } else {
          break
        }
      }

      setNextRuns(times)
      setError('')
    } catch (err) {
      setError(err instanceof Error ? err.message : '无效的 Cron 表达式')
      setDescription('')
      setNextRuns([])
    }
  }

  const generateDescription = (cronExpr: string): string => {
    const parts = cronExpr.split(' ')

    if (parts.length < 5 || parts.length > 6) {
      throw new Error('Cron 表达式应包含 5 或 6 个字段')
    }

    const [second, minute, hour, day, month, weekday] = parts.length === 6 ? parts : ['0', ...parts]

    let desc = '每'

    // Weekday
    if (weekday && weekday !== '*') {
      const weekdayDesc = parseWeekday(weekday)
      desc += weekdayDesc
    }

    // Month
    if (month && month !== '*') {
      const monthDesc = parseMonth(month)
      desc += monthDesc
    }

    // Day
    if (day && day !== '*') {
      desc += `${day}日`
    }

    // Hour
    if (hour && hour !== '*') {
      if (hour.includes('/')) {
        const [start, interval] = hour.split('/')
        desc += `从${start}点开始每${interval}小时`
      } else if (hour.includes('-')) {
        const [start, end] = hour.split('-')
        desc += `${start}-${end}点`
      } else {
        desc += `${hour}点`
      }
    }

    // Minute
    if (minute && minute !== '*') {
      if (minute.includes('/')) {
        const [start, interval] = minute.split('/')
        desc += `从${start}分开始每${interval}分钟`
      } else if (minute.includes('-')) {
        const [start, end] = minute.split('-')
        desc += `${start}-${end}分`
      } else {
        desc += `${minute}分`
      }
    } else {
      desc += '分钟'
    }

    // Second (if 6-field format)
    if (parts.length === 6 && second !== '0' && second !== '*') {
      desc += `${second}秒`
    }

    return desc + '执行'
  }

  const parseWeekday = (weekday: string): string => {
    const weekdays = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']
    if (weekday.includes(',')) {
      return weekday.split(',').map(d => weekdays[parseInt(d)] || d).join('、')
    }
    if (weekday.includes('-')) {
      const [start, end] = weekday.split('-')
      return `${weekdays[parseInt(start)]}-${weekdays[parseInt(end)]}`
    }
    return weekdays[parseInt(weekday)] || weekday
  }

  const parseMonth = (month: string): string => {
    const months = ['1月', '2月', '3月', '4月', '5月', '6月', '7月', '8月', '9月', '10月', '11月', '12月']
    if (month.includes(',')) {
      return month.split(',').map(m => months[parseInt(m) - 1] || m).join('、')
    }
    return months[parseInt(month) - 1] || `${month}月`
  }

  const handleInputChange = (value: string) => {
    setExpression(value)
    parseCron(value)
  }

  const examples = [
    { name: '每分钟', cron: '* * * * *' },
    { name: '每小时', cron: '0 * * * *' },
    { name: '每天午夜', cron: '0 0 * * *' },
    { name: '每天早上9点', cron: '0 9 * * *' },
    { name: '工作日早上9点', cron: '0 9 * * 1-5' },
    { name: '每周一早上9点', cron: '0 9 * * 1' },
    { name: '每月1号午夜', cron: '0 0 1 * *' },
    { name: '每5分钟', cron: '*/5 * * * *' },
  ]

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">Cron 表达式解析工具</h1>
        <p className="text-muted-foreground mt-2">
          在线解析 Cron 表达式，生成人类可读描述和下次执行时间
        </p>
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        {/* Input section */}
        <Card>
          <CardHeader>
            <CardTitle>Cron 表达式输入</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <label className="text-sm font-medium">
                Cron 表达式（5 或 6 字段）
              </label>
              <Input
                placeholder="例如：0 9 * * 1-5"
                value={expression}
                onChange={(e) => handleInputChange(e.target.value)}
                className="font-mono"
              />
              <p className="text-xs text-muted-foreground">
                格式：[秒] 分 时 日 月 周 （秒字段可选）
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
                    onClick={() => setExpression(example.cron)}
                    className="block w-full text-left px-2 py-1 text-xs bg-muted/50 hover:bg-muted rounded"
                  >
                    <span className="font-medium">{example.name}</span>: <code>{example.cron}</code>
                  </button>
                ))}
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Output section */}
        <Card>
          <CardHeader>
            <CardTitle>解析结果</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            {description && (
              <div className="space-y-2">
                <label className="text-sm font-medium text-muted-foreground">
                  人类可读描述
                </label>
                <div className="p-3 bg-muted/50 rounded border">
                  <p className="text-sm">{description}</p>
                </div>
              </div>
            )}

            {nextRuns.length > 0 && (
              <div className="space-y-2">
                <label className="text-sm font-medium text-muted-foreground">
                  接下来的 {nextRuns.length} 次执行时间
                </label>
                <div className="space-y-1">
                  {nextRuns.map((time, index) => (
                    <div
                      key={index}
                      className="p-2 bg-muted/30 rounded text-sm font-mono"
                    >
                      {index + 1}. {time}
                    </div>
                  ))}
                </div>
              </div>
            )}

            {!expression && (
              <p className="text-muted-foreground text-sm">
                请输入 Cron 表达式查看解析结果
              </p>
            )}
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Cron 格式说明</CardTitle>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground">
          <div className="space-y-3">
            <div>
              <p><strong className="text-foreground">字段格式</strong>（从左到右）：</p>
              <p className="font-mono ml-4">分 时 日 月 周 （标准 5 字段）</p>
              <p className="font-mono ml-4">秒 分 时 日 月 周 （扩展 6 字段）</p>
            </div>

            <div>
              <p><strong className="text-foreground">字段范围</strong>：</p>
              <p className="ml-4">• 秒：0-59，分：0-59，时：0-23</p>
              <p className="ml-4">• 日：1-31，月：1-12，周：0-6（0=周日）</p>
            </div>

            <div>
              <p><strong className="text-foreground">特殊字符</strong>：</p>
              <p className="ml-4">• <code>*</code> 任意值</p>
              <p className="ml-4">• <code>/</code> 间隔（如 */5 表示每5个单位）</p>
              <p className="ml-4">• <code>-</code> 范围（如 1-5 表示1到5）</p>
              <p className="ml-4">• <code>,</code> 列表（如 1,3,5 表示1、3、5）</p>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}