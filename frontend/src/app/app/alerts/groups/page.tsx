"use client"

import { useEffect, useState } from "react"
import { Info, Layers, Plus, Trash2 } from "lucide-react"
import { useTranslations, useLocale } from "next-intl"

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
import { apiRequest, ApiError } from "@/lib/api"
import { translateApiError } from "@/lib/api-error"
import { bcp47Of } from "@/i18n/registry"

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

// ─── Helpers ──────────────────────────────────────────────────────────────────

function formatDate(iso: string, bcp47: string) {
  return new Date(iso).toLocaleString(bcp47, {
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
  const t = useTranslations("alerts.groups")
  const tErr = useTranslations()
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
      form.setError("name", { message: t("dialog.nameRequired") })
      return
    }
    if (!values.group_by) {
      form.setError("group_by", { message: t("dialog.groupByRequired") })
      return
    }
    if (!values.group_value.trim()) {
      form.setError("group_value", { message: t("dialog.groupValueRequired") })
      return
    }

    const wait = parseInt(values.wait_seconds, 10)
    if (isNaN(wait) || wait <= 0) {
      form.setError("wait_seconds", { message: t("dialog.waitInvalid") })
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
      toast.success(t("createSuccess"))
      onCreated(res.data)
      form.reset()
      setOpen(false)
    } catch (err: unknown) {
      const msg =
        err instanceof ApiError
          ? translateApiError(err, tErr)
          : err instanceof Error
            ? err.message
            : t("createFailed")
      toast.error(msg)
    } finally {
      setSubmitting(false)
    }
  }

  // eslint-disable-next-line react-hooks/incompatible-library -- react-hook-form 的 form.watch 返回值不能被 memoized (库限制)
  const groupByValue = form.watch("group_by")
  const groupValuePlaceholder =
    groupByValue === "tag"
      ? t("dialog.groupValuePlaceholderTag")
      : groupByValue === "monitor_prefix"
        ? t("dialog.groupValuePlaceholderPrefix")
        : t("dialog.groupValuePlaceholderType")

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button size="sm">
          <Plus className="mr-2 h-4 w-4" />
          {t("create")}
        </Button>
      </DialogTrigger>

      <DialogContent className="sm:max-w-[480px]">
        <DialogHeader>
          <DialogTitle>{t("dialog.title")}</DialogTitle>
          <DialogDescription>{t("dialog.desc")}</DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4 pt-2">
            <FormField
              control={form.control}
              name="name"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("dialog.nameLabel")}</FormLabel>
                  <FormControl>
                    <Input placeholder={t("dialog.namePlaceholder")} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="group_by"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("dialog.groupByLabel")}</FormLabel>
                  <Select onValueChange={field.onChange} value={field.value}>
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue placeholder={t("dialog.groupByPlaceholder")} />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      <SelectItem value="monitor_prefix">{t("groupByLabels.monitor_prefix")}</SelectItem>
                      <SelectItem value="tag">{t("groupByLabels.tag")}</SelectItem>
                      <SelectItem value="type">{t("groupByLabels.type")}</SelectItem>
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="group_value"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("dialog.groupValueLabel")}</FormLabel>
                  <FormControl>
                    <Input placeholder={groupValuePlaceholder} {...field} />
                  </FormControl>
                  <FormDescription>{t("dialog.groupValueHint")}</FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="wait_seconds"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("dialog.waitLabel")}</FormLabel>
                  <FormControl>
                    <Input type="number" min={1} {...field} />
                  </FormControl>
                  <FormDescription>{t("dialog.waitHint")}</FormDescription>
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
                {t("dialog.cancel")}
              </Button>
              <Button type="submit" disabled={submitting}>
                {submitting ? t("dialog.creating") : t("dialog.create")}
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
  const t = useTranslations("alerts.groups")
  const tErr = useTranslations()
  const locale = useLocale()
  const bcp47 = bcp47Of(locale)
  const [groups, setGroups] = useState<AlertGroup[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    async function fetchGroups() {
      try {
        const res = await apiRequest<{ data: { items: AlertGroup[] } }>("/v1/alert-groups")
        setGroups(res.data.items ?? [])
      } catch {
        toast.error(t("loadFailed"))
      } finally {
        setLoading(false)
      }
    }
    fetchGroups()
  }, [t])

  function handleCreated(group: AlertGroup) {
    setGroups((prev) => [group, ...prev])
  }

  async function handleDelete(id: string) {
    try {
      await apiRequest(`/v1/alert-groups/${id}`, { method: "DELETE" })
      setGroups((prev) => prev.filter((g) => g.id !== id))
      toast.success(t("deleteSuccess"))
    } catch (err: unknown) {
      const msg =
        err instanceof ApiError
          ? translateApiError(err, tErr)
          : err instanceof Error
            ? err.message
            : t("deleteFailed")
      toast.error(msg)
    }
  }

  function groupByLabel(key: string): string {
    const known = ["monitor_prefix", "tag", "type"]
    if (known.includes(key)) return t(`groupByLabels.${key}` as never)
    return key
  }

  return (
    <div className="space-y-6">
      {/* Page header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{t("title")}</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            {t("subtitle")}
          </p>
        </div>
        <CreateGroupDialog onCreated={handleCreated} />
      </div>

      {/* Info banner */}
      <Alert>
        <Info className="h-4 w-4" />
        <AlertDescription>{t("infoBanner")}</AlertDescription>
      </Alert>

      {/* Groups table */}
      <Card>
        <CardHeader>
          <CardTitle>{t("listTitle")}</CardTitle>
          <CardDescription>{t("listDescription")}</CardDescription>
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
              <p className="text-sm font-medium">{t("empty")}</p>
              <p className="mt-1 text-xs">{t("emptyHint")}</p>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("table.name")}</TableHead>
                  <TableHead>{t("table.rule")}</TableHead>
                  <TableHead className="text-right">{t("table.wait")}</TableHead>
                  <TableHead>{t("table.createdAt")}</TableHead>
                  <TableHead className="w-16 text-right">{t("table.actions")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {groups.map((group) => (
                  <TableRow key={group.id}>
                    <TableCell className="font-medium">{group.name}</TableCell>
                    <TableCell className="text-muted-foreground">
                      {groupByLabel(group.group_by)}
                      {": "}
                      <span className="font-mono text-foreground">
                        {group.group_value}
                      </span>
                    </TableCell>
                    <TableCell className="text-right">{group.wait_seconds}s</TableCell>
                    <TableCell className="text-muted-foreground text-sm">
                      {formatDate(group.created_at, bcp47)}
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
                            <span className="sr-only">{t("delete.label")}</span>
                          </Button>
                        </AlertDialogTrigger>
                        <AlertDialogContent>
                          <AlertDialogHeader>
                            <AlertDialogTitle>{t("delete.title")}</AlertDialogTitle>
                            <AlertDialogDescription>
                              {t("delete.desc", { name: group.name })}
                            </AlertDialogDescription>
                          </AlertDialogHeader>
                          <AlertDialogFooter>
                            <AlertDialogCancel>{t("delete.cancel")}</AlertDialogCancel>
                            <AlertDialogAction
                              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                              onClick={() => handleDelete(group.id)}
                            >
                              {t("delete.confirm")}
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
