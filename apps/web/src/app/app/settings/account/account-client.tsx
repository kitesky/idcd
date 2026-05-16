"use client"

import { useState, useEffect } from "react"
import { useRouter } from "next/navigation"
import Link from "next/link"
import { useTranslations } from "next-intl"
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
} from "@/components/ui"
import { apiRequest } from "@/lib/api"

// ── Schemas ──────────────────────────────────────────────────────────────────

const passwordSchema = z
  .object({
    current_password: z.string().min(1),
    new_password: z.string().min(8),
    confirm_password: z.string().min(1),
  })
  .refine((data) => data.new_password === data.confirm_password, {
    path: ["confirm_password"],
  })

type PasswordFormValues = z.infer<typeof passwordSchema>

// ── AccountClient ─────────────────────────────────────────────────────────

export function AccountClient() {
  const router = useRouter()
  const t = useTranslations("settings")

  // ── Profile / email state ────────────────────────────────────────────────
  const [userEmail, setUserEmail] = useState<string | null>(null)
  const [profileLoading, setProfileLoading] = useState(true)

  useEffect(() => {
    apiRequest<{ data: { email: string } }>("/v1/account/profile")
      .then((res) => setUserEmail(res.data.email))
      .catch(() => {
        // Keep null; delete confirm will remain hidden until loaded
      })
      .finally(() => setProfileLoading(false))
  }, [])

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
      await apiRequest("/v1/account/password", {
        method: "PATCH",
        body: JSON.stringify({
          current_password: values.current_password,
          new_password: values.new_password,
        }),
      })
      setPwdSuccess(true)
      form.reset()
      setTimeout(() => setPwdSuccess(false), 3000)
    } catch (err) {
      setPwdError(err instanceof Error ? err.message : t("account.pwdFailed"))
    } finally {
      setPwdLoading(false)
    }
  }

  // ── Danger zone state ────────────────────────────────────────────────────
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)
  const [deleteEmailInput, setDeleteEmailInput] = useState("")
  const [deleteLoading, setDeleteLoading] = useState(false)
  const [deleteError, setDeleteError] = useState<string | null>(null)

  async function handleDeleteConfirm() {
    if (deleteEmailInput !== userEmail) {
      setDeleteError(t("account.emailMismatch"))
      return
    }
    setDeleteLoading(true)
    setDeleteError(null)
    try {
      await apiRequest("/v1/account", { method: "DELETE" })
      router.push("/auth/logout")
    } catch {
      setDeleteError(t("account.deleteFailed"))
      setDeleteLoading(false)
    }
  }

  return (
    <div data-testid="account-page" className="space-y-6">
      {/* ── Card 1: 修改密码 ─────────────────────────────────────────── */}
      <Card data-testid="password-card">
        <CardHeader>
          <CardTitle>{t("account.changePassword")}</CardTitle>
          <CardDescription>{t("account.changePasswordDesc")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {pwdError && (
            <Alert variant="destructive" data-testid="pwd-error">
              <AlertDescription>{pwdError}</AlertDescription>
            </Alert>
          )}
          {pwdSuccess && (
            <Alert data-testid="pwd-success">
              <AlertDescription>{t("account.pwdUpdated")}</AlertDescription>
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
                    <FormLabel>{t("account.currentPassword")}</FormLabel>
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
                    <FormLabel>{t("account.newPassword")}</FormLabel>
                    <FormControl>
                      <Input
                        type="password"
                        placeholder={t("account.pwdMinLength")}
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
                    <FormLabel>{t("account.confirmPassword")}</FormLabel>
                    <FormControl>
                      <Input
                        type="password"
                        placeholder={t("account.pwdRepeat")}
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
                {pwdLoading ? t("account.saving") : t("account.save")}
              </Button>
            </form>
          </Form>
        </CardContent>
      </Card>

      {/* ── Card 2: 两步验证 ─────────────────────────────────────────── */}
      <Card data-testid="2fa-card">
        <CardHeader>
          <CardTitle>{t("account.twoFactor")}</CardTitle>
          <CardDescription>{t("account.twoFactorDesc")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center gap-3">
            <span className="text-sm text-muted-foreground">{t("account.currentStatus")}</span>
            <Badge variant="secondary" data-testid="2fa-status-badge">
              {t("account.twoFactorDisabled")}
            </Badge>
          </div>

          <Button variant="outline" asChild data-testid="btn-enable-2fa">
            <Link href="/app/settings/security">{t("account.goToSecurity")}</Link>
          </Button>
        </CardContent>
      </Card>

      {/* ── Card 3: 危险区 ───────────────────────────────────────────── */}
      <Card
        className="border-destructive"
        data-testid="danger-zone-card"
      >
        <CardHeader>
          <CardTitle className="text-destructive">{t("account.dangerZone")}</CardTitle>
          <CardDescription>
            {t("account.dangerZoneDesc")}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {profileLoading ? (
            <div className="h-9 w-32 rounded-md bg-muted animate-pulse" data-testid="delete-btn-skeleton" />
          ) : !showDeleteConfirm ? (
            <Button
              variant="destructive"
              data-testid="btn-delete-account"
              onClick={() => setShowDeleteConfirm(true)}
            >
              {t("account.deleteBtn")}
            </Button>
          ) : (
            <div
              className="space-y-3 p-4 border border-destructive rounded-md"
              data-testid="delete-confirm-panel"
            >
              <p className="text-sm font-medium">
                {t("account.confirmDeletePrompt")}
                <span className="font-semibold"> {userEmail} </span>
                {t("account.confirmDeleteSuffix")}
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
                placeholder={t("account.emailPlaceholder")}
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
                  {deleteLoading ? t("account.confirming") : t("account.confirmDelete")}
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
                  {t("account.cancelDelete")}
                </Button>
              </div>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

