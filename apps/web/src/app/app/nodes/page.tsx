"use client"

import { useEffect, useState } from "react"
import { CheckCircle2, Plus, Server } from "lucide-react"
import { useTranslations } from "next-intl"

import { Alert, AlertDescription } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Textarea } from "@/components/ui/textarea"
import { toast } from "sonner"
import { useForm } from "react-hook-form"
import { apiRequest } from "@/lib/api"

interface NodeApplication {
  id: string
  user_id: string
  hostname: string
  ip_address: string
  country: string
  city?: string
  isp?: string
  status: "pending" | "probation" | "active" | "rejected"
  created_at: string
  updated_at: string
}

interface ApplyFormValues {
  ip_address: string
  hostname: string
  country: string
  city: string
  isp: string
  bandwidth_mbps: string
  motivation: string
}

function StatusBadge({ status }: { status: NodeApplication["status"] }) {
  const t = useTranslations("nodes.myApplications.statusLabel")
  switch (status) {
    case "pending":
      return <Badge variant="secondary">{t("pending")}</Badge>
    case "probation":
      return <Badge variant="outline">{t("probation")}</Badge>
    case "active":
      return <Badge variant="success">{t("active")}</Badge>
    case "rejected":
      return <Badge variant="destructive">{t("rejected")}</Badge>
    default:
      return <Badge variant="secondary">{status}</Badge>
  }
}

function TableSkeleton() {
  return (
    <TableBody>
      {Array.from({ length: 3 }).map((_, i) => (
        <TableRow key={i}>
          <TableCell>
            <Skeleton className="h-4 w-28" />
          </TableCell>
          <TableCell>
            <Skeleton className="h-4 w-20" />
          </TableCell>
          <TableCell>
            <Skeleton className="h-5 w-16" />
          </TableCell>
          <TableCell>
            <Skeleton className="h-4 w-32" />
          </TableCell>
        </TableRow>
      ))}
    </TableBody>
  )
}

export default function NodesPage() {
  const t = useTranslations("nodes")
  const [applications, setApplications] = useState<NodeApplication[]>([])
  const [loading, setLoading] = useState(true)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [submitting, setSubmitting] = useState(false)

  const form = useForm<ApplyFormValues>({
    defaultValues: {
      ip_address: "",
      hostname: "",
      country: "",
      city: "",
      isp: "",
      bandwidth_mbps: "",
      motivation: "",
    },
  })

  async function fetchApplications() {
    try {
      const res = await apiRequest<{ data: { applications: NodeApplication[] } }>("/v1/nodes/my-applications")
      setApplications(res.data.applications ?? [])
    } catch {
      toast.error(t("myApplications.fetchFailed"))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- fetchApplications 内部 await 后 setState
    void fetchApplications()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  async function onSubmit(values: ApplyFormValues) {
    setSubmitting(true)
    try {
      const body: Record<string, unknown> = {
        ip_address: values.ip_address,
        hostname: values.hostname,
        country: values.country,
      }
      if (values.city) body.city = values.city
      if (values.isp) body.isp = values.isp
      if (values.bandwidth_mbps) body.bandwidth_mbps = parseInt(values.bandwidth_mbps, 10)
      if (values.motivation) body.motivation = values.motivation

      await apiRequest("/v1/nodes/apply", {
        method: "POST",
        body: JSON.stringify(body),
      })

      toast.success(t("myApplications.submitSuccess"))
      form.reset()
      setDialogOpen(false)
      await fetchApplications()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "申请提交失败"
      toast.error(msg)
    } finally {
      setSubmitting(false)
    }
  }

  function formatDate(iso: string) {
    return new Date(iso).toLocaleDateString("zh-CN", {
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
    })
  }

  return (
    <div className="flex flex-col gap-6 p-4 md:p-6">
      {/* 说明区块 */}
      <Alert>
        <Server className="h-4 w-4" />
        <AlertDescription className="space-y-1">
          <p>
            {t("contribute.desc", {
              heartbeat: t("contribute.heartbeatPoints"),
              activation: t("contribute.activationBonus"),
            })}
          </p>
          <p className="text-muted-foreground text-sm">
            {t("contribute.hint")}
          </p>
        </AlertDescription>
      </Alert>

      {/* 部署指南：仅当有 active 节点时显示 */}
      {applications.some((a) => a.status === "active") && (
        <Card className="border-green-500/30 bg-green-500/5">
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-base text-green-700 dark:text-green-400">
              <CheckCircle2 className="h-4 w-4" />
              {t("deploy.title")}
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div>
              <p className="text-sm font-medium mb-2">{t("deploy.step1")}</p>
              <pre className="rounded-md bg-muted px-4 py-3 font-mono text-xs overflow-x-auto">
                curl -sL https://get.idcd.com/install.sh | sudo bash
              </pre>
            </div>
            <div>
              <p className="text-sm font-medium mb-2">{t("deploy.step2")}</p>
              <pre className="rounded-md bg-muted px-4 py-3 font-mono text-xs overflow-x-auto">
                {`IDCD_API_URL=https://api.idcd.com\nIDCD_ENROLL_TOKEN=<管理员提供的注册令牌>`}
              </pre>
            </div>
            <div>
              <p className="text-sm font-medium mb-2">{t("deploy.step3")}</p>
              <pre className="rounded-md bg-muted px-4 py-3 font-mono text-xs overflow-x-auto">
                sudo systemctl start idcd-agent && sudo systemctl enable idcd-agent
              </pre>
            </div>
            <p className="text-xs text-muted-foreground">
              {t("deploy.enrollTokenHint")}
            </p>
          </CardContent>
        </Card>
      )}

      {/* 申请列表 */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between gap-4">
          <div>
            <CardTitle>{t("myApplications.title")}</CardTitle>
            <CardDescription>{t("myApplications.desc")}</CardDescription>
          </div>
          <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
            <DialogTrigger asChild>
              <Button size="sm">
                <Plus className="mr-1.5 h-4 w-4" />
                {t("myApplications.applyNew")}
              </Button>
            </DialogTrigger>
            <DialogContent className="sm:max-w-lg">
              <DialogHeader>
                <DialogTitle>{t("apply2.dialogTitle")}</DialogTitle>
                <DialogDescription>
                  {t("apply2.dialogDesc")}
                </DialogDescription>
              </DialogHeader>

              <Form {...form}>
                <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
                  <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                    <FormField
                      control={form.control}
                      name="ip_address"
                      rules={{ required: t("apply2.fields.ipRequired") }}
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>
                            {t("apply2.fields.ipAddress")} <span className="text-destructive">*</span>
                          </FormLabel>
                          <FormControl>
                            <Input placeholder="1.2.3.4" {...field} />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                    <FormField
                      control={form.control}
                      name="hostname"
                      rules={{ required: t("apply2.fields.hostnameRequired") }}
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>
                            {t("apply2.fields.hostname")} <span className="text-destructive">*</span>
                          </FormLabel>
                          <FormControl>
                            <Input placeholder="node-sg-01.example.com" {...field} />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                    <FormField
                      control={form.control}
                      name="country"
                      rules={{ required: t("apply2.fields.countryCodeRequired") }}
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>
                            {t("apply2.fields.countryCode")} <span className="text-destructive">*</span>
                          </FormLabel>
                          <FormControl>
                            <Input placeholder={t("apply2.fields.countryCodePlaceholder")} maxLength={3} {...field} />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                    <FormField
                      control={form.control}
                      name="city"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>{t("apply2.fields.city")}</FormLabel>
                          <FormControl>
                            <Input placeholder="Singapore" {...field} />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                    <FormField
                      control={form.control}
                      name="isp"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>{t("apply2.fields.isp")}</FormLabel>
                          <FormControl>
                            <Input placeholder="Tencent Cloud" {...field} />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                    <FormField
                      control={form.control}
                      name="bandwidth_mbps"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>{t("apply2.fields.bandwidth")}</FormLabel>
                          <FormControl>
                            <Input
                              type="number"
                              min={1}
                              placeholder="100"
                              {...field}
                            />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  </div>

                  <FormField
                    control={form.control}
                    name="motivation"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t("apply2.fields.motivation")}</FormLabel>
                        <FormControl>
                          <Textarea
                            placeholder={t("apply2.fields.motivationPlaceholder")}
                            rows={3}
                            {...field}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <DialogFooter>
                    <Button
                      type="button"
                      variant="outline"
                      onClick={() => setDialogOpen(false)}
                      disabled={submitting}
                    >
                      {t("apply2.cancel")}
                    </Button>
                    <Button type="submit" disabled={submitting}>
                      {submitting ? t("apply2.submitting") : t("apply2.submit")}
                    </Button>
                  </DialogFooter>
                </form>
              </Form>
            </DialogContent>
          </Dialog>
        </CardHeader>

        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("myApplications.table.ip")}</TableHead>
                <TableHead>{t("myApplications.table.country")}</TableHead>
                <TableHead>{t("myApplications.table.status")}</TableHead>
                <TableHead>{t("myApplications.table.submittedAt")}</TableHead>
              </TableRow>
            </TableHeader>
            {loading ? (
              <TableSkeleton />
            ) : applications.length === 0 ? (
              <TableBody>
                <TableRow>
                  <TableCell colSpan={4} className="py-12 text-center text-muted-foreground">
                    {t("myApplications.empty")}
                  </TableCell>
                </TableRow>
              </TableBody>
            ) : (
              <TableBody>
                {applications.map((app) => (
                  <TableRow key={app.id}>
                    <TableCell className="font-mono text-sm">{app.ip_address}</TableCell>
                    <TableCell>{app.country}</TableCell>
                    <TableCell>
                      <StatusBadge status={app.status} />
                    </TableCell>
                    <TableCell className="text-muted-foreground text-sm">
                      {formatDate(app.created_at)}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            )}
          </Table>
        </CardContent>
      </Card>
    </div>
  )
}
