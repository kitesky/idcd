"use client"

import { useState } from "react"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Badge } from "@/components/ui/badge"
import { Alert, AlertDescription } from "@/components/ui/alert"

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

const COUNTRIES = [
  { value: "CN", label: "中国" },
  { value: "US", label: "美国" },
  { value: "JP", label: "日本" },
  { value: "SG", label: "新加坡" },
  { value: "DE", label: "德国" },
  { value: "OTHER", label: "其他" },
]

const STEPS = [
  { step: 1, title: "提交申请", desc: "填写服务器信息和申请理由" },
  { step: 2, title: "14 天观察期", desc: "系统自动检测节点稳定性" },
  { step: 3, title: "加入全球节点池", desc: "通过审核后正式成为节点" },
]

export default function NodeApplyPage() {
  const [form, setForm] = useState({
    hostname: "",
    ip_address: "",
    country: "",
    city: "",
    isp: "",
    bandwidth_mbps: "",
    motivation: "",
  })
  const [submitting, setSubmitting] = useState(false)
  const [success, setSuccess] = useState(false)
  const [error, setError] = useState<string | null>(null)

  function handleChange(
    e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>
  ) {
    setForm((prev) => ({ ...prev, [e.target.name]: e.target.value }))
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError(null)
    setSubmitting(true)

    try {
      const body: Record<string, unknown> = {
        hostname: form.hostname,
        ip_address: form.ip_address,
        country: form.country,
      }
      if (form.city) body.city = form.city
      if (form.isp) body.isp = form.isp
      if (form.bandwidth_mbps) body.bandwidth_mbps = Number(form.bandwidth_mbps)
      if (form.motivation) body.motivation = form.motivation

      const res = await fetch(`${API_BASE}/v1/nodes/apply`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify(body),
      })

      if (res.status === 401) {
        setError("请先登录后再提交申请")
        return
      }
      if (!res.ok) {
        const json = await res.json().catch(() => ({}))
        setError(json?.message ?? "提交失败，请稍后重试")
        return
      }

      setSuccess(true)
    } catch {
      setError("网络错误，请稍后重试")
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="min-h-screen bg-background">
      <div className="container mx-auto px-4 py-12 max-w-3xl">
        <div className="mb-10 text-center">
          <Badge variant="secondary" className="mb-3">社区节点计划</Badge>
          <h1 className="text-3xl font-bold tracking-tight">贡献节点，赚取积分</h1>
          <p className="mt-3 text-muted-foreground">
            将您的服务器加入 idcd 全球监控网络，每次心跳获得积分，可兑换 API 调用额度和监控套餐。
          </p>
        </div>

        <div className="grid grid-cols-3 gap-4 mb-10" data-testid="steps">
          {STEPS.map((s) => (
            <Card key={s.step} className="text-center">
              <CardContent className="pt-6 pb-5">
                <div className="text-2xl font-bold text-primary mb-1">{s.step}</div>
                <div className="text-sm font-medium">{s.title}</div>
                <div className="text-xs text-muted-foreground mt-1">{s.desc}</div>
              </CardContent>
            </Card>
          ))}
        </div>

        {success ? (
          <Alert data-testid="success-alert">
            <AlertDescription>
              申请已提交！我们将在 2 个工作日内审核您的申请，审核结果将通过邮件通知您。
            </AlertDescription>
          </Alert>
        ) : (
          <Card>
            <CardHeader>
              <CardTitle>提交节点申请</CardTitle>
              <CardDescription>
                请填写您的服务器信息，带 * 的字段为必填项
              </CardDescription>
            </CardHeader>
            <CardContent>
              {error && (
                <Alert variant="destructive" className="mb-6" data-testid="error-alert">
                  <AlertDescription>{error}</AlertDescription>
                </Alert>
              )}
              <form onSubmit={handleSubmit} className="space-y-5" data-testid="apply-form">
                <div className="grid grid-cols-1 gap-5 sm:grid-cols-2">
                  <div className="space-y-2">
                    <Label htmlFor="hostname">服务器主机名 *</Label>
                    <Input
                      id="hostname"
                      name="hostname"
                      placeholder="my-server.example.com"
                      value={form.hostname}
                      onChange={handleChange}
                      required
                      data-testid="input-hostname"
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="ip_address">IP 地址 *</Label>
                    <Input
                      id="ip_address"
                      name="ip_address"
                      placeholder="1.2.3.4"
                      value={form.ip_address}
                      onChange={handleChange}
                      required
                      data-testid="input-ip-address"
                    />
                  </div>
                </div>

                <div className="grid grid-cols-1 gap-5 sm:grid-cols-2">
                  <div className="space-y-2">
                    <Label htmlFor="country">所在国家 *</Label>
                    <Select
                      value={form.country}
                      onValueChange={(v) => setForm((p) => ({ ...p, country: v }))}
                      required
                    >
                      <SelectTrigger id="country" data-testid="select-country">
                        <SelectValue placeholder="选择国家" />
                      </SelectTrigger>
                      <SelectContent>
                        {COUNTRIES.map((c) => (
                          <SelectItem key={c.value} value={c.value}>
                            {c.label}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="city">城市</Label>
                    <Input
                      id="city"
                      name="city"
                      placeholder="Beijing"
                      value={form.city}
                      onChange={handleChange}
                      data-testid="input-city"
                    />
                  </div>
                </div>

                <div className="grid grid-cols-1 gap-5 sm:grid-cols-2">
                  <div className="space-y-2">
                    <Label htmlFor="isp">ISP 运营商</Label>
                    <Input
                      id="isp"
                      name="isp"
                      placeholder="China Telecom"
                      value={form.isp}
                      onChange={handleChange}
                      data-testid="input-isp"
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="bandwidth_mbps">带宽 (Mbps)</Label>
                    <Input
                      id="bandwidth_mbps"
                      name="bandwidth_mbps"
                      type="number"
                      min="1"
                      placeholder="100"
                      value={form.bandwidth_mbps}
                      onChange={handleChange}
                      data-testid="input-bandwidth"
                    />
                  </div>
                </div>

                <div className="space-y-2">
                  <Label htmlFor="motivation">申请理由</Label>
                  <Textarea
                    id="motivation"
                    name="motivation"
                    placeholder="请简述您想贡献节点的原因（可选）"
                    value={form.motivation}
                    onChange={handleChange}
                    rows={3}
                    data-testid="input-motivation"
                  />
                </div>

                <Button
                  type="submit"
                  className="w-full"
                  disabled={submitting}
                  data-testid="submit-button"
                >
                  {submitting ? "提交中..." : "提交申请"}
                </Button>
              </form>
            </CardContent>
          </Card>
        )}
      </div>
    </div>
  )
}
