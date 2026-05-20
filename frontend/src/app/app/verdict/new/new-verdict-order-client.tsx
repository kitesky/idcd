"use client"

import { useMemo, useState } from "react"
import { useRouter } from "next/navigation"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import * as z from "zod/v3"
import { toast } from "sonner"
import { AlertCircle, ArrowRight, ExternalLink } from "lucide-react"

import {
  Alert,
  AlertDescription,
  AlertTitle,
  Button,
  Card,
  CardContent,
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
  Input,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui"
import {
  createVerdictOrder,
  VERDICT_TEMPLATE_LABELS,
  type VerdictTemplate,
} from "@/lib/api/verdict"

/**
 * Payment channels. Read from env so ops can flip channels without a rebuild;
 * fall back to the v2 default trio (PaymentHub / Alipay / WeChat).
 */
function getChannels(): string[] {
  const raw = process.env.NEXT_PUBLIC_PAYMENT_CHANNELS
  if (!raw) return ["paymenthub", "alipay", "wechat"]
  return raw
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean)
}

const CHANNEL_LABELS: Record<string, string> = {
  paymenthub: "PaymentHub（信用卡 / PayPal）",
  alipay: "支付宝",
  wechat: "微信支付",
}

const TEMPLATES: VerdictTemplate[] = ["sla", "incident", "compliance", "legal"]

// `datetime-local` inputs emit values like "2026-05-17T10:00".
const DATETIME_LOCAL = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}(:\d{2})?$/

const schema = z
  .object({
    template: z.enum(["sla", "incident", "compliance", "legal"] as const, {
      errorMap: () => ({ message: "请选择报告模板" }),
    }),
    target: z
      .string()
      .min(1, { message: "请填写目标域名 / URL / IP" })
      .max(255, { message: "目标长度不超过 255 字符" }),
    start: z.string().regex(DATETIME_LOCAL, { message: "请选择起始时间" }),
    end: z.string().regex(DATETIME_LOCAL, { message: "请选择结束时间" }),
    channel: z.string().min(1, { message: "请选择支付渠道" }),
  })
  .refine((data) => new Date(data.start).getTime() < new Date(data.end).getTime(), {
    message: "结束时间必须晚于起始时间",
    path: ["end"],
  })

type FormValues = z.infer<typeof schema>

/**
 * Default time window = the most recent 24h, rounded down to the nearest minute.
 * Pre-filling reduces friction for the most common Verdict use case (incident
 * postmortem covering the last day).
 */
function defaultWindow(): { start: string; end: string } {
  const now = new Date()
  now.setSeconds(0, 0)
  const start = new Date(now.getTime() - 24 * 60 * 60 * 1000)
  return {
    start: toLocalInputValue(start),
    end: toLocalInputValue(now),
  }
}

function toLocalInputValue(d: Date): string {
  const pad = (n: number) => n.toString().padStart(2, "0")
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(
    d.getHours(),
  )}:${pad(d.getMinutes())}`
}

export function NewVerdictOrderClient() {
  const router = useRouter()
  const channels = useMemo(() => getChannels(), [])
  const [submitError, setSubmitError] = useState<string | null>(null)
  const [pendingCheckout, setPendingCheckout] = useState<{ url: string; orderId: string } | null>(
    null,
  )

  const defaults = useMemo(() => defaultWindow(), [])

  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      template: "incident" as VerdictTemplate,
      target: "",
      start: defaults.start,
      end: defaults.end,
      channel: channels[0] ?? "paymenthub",
    },
  })

  const isSubmitting = form.formState.isSubmitting

  async function onSubmit(values: FormValues) {
    setSubmitError(null)

    // Build the return URL so PaymentHub (or any channel) can redirect back
    // to the order detail page once payment completes. The backend uses
    // it as the `success_url` of the hosted checkout.
    const origin = typeof window !== "undefined" ? window.location.origin : ""

    try {
      const result = await createVerdictOrder({
        template: values.template,
        target: values.target.trim(),
        time_window_start: new Date(values.start).toISOString(),
        time_window_end: new Date(values.end).toISOString(),
        channel: values.channel,
        return_url: `${origin}/app/verdict/{order_id}`,
      })

      toast.success("订单已创建，正在跳转支付页…")

      if (typeof window !== "undefined" && result.pay_url) {
        setPendingCheckout({ url: result.pay_url, orderId: result.order_id })
        window.location.assign(result.pay_url)
      } else {
        router.push(`/app/verdict/${result.order_id}` as never)
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : "创建订单失败"
      setSubmitError(message)
      toast.error("创建订单失败：" + message)
    }
  }

  return (
    <div className="max-w-2xl">
      <Form {...form}>
        <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-6">
          <Card>
            <CardContent className="space-y-6 pt-6">
              <FormField
                control={form.control}
                name="template"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>报告模板</FormLabel>
                    <Select onValueChange={field.onChange} value={field.value}>
                      <FormControl>
                        <SelectTrigger data-testid="template-select">
                          <SelectValue placeholder="选择模板" />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        {TEMPLATES.map((t) => (
                          <SelectItem key={t} value={t}>
                            {VERDICT_TEMPLATE_LABELS[t]}（{t}）
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <FormDescription>
                      模板决定报告页面结构、章节顺序和披露的指标集合。
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="target"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>目标</FormLabel>
                    <FormControl>
                      <Input
                        data-testid="target-input"
                        placeholder="example.com、https://example.com/path 或 1.2.3.4"
                        autoComplete="off"
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      支持域名、URL 或 IP。仅可对你已通过所有权验证的目标下单。
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <FormField
                  control={form.control}
                  name="start"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>起始时间</FormLabel>
                      <FormControl>
                        <Input
                          data-testid="start-input"
                          type="datetime-local"
                          {...field}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name="end"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>结束时间</FormLabel>
                      <FormControl>
                        <Input
                          data-testid="end-input"
                          type="datetime-local"
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
                name="channel"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>支付渠道</FormLabel>
                    <Select onValueChange={field.onChange} value={field.value}>
                      <FormControl>
                        <SelectTrigger data-testid="channel-select">
                          <SelectValue placeholder="选择渠道" />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        {channels.map((c) => (
                          <SelectItem key={c} value={c}>
                            {CHANNEL_LABELS[c] ?? c}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </CardContent>
          </Card>

          {submitError && (
            <Alert variant="destructive" data-testid="submit-error">
              <AlertCircle className="h-4 w-4" />
              <AlertTitle>无法创建订单</AlertTitle>
              <AlertDescription>{submitError}</AlertDescription>
            </Alert>
          )}

          {pendingCheckout && (
            <Alert data-testid="checkout-fallback">
              <AlertTitle>未自动跳转支付页？</AlertTitle>
              <AlertDescription className="space-y-2">
                <p>
                  浏览器可能拦截了跳转。订单号
                  <code className="mx-1 rounded bg-muted px-1 py-0.5 text-xs">
                    {pendingCheckout.orderId}
                  </code>
                  已创建，请点击下方按钮继续支付：
                </p>
                <Button asChild variant="outline" size="sm">
                  <a href={pendingCheckout.url} target="_blank" rel="noreferrer noopener">
                    打开支付页 <ExternalLink className="ml-2 h-3.5 w-3.5" />
                  </a>
                </Button>
              </AlertDescription>
            </Alert>
          )}

          <div className="flex items-center justify-end gap-3">
            <Button
              type="button"
              variant="outline"
              onClick={() => router.back()}
              disabled={isSubmitting}
            >
              取消
            </Button>
            <Button type="submit" data-testid="submit-btn" disabled={isSubmitting}>
              {isSubmitting ? "创建中…" : "创建订单"}
              <ArrowRight className="ml-2 h-4 w-4" />
            </Button>
          </div>
        </form>
      </Form>
    </div>
  )
}
