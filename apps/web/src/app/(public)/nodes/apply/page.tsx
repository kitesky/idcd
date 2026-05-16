"use client"

import { useState } from "react"
import { useTranslations } from "next-intl"
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

const COUNTRY_CODES = ["CN", "US", "JP", "SG", "DE", "OTHER"] as const

const STEP_NUMS = [1, 2, 3] as const

export default function NodeApplyPage() {
  const t = useTranslations("nodes")
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
        setError(t("apply.form.loginRequired"))
        return
      }
      if (!res.ok) {
        const json = await res.json().catch(() => ({}))
        setError(json?.message ?? t("apply.form.submitFailed"))
        return
      }

      setSuccess(true)
    } catch {
      setError(t("apply.form.networkError"))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="min-h-screen bg-background">
      <div className="container mx-auto px-4 py-12 max-w-3xl">
        <div className="mb-10 text-center">
          <Badge variant="secondary" className="mb-3">{t("apply.badge")}</Badge>
          <h1 className="text-3xl font-bold tracking-tight">{t("apply.pageTitle")}</h1>
          <p className="mt-3 text-muted-foreground">
            {t("apply.pageDesc")}
          </p>
        </div>

        <div className="grid grid-cols-3 gap-4 mb-10" data-testid="steps">
          {STEP_NUMS.map((step) => (
            <Card key={step} className="text-center">
              <CardContent className="pt-6 pb-5">
                <div className="text-2xl font-bold text-primary mb-1">{step}</div>
                <div className="text-sm font-medium">{t(`apply.steps.${step}.title`)}</div>
                <div className="text-xs text-muted-foreground mt-1">{t(`apply.steps.${step}.desc`)}</div>
              </CardContent>
            </Card>
          ))}
        </div>

        {success ? (
          <Alert data-testid="success-alert">
            <AlertDescription>
              {t("apply.success")}
            </AlertDescription>
          </Alert>
        ) : (
          <Card>
            <CardHeader>
              <CardTitle>{t("apply.form.title")}</CardTitle>
              <CardDescription>
                {t("apply.form.desc")}
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
                    <Label htmlFor="hostname">{t("apply.form.hostname")}</Label>
                    <Input
                      id="hostname"
                      name="hostname"
                      placeholder={t("apply.form.hostnamePlaceholder")}
                      value={form.hostname}
                      onChange={handleChange}
                      required
                      data-testid="input-hostname"
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="ip_address">{t("apply.form.ipAddress")}</Label>
                    <Input
                      id="ip_address"
                      name="ip_address"
                      placeholder={t("apply.form.ipPlaceholder")}
                      value={form.ip_address}
                      onChange={handleChange}
                      required
                      data-testid="input-ip-address"
                    />
                  </div>
                </div>

                <div className="grid grid-cols-1 gap-5 sm:grid-cols-2">
                  <div className="space-y-2">
                    <Label htmlFor="country">{t("apply.form.country")}</Label>
                    <Select
                      value={form.country}
                      onValueChange={(v) => setForm((p) => ({ ...p, country: v }))}
                      required
                    >
                      <SelectTrigger id="country" data-testid="select-country">
                        <SelectValue placeholder={t("apply.form.countryPlaceholder")} />
                      </SelectTrigger>
                      <SelectContent>
                        {COUNTRY_CODES.map((code) => (
                          <SelectItem key={code} value={code}>
                            {t(`apply.countries.${code}`)}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="city">{t("apply.form.city")}</Label>
                    <Input
                      id="city"
                      name="city"
                      placeholder={t("apply.form.cityPlaceholder")}
                      value={form.city}
                      onChange={handleChange}
                      data-testid="input-city"
                    />
                  </div>
                </div>

                <div className="grid grid-cols-1 gap-5 sm:grid-cols-2">
                  <div className="space-y-2">
                    <Label htmlFor="isp">{t("apply.form.isp")}</Label>
                    <Input
                      id="isp"
                      name="isp"
                      placeholder={t("apply.form.ispPlaceholder")}
                      value={form.isp}
                      onChange={handleChange}
                      data-testid="input-isp"
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="bandwidth_mbps">{t("apply.form.bandwidth")}</Label>
                    <Input
                      id="bandwidth_mbps"
                      name="bandwidth_mbps"
                      type="number"
                      min="1"
                      placeholder={t("apply.form.bandwidthPlaceholder")}
                      value={form.bandwidth_mbps}
                      onChange={handleChange}
                      data-testid="input-bandwidth"
                    />
                  </div>
                </div>

                <div className="space-y-2">
                  <Label htmlFor="motivation">{t("apply.form.motivation")}</Label>
                  <Textarea
                    id="motivation"
                    name="motivation"
                    placeholder={t("apply.form.motivationPlaceholder")}
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
                  {submitting ? t("apply.form.submitting") : t("apply.form.submitButton")}
                </Button>
              </form>
            </CardContent>
          </Card>
        )}
      </div>
    </div>
  )
}
