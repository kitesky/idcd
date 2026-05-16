"use client"

import { useEffect, useState } from "react"
import { CheckCircle2, Plus, Server } from "lucide-react"

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
  switch (status) {
    case "pending":
      return <Badge variant="secondary">审核中</Badge>
    case "probation":
      return <Badge variant="outline">试用中</Badge>
    case "active":
      return (
        <Badge className="bg-green-500/15 text-green-700 dark:text-green-400 border-green-500/20">
          已激活
        </Badge>
      )
    case "rejected":
      return <Badge variant="destructive">已拒绝</Badge>
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
      const res = await apiRequest<{ applications: NodeApplication[] }>("/v1/nodes/my-applications")
      setApplications(res.applications ?? [])
    } catch {
      toast.error("无法获取节点申请列表")
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchApplications()
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

      toast.success("申请已提交，我们将在 1-3 个工作日内完成审核")
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
            贡献社区节点即可获得积分：每次心跳 <strong>+1 积分</strong>，节点激活奖励{" "}
            <strong>+200 积分</strong>。
          </p>
          <p className="text-muted-foreground text-sm">
            审核通过后，按节点安装指南完成部署，节点上线后自动开始计入积分。节点申请通过后，您将看到完整的部署指南。通常在 1-3 个工作日内审核完成。
          </p>
        </AlertDescription>
      </Alert>

      {/* 部署指南：仅当有 active 节点时显示 */}
      {applications.some((a) => a.status === "active") && (
        <Card className="border-green-500/30 bg-green-500/5">
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-base text-green-700 dark:text-green-400">
              <CheckCircle2 className="h-4 w-4" />
              您有已批准的节点，请按以下步骤完成部署
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            {/* Step 1: 下载安装 */}
            <div>
              <p className="text-sm font-medium mb-2">1. 下载并安装 Agent</p>
              <pre className="rounded-md bg-muted px-4 py-3 font-mono text-xs overflow-x-auto">
                curl -sL https://get.idcd.com/install.sh | sudo bash
              </pre>
            </div>
            {/* Step 2: 配置 */}
            <div>
              <p className="text-sm font-medium mb-2">2. 配置环境变量</p>
              <pre className="rounded-md bg-muted px-4 py-3 font-mono text-xs overflow-x-auto">
                {`IDCD_API_URL=https://api.idcd.com\nIDCD_ENROLL_TOKEN=<管理员提供的注册令牌>`}
              </pre>
            </div>
            {/* Step 3: 启动 */}
            <div>
              <p className="text-sm font-medium mb-2">3. 启动 Agent</p>
              <pre className="rounded-md bg-muted px-4 py-3 font-mono text-xs overflow-x-auto">
                sudo systemctl start idcd-agent && sudo systemctl enable idcd-agent
              </pre>
            </div>
            <p className="text-xs text-muted-foreground">
              注册令牌需联系管理员获取。部署成功后节点状态将自动更新为在线。
            </p>
          </CardContent>
        </Card>
      )}

      {/* 申请列表 */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between gap-4">
          <div>
            <CardTitle>我的节点申请</CardTitle>
            <CardDescription>管理你提交的社区节点申请</CardDescription>
          </div>
          <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
            <DialogTrigger asChild>
              <Button size="sm">
                <Plus className="mr-1.5 h-4 w-4" />
                申请新节点
              </Button>
            </DialogTrigger>
            <DialogContent className="sm:max-w-lg">
              <DialogHeader>
                <DialogTitle>申请贡献社区节点</DialogTitle>
                <DialogDescription>
                  填写节点信息后提交审核，我们将在 1-3 个工作日内完成审核。
                </DialogDescription>
              </DialogHeader>

              <Form {...form}>
                <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
                  <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                    <FormField
                      control={form.control}
                      name="ip_address"
                      rules={{ required: "请填写 IP 地址" }}
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>
                            IP 地址 <span className="text-destructive">*</span>
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
                      rules={{ required: "请填写主机名" }}
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>
                            主机名 <span className="text-destructive">*</span>
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
                      rules={{ required: "请填写国家代码" }}
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>
                            国家代码 <span className="text-destructive">*</span>
                          </FormLabel>
                          <FormControl>
                            <Input placeholder="CN / US / SG" maxLength={3} {...field} />
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
                          <FormLabel>城市</FormLabel>
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
                          <FormLabel>ISP</FormLabel>
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
                          <FormLabel>带宽（Mbps）</FormLabel>
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
                        <FormLabel>申请原因</FormLabel>
                        <FormControl>
                          <Textarea
                            placeholder="请简要说明贡献节点的动机（可选）"
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
                      取消
                    </Button>
                    <Button type="submit" disabled={submitting}>
                      {submitting ? "提交中…" : "提交申请"}
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
                <TableHead>节点 IP</TableHead>
                <TableHead>国家</TableHead>
                <TableHead>状态</TableHead>
                <TableHead>提交时间</TableHead>
              </TableRow>
            </TableHeader>
            {loading ? (
              <TableSkeleton />
            ) : applications.length === 0 ? (
              <TableBody>
                <TableRow>
                  <TableCell colSpan={4} className="py-12 text-center text-muted-foreground">
                    暂无节点申请，点击右上角「申请新节点」开始贡献
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
