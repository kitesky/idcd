"use client"

import { useState, useEffect, useRef } from "react"
import { useRouter } from "next/navigation"
import { useTranslations } from "next-intl"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import * as z from "zod/v3"
import {
  Alert,
  AlertDescription,
  Avatar,
  AvatarImage,
  AvatarFallback,
  Button,
  Input,
  Textarea,
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
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui"
import { apiRequest } from "@/lib/api"

const MAX_FILE_SIZE = 5 * 1024 * 1024 // 5 MB
const ALLOWED_TYPES = ["image/jpeg", "image/png", "image/gif", "image/webp"]

function getCurrentLocale(): string {
  if (typeof document === "undefined") return "zh"
  const match = document.cookie.match(/(?:^|;\s*)locale=([^;]*)/)
  return match?.[1] === "en" ? "en" : "zh"
}

function LanguageSwitcher() {
  const t = useTranslations("settings")
  // 初始用 "zh" 保证 SSR 与 hydration 一致；客户端挂载后再从 cookie 同步真实值
  const [locale, setLocale] = useState<string>("zh")

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- 客户端挂载后从 cookie 同步实际 locale
    setLocale(getCurrentLocale())
  }, [])

  function handleChange(newLocale: "zh" | "en") {
    document.cookie = `locale=${newLocale};path=/;max-age=31536000`
    setLocale(newLocale)
    window.location.reload()
  }

  return (
    <div className="space-y-2">
      <label className="text-sm font-medium leading-none">{t("profile.language")}</label>
      <p className="text-xs text-muted-foreground">{t("profile.languageDesc")}</p>
      <Select value={locale} onValueChange={(v) => handleChange(v as "zh" | "en")}>
        <SelectTrigger className="w-48" data-testid="language-switcher">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="zh">{t("language.zh")}</SelectItem>
          <SelectItem value="en">{t("language.en")}</SelectItem>
        </SelectContent>
      </Select>
    </div>
  )
}

/**
 * Build a locale-aware zod schema. The validation messages are pulled from the
 * `settings.profile.validation.*` namespace so they reflect the user's
 * selected language. Schema is rebuilt on every render — cheap because zod
 * just wraps validators.
 */
function buildProfileSchema(
  t: ReturnType<typeof useTranslations<"settings">>,
) {
  return z.object({
    email: z.string().email(),
    display_name: z
      .string()
      .max(50, { message: t("profile.validation.displayNameMax", { max: 50 }) })
      .optional(),
    bio: z
      .string()
      .max(500, { message: t("profile.validation.bioMax", { max: 500 }) })
      .optional(),
  })
}

type ProfileFormValues = z.infer<ReturnType<typeof buildProfileSchema>>

interface UserProfile {
  email: string
  display_name?: string
  bio?: string
  avatar_url?: string
}

export default function ProfilePage() {
  const router = useRouter()
  const t = useTranslations("settings")
  const [isLoading, setIsLoading] = useState(false)
  const [isDeleting, setIsDeleting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState(false)
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)

  // Avatar state
  const [avatarUrl, setAvatarUrl] = useState<string | null>(null)
  const [avatarPreview, setAvatarPreview] = useState<string | null>(null)
  const [avatarError, setAvatarError] = useState<string | null>(null)
  const [isUploadingAvatar, setIsUploadingAvatar] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const profileSchema = buildProfileSchema(t)
  const form = useForm<ProfileFormValues>({
    resolver: zodResolver(profileSchema),
    defaultValues: {
      email: "",
      display_name: "",
      bio: "",
    },
  })

  useEffect(() => {
    async function loadProfile() {
      try {
        const res = await apiRequest<{ data: UserProfile }>("/v1/account/profile")
        const profile = res.data
        form.reset({
          email: profile.email,
          display_name: profile.display_name || "",
          bio: profile.bio || "",
        })
        if (profile.avatar_url) {
          setAvatarUrl(profile.avatar_url)
        }
      } catch (err) {
        console.error("Failed to load profile:", err)
        // Cookie may be expired — redirect to login.
        router.push("/auth/login" as any)
      }
    }

    void loadProfile()
  }, [router, form])

  // Derive initials for the Avatar fallback from display_name or email
  // eslint-disable-next-line react-hooks/incompatible-library -- react-hook-form 的 form.watch 返回值不能被 memoized (库限制)
  const displayName = form.watch("display_name")
  const email = form.watch("email")
  const initials = (() => {
    const name = displayName?.trim() || email?.trim() || ""
    if (!name) return "?"
    const parts = name.split(/\s+/)
    if (parts.length >= 2) {
      return (parts[0]![0]! + parts[1]![0]!).toUpperCase()
    }
    return name.slice(0, 2).toUpperCase()
  })()

  function handleAvatarClick() {
    fileInputRef.current?.click()
  }

  async function handleFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return

    // Reset error
    setAvatarError(null)

    // Client-side validation
    if (!ALLOWED_TYPES.includes(file.type)) {
      setAvatarError(t("profile.avatarFormatError"))
      e.target.value = ""
      return
    }
    if (file.size > MAX_FILE_SIZE) {
      setAvatarError(t("profile.avatarSizeError"))
      e.target.value = ""
      return
    }

    // Show local preview immediately
    const previewURL = URL.createObjectURL(file)
    setAvatarPreview(previewURL)

    // Upload to backend
    setIsUploadingAvatar(true)
    try {
      const formData = new FormData()
      formData.append("avatar", file)

      const result = await apiRequest<{ data: { avatar_url: string } }>("/v1/account/avatar", {
        method: "POST",
        body: formData,
        // No Content-Type header needed — apiRequest detects FormData and lets
        // the browser set the multipart/form-data boundary automatically.
      })

      setAvatarUrl(result.data.avatar_url)
      setAvatarPreview(null)
    } catch (err) {
      setAvatarError(err instanceof Error ? err.message : t("profile.avatarFailed"))
      setAvatarPreview(null)
    } finally {
      setIsUploadingAvatar(false)
      e.target.value = ""
    }
  }

  async function onSubmit(values: ProfileFormValues) {
    setIsLoading(true)
    setError(null)
    setSuccess(false)

    try {
      await apiRequest("/v1/account/profile", {
        method: "PATCH",
        body: JSON.stringify({
          display_name: values.display_name || undefined,
          bio: values.bio || undefined,
        }),
      })

      setSuccess(true)
      setTimeout(() => setSuccess(false), 3000)
    } catch (err) {
      setError(err instanceof Error ? err.message : t("profile.saveFailed"))
    } finally {
      setIsLoading(false)
    }
  }

  async function handleDeleteAccount() {
    if (!showDeleteConfirm) {
      setShowDeleteConfirm(true)
      return
    }

    setIsDeleting(true)
    setError(null)

    try {
      await apiRequest("/v1/account", {
        method: "DELETE",
      })

      // Cookie is cleared server-side by the delete-account endpoint.
      router.push("/")
    } catch (err) {
      setError(err instanceof Error ? err.message : t("profile.deleteFailed"))
      setIsDeleting(false)
      setShowDeleteConfirm(false)
    }
  }

  return (
    <main className="flex-1 container max-w-3xl">
      <Card>
        <CardHeader>
          <CardTitle>{t("profile.title")}</CardTitle>
          <CardDescription>{t("profile.desc")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          {error && (
            <Alert variant="destructive">
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}

          {success && (
            <Alert>
              <AlertDescription>{t("profile.saveSuccess")}</AlertDescription>
            </Alert>
          )}

          {/* Avatar upload area */}
          <div className="flex flex-col items-start gap-3">
            <span className="text-sm font-medium leading-none">{t("profile.avatar")}</span>
            <div className="flex items-center gap-4">
              <button
                type="button"
                onClick={handleAvatarClick}
                disabled={isUploadingAvatar}
                className="relative group focus:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded-full"
                aria-label={t("profile.avatarLabel")}
              >
                <Avatar className="h-16 w-16 cursor-pointer ring-2 ring-border group-hover:ring-primary transition-all">
                  <AvatarImage
                    src={avatarPreview ?? avatarUrl ?? undefined}
                    alt={t("profile.avatar")}
                  />
                  <AvatarFallback className="text-lg font-semibold">
                    {initials}
                  </AvatarFallback>
                </Avatar>
                {/* Overlay on hover */}
                <div className="absolute inset-0 bg-black/40 rounded-full opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center pointer-events-none">
                  <span className="text-white text-xs font-medium">
                    {isUploadingAvatar ? t("profile.avatarUploading") : t("profile.avatarChange")}
                  </span>
                </div>
              </button>
              <div className="text-sm text-muted-foreground">
                <p>{t("profile.avatarClickHint")}</p>
                <p>{t("profile.avatarFormatHint")}</p>
              </div>
            </div>
            {/* Hidden file input */}
            <input
              ref={fileInputRef}
              type="file"
              accept="image/jpeg,image/png,image/gif,image/webp"
              className="hidden"
              onChange={handleFileChange}
              aria-label={t("profile.avatarFileLabel")}
            />
            {avatarError && (
              <p className="text-sm text-destructive">{avatarError}</p>
            )}
          </div>

          <Form {...form}>
            <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
              <FormField
                control={form.control}
                name="email"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t("profile.email")}</FormLabel>
                    <FormControl>
                      <Input type="email" disabled {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="display_name"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t("profile.displayName")}</FormLabel>
                    <FormControl>
                      <Input
                        type="text"
                        placeholder={t("profile.displayNamePlaceholder")}
                        disabled={isLoading}
                        {...field}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="bio"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t("profile.bio")}</FormLabel>
                    <FormControl>
                      <Textarea
                        placeholder={t("profile.bioPlaceholder")}
                        disabled={isLoading}
                        {...field}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <Button type="submit" disabled={isLoading}>
                {isLoading ? t("profile.saving") : t("profile.save")}
              </Button>
            </form>
          </Form>

          {/* Language Switcher */}
          <div className="pt-2">
            <LanguageSwitcher />
          </div>

          <div className="pt-6 border-t border-border">
            <h3 className="text-lg font-semibold text-destructive mb-2">{t("profile.dangerZone")}</h3>
            <p className="text-sm text-muted-foreground mb-4">
              {t("profile.dangerZoneDesc")}
            </p>

            {showDeleteConfirm ? (
              <Alert variant="destructive">
                <AlertDescription className="space-y-3">
                  <p className="font-medium">{t("profile.deleteConfirmMsg")}</p>
                  <div className="flex gap-3">
                    <Button
                      variant="destructive"
                      onClick={handleDeleteAccount}
                      disabled={isDeleting}
                    >
                      {isDeleting ? t("profile.deleting") : t("profile.confirmDelete")}
                    </Button>
                    <Button
                      variant="outline"
                      onClick={() => setShowDeleteConfirm(false)}
                      disabled={isDeleting}
                    >
                      {t("profile.cancel")}
                    </Button>
                  </div>
                </AlertDescription>
              </Alert>
            ) : (
              <Button variant="destructive" onClick={handleDeleteAccount}>
                {t("profile.deleteAccount")}
              </Button>
            )}
          </div>
        </CardContent>
      </Card>
    </main>
  )
}
