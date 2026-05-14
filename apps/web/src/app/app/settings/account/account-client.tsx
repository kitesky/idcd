"use client"

import { useState } from "react"
import { useRouter } from "next/navigation"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import * as z from "zod/v3"
import {
  Button,
  Input,
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  Badge,
  Alert,
  AlertDescription,
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui"

// ── Schemas ──────────────────────────────────────────────────────────────────

const passwordSchema = z
  .object({
    current_password: z.string().min(1, { message: "请输入当前密码" }),
    new_password: z
      .string()
      .min(8, { message: "新密码至少 8 位" }),
    confirm_password: z.string().min(1, { message: "请确认新密码" }),
  })
  .refine((data) => data.new_password === data.confirm_password, {
    message: "两次密码不一致",
    path: ["confirm_password"],
  })

type PasswordFormValues = z.infer<typeof passwordSchema>

// ── Mock email (replace with real session data in production) ─────────────
const MOCK_USER_EMAIL = "user@example.com"

// ── AccountClient ─────────────────────────────────────────────────────────

export function AccountClient() {
  const router = useRouter()

  // ── Password card state ──────────────────────────────────────────────────
  const [pwdSuccess, setPwdSuccess] = useState(false)
  const [pwdError, setPwdError] = useState<string | null>(null)
  const [pwdLoading, setPwdLoading] = useState(false)

  const form = useForm<PasswordFormValues>({
    resolver: zodResolver(passwordSchema),
    defaultValues: {
      current_password: "",
      new_password: "",
      confirm_password: "",
    },
  })

  async function onSubmitPassword(values: PasswordFormValues) {
    setPwdLoading(true)
    setPwdError(null)
    setPwdSuccess(false)
    try {
      // Mock: simulate API call (values used in real impl)
      void values
      await new Promise((r) => setTimeout(r, 300))
      setPwdSuccess(true)
      form.reset()
      setTimeout(() => setPwdSuccess(false), 3000)
    } catch {
      setPwdError("密码更新失败，请稍后重试")
    } finally {
      setPwdLoading(false)
    }
  }

  // ── 2FA card state ───────────────────────────────────────────────────────
  const [show2FADialog, setShow2FADialog] = useState(false)

  // ── Danger zone state ────────────────────────────────────────────────────
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)
  const [deleteEmailInput, setDeleteEmailInput] = useState("")
  const [deleteLoading, setDeleteLoading] = useState(false)
  const [deleteError, setDeleteError] = useState<string | null>(null)

  async function handleDeleteConfirm() {
    if (deleteEmailInput !== MOCK_USER_EMAIL) {
      setDeleteError("邮箱地址不匹配，请重新输入")
      return
    }
    setDeleteLoading(true)
    setDeleteError(null)
    try {
      // Mock: real impl calls DELETE /v1/account
      await new Promise((r) => setTimeout(r, 300))
      // Show toast-like message then redirect
      router.push("/auth/logout")
    } catch {
      setDeleteError("提交失败，请稍后重试")
      setDeleteLoading(false)
    }
  }

  return (
    <div data-testid="account-page" className="space-y-6">
      {/* ── Card 1: 修改密码 ─────────────────────────────────────────── */}
      <Card data-testid="password-card">
        <CardHeader>
          <CardTitle>修改密码</CardTitle>
          <CardDescription>定期更换密码以保护账号安全</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {pwdError && (
            <Alert variant="destructive" data-testid="pwd-error">
              <AlertDescription>{pwdError}</AlertDescription>
            </Alert>
          )}
          {pwdSuccess && (
            <Alert data-testid="pwd-success">
              <AlertDescription>密码已更新</AlertDescription>
            </Alert>
          )}

          <Form {...form}>
            <form
              onSubmit={form.handleSubmit(onSubmitPassword)}
              className="space-y-4"
            >
              <FormField
                control={form.control}
                name="current_password"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>当前密码</FormLabel>
                    <FormControl>
                      <Input
                        type="password"
                        placeholder="••••••••"
                        disabled={pwdLoading}
                        data-testid="input-current-password"
                        {...field}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="new_password"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>新密码</FormLabel>
                    <FormControl>
                      <Input
                        type="password"
                        placeholder="至少 8 位"
                        disabled={pwdLoading}
                        data-testid="input-new-password"
                        {...field}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="confirm_password"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>确认新密码</FormLabel>
                    <FormControl>
                      <Input
                        type="password"
                        placeholder="再次输入新密码"
                        disabled={pwdLoading}
                        data-testid="input-confirm-password"
                        {...field}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <Button
                type="submit"
                disabled={pwdLoading}
                data-testid="btn-save-password"
              >
                {pwdLoading ? "保存中..." : "保存"}
              </Button>
            </form>
          </Form>
        </CardContent>
      </Card>

      {/* ── Card 2: 两步验证 ─────────────────────────────────────────── */}
      <Card data-testid="2fa-card">
        <CardHeader>
          <CardTitle>两步验证</CardTitle>
          <CardDescription>为您的账号添加额外的安全保护层</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center gap-3">
            <span className="text-sm text-muted-foreground">当前状态：</span>
            <Badge variant="secondary" data-testid="2fa-status-badge">
              未启用
            </Badge>
          </div>

          <Button
            variant="outline"
            data-testid="btn-enable-2fa"
            onClick={() => setShow2FADialog(true)}
          >
            启用 2FA
          </Button>

          <Dialog open={show2FADialog} onOpenChange={setShow2FADialog}>
            <DialogContent data-testid="2fa-dialog">
              <DialogHeader>
                <DialogTitle>两步验证即将上线</DialogTitle>
                <DialogDescription>
                  两步验证功能即将上线，敬请期待
                </DialogDescription>
              </DialogHeader>
              <DialogFooter>
                <Button data-testid="btn-2fa-dialog-close" onClick={() => setShow2FADialog(false)}>
                  确认
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </CardContent>
      </Card>

      {/* ── Card 3: 危险区 ───────────────────────────────────────────── */}
      <Card
        className="border-destructive"
        data-testid="danger-zone-card"
      >
        <CardHeader>
          <CardTitle className="text-destructive">注销账号</CardTitle>
          <CardDescription>
            注销账号将删除所有数据，此操作不可撤销。删除将在 30
            天后生效，期间可登录取消。
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {!showDeleteConfirm ? (
            <Button
              variant="destructive"
              data-testid="btn-delete-account"
              onClick={() => setShowDeleteConfirm(true)}
            >
              注销账号
            </Button>
          ) : (
            <div
              className="space-y-3 p-4 border border-destructive rounded-md"
              data-testid="delete-confirm-panel"
            >
              <p className="text-sm font-medium">
                请输入您的邮箱地址
                <span className="font-semibold"> {MOCK_USER_EMAIL} </span>
                以确认注销
              </p>

              {deleteError && (
                <p
                  className="text-sm text-destructive"
                  data-testid="delete-error"
                >
                  {deleteError}
                </p>
              )}

              <Input
                type="email"
                placeholder="输入您的邮箱"
                value={deleteEmailInput}
                onChange={(e) => {
                  setDeleteEmailInput(e.target.value)
                  setDeleteError(null)
                }}
                disabled={deleteLoading}
                data-testid="input-delete-email"
              />

              <div className="flex gap-3">
                <Button
                  variant="destructive"
                  onClick={handleDeleteConfirm}
                  disabled={deleteLoading || deleteEmailInput === ""}
                  data-testid="btn-confirm-delete"
                >
                  {deleteLoading ? "提交中..." : "确认注销"}
                </Button>
                <Button
                  variant="outline"
                  onClick={() => {
                    setShowDeleteConfirm(false)
                    setDeleteEmailInput("")
                    setDeleteError(null)
                  }}
                  disabled={deleteLoading}
                  data-testid="btn-cancel-delete"
                >
                  取消
                </Button>
              </div>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

