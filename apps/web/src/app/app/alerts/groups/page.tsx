"use client"

import { useEffect, useState } from "react"
import { Info, Layers, Plus, Trash2 } from "lucide-react"

import { Alert, AlertDescription } from "@/components/ui/alert"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog"
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
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form"
import { Input } from "@/components/ui/input"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { toast } from "sonner"
import { useForm } from "react-hook-form"
import { apiRequest } from "@/lib/api"

// ─── Types ────────────────────────────────────────────────────────────────────

interface AlertGroup {
  id: string
  user_id: string
  name: string
  group_by: string
  group_value: string
  wait_seconds: number
  created_at: string
}

interface CreateGroupFormValues {
  name: string
  group_by: string
  group_value: string
  wait_seconds: string
}

// ─── group_by label map ───────────────────────────────────────────────────────

const GROUP_BY_LABELS: Record<string, string> = {
  monitor_prefix: "监控前缀",
  tag:            "标签",
  type:           "告警类型",
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

function formatDate(iso: string) {
  return new Date(iso).toLocaleString("zh-CN", {
    year:   "numeric",
    month:  "2-digit",
    day:    "2-digit",
    hour:   "2-digit",
    minute: "2-digit",
  })
}

// ─── Create Dialog ────────────────────────────────────────────────────────────

interface CreateGroupDialogProps {
  onCreated: (group: AlertGroup) => void
}

function CreateGroupDialog({ onCreated }: CreateGroupDialogProps) {
  const [open, setOpen] = useState(false)
  const [submitting, setSubmitting] = useState(false)

  const form = useForm<CreateGroupFormValues>({
    defaultValues: {
      name:         "",
      group_by:     "",
      group_value:  "",
      wait_seconds: "60",
    },
  })

  async function onSubmit(values: CreateGroupFormValues) {
    if (!values.name.trim()) {
      form.setError("name", { message: "分组名称不能为空" })
      return
    }
    if (!values.group_by) {
      form.setError("group_by", { message: "请选择分组规则" })
      return
    }
    if (!values.group_value.trim()) {
      form.setError("group_value", { message: "分组值不能为空" })
      return
    }

    const wait = parseInt(values.wait_seconds, 10)
    if (isNaN(wait) || wait <= 0) {
      form.setError("wait_seconds", { message: "合并窗口秒数必须为正整数" })
      return
    }

    setSubmitting(true)
    try {
      const res = await apiRequest<{ data: AlertGroup }>("/v1/alert-groups", {
        method: "POST",
        body: JSON.stringify({
          name:         values.name.trim(),
          group_by:     values.group_by,
          group_value:  values.group_value.trim(),
          wait_seconds: wait,
        }),
      })
      toast.success("告警分组已创建")
      onCreated(res.data)
      form.reset()
      setOpen(false)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "创建失败，请重试"
      toast.error(msg)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button size="sm">
          <Plus className="mr-2 h-4 w-4" />
          新建分组
        </Button>
      </DialogTrigger>

      <DialogContent className="sm:max-w-[480px]">
        <DialogHeader>
          <DialogTitle>新建告警分组</DialogTitle>
          <DialogDescription>
            配置分组规则与合并窗口，相同分组内的告警将在窗口期内合并为一条通知。
          </DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4 pt-2">
            {/* 分组名称 */}
            <FormField
              control={form.control}
              name="name"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>分组名称</FormLabel>
                  <FormControl>
                    <Input placeholder="例：生产环境告警组" {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            {/* 分组规则 */}
            <FormField
              control={form.control}
              name="group_by"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>分组规则</FormLabel>
                  <Select onValueChange={field.onChange} value={field.value}>
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue placeholder="选择分组依据" />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      <SelectItem value="monitor_prefix">监控前缀</SelectItem>
                      <SelectItem value="tag">标签</SelectItem>
                      <SelectItem value="type">告警类型</SelectItem>
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />

            {/* 分组值 */}
            <FormField
              control={form.control}
              name="group_value"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>分组值</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={
                        form.watch("group_by") === "tag"
                          ? "输入标签名称，例：prod"
                          : form.watch("group_by") === "monitor_prefix"
                          ? "输入监控名前缀，例：api-"
                          : "输入告警类型，例：down"
                      }
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    与分组规则对应的具体值（标签名、监控前缀或告警类型）
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            {/* 等待秒数 */}
            <FormField
              control={form.control}
              name="wait_seconds"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>合并窗口秒数</FormLabel>
                  <FormControl>
                    <Input type="number" min={1} {...field} />
                  </FormControl>
                  <FormDescription>
                    在此时间窗口内，同分组的告警将合并为一条通知（默认 60 秒）
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                onClick={() => setOpen(false)}
                disabled={submitting}
              >
                取消
              </Button>
              <Button type="submit" disabled={submitting}>
                {submitting ? "创建中…" : "创建"}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}

// ─── Main Page ────────────────────────────────────────────────────────────────

export default function AlertGroupsPage() {
  const [groups, setGroups] = useState<AlertGroup[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    async function fetchGroups() {
      try {
        const res = await apiRequest<{ data: { items: AlertGroup[] } }>("/v1/alert-groups")
        setGroups(res.data.items ?? [])
      } catch {
        toast.error("加载告警分组失败")
      } finally {
        setLoading(false)
      }
    }
    fetchGroups()
  }, [])

  function handleCreated(group: AlertGroup) {
    setGroups((prev) => [group, ...prev])
  }

  async function handleDelete(id: string) {
    try {
      await apiRequest(`/v1/alert-groups/${id}`, { method: "DELETE" })
      setGroups((prev) => prev.filter((g) => g.id !== id))
      toast.success("告警分组已删除")
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "删除失败，请重试"
      toast.error(msg)
    }
  }

  return (
    <div className="space-y-6">
      {/* Page header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">告警分组</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            将相关监控的告警合并，减少重复通知
          </p>
        </div>
        <CreateGroupDialog onCreated={handleCreated} />
      </div>

      {/* Info banner */}
      <Alert>
        <Info className="h-4 w-4" />
        <AlertDescription>
          告警分组可将相关监控的告警合并，避免重复通知。配置"等待秒数"，在该窗口内同组监控的告警会合并为一条通知。
        </AlertDescription>
      </Alert>

      {/* Groups table */}
      <Card>
        <CardHeader>
          <CardTitle>分组列表</CardTitle>
          <CardDescription>已配置的告警分组规则</CardDescription>
        </CardHeader>
        <CardContent>
          {loading ? (
            <div className="space-y-3">
              {Array.from({ length: 3 }).map((_, i) => (
                <Skeleton key={i} className="h-10 w-full" />
              ))}
            </div>
          ) : groups.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-12 text-center text-muted-foreground">
              <Layers className="mb-3 h-10 w-10 opacity-30" />
              <p className="text-sm font-medium">暂无告警分组</p>
              <p className="mt-1 text-xs">点击"新建分组"配置第一个告警分组规则</p>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>分组名</TableHead>
                  <TableHead>分组规则</TableHead>
                  <TableHead className="text-right">等待秒数</TableHead>
                  <TableHead>创建时间</TableHead>
                  <TableHead className="w-16 text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {groups.map((group) => (
                  <TableRow key={group.id}>
                    <TableCell className="font-medium">{group.name}</TableCell>
                    <TableCell className="text-muted-foreground">
                      {GROUP_BY_LABELS[group.group_by] ?? group.group_by}
                      {": "}
                      <span className="font-mono text-foreground">
                        {group.group_value}
                      </span>
                    </TableCell>
                    <TableCell className="text-right">{group.wait_seconds}s</TableCell>
                    <TableCell className="text-muted-foreground text-sm">
                      {formatDate(group.created_at)}
                    </TableCell>
                    <TableCell className="text-right">
                      <AlertDialog>
                        <AlertDialogTrigger asChild>
                          <Button
                            variant="ghost"
                            size="icon"
                            className="h-8 w-8 text-muted-foreground hover:text-destructive"
                          >
                            <Trash2 className="h-4 w-4" />
                            <span className="sr-only">删除</span>
                          </Button>
                        </AlertDialogTrigger>
                        <AlertDialogContent>
                          <AlertDialogHeader>
                            <AlertDialogTitle>确认删除</AlertDialogTitle>
                            <AlertDialogDescription>
                              将永久删除告警分组「{group.name}」，此操作无法撤销。
                            </AlertDialogDescription>
                          </AlertDialogHeader>
                          <AlertDialogFooter>
                            <AlertDialogCancel>取消</AlertDialogCancel>
                            <AlertDialogAction
                              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                              onClick={() => handleDelete(group.id)}
                            >
                              确认删除
                            </AlertDialogAction>
                          </AlertDialogFooter>
                        </AlertDialogContent>
                      </AlertDialog>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
