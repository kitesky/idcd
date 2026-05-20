"use client"

import { useState, useEffect } from "react"
import { useTranslations, useLocale } from "next-intl"
import QRCode from "qrcode"
import { bcp47Of } from "@/i18n/registry"
import {
  Button,
  Input,
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
  Skeleton,
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui"
import { apiRequest } from "@/lib/api"

// detectPasskeyPlatform returns a short, human-friendly label for the current
// device/browser so a newly registered passkey isn't named "My Passkey" for
// every device. Pure best-effort UA sniff — we only use it as a default name
// the server stores; the user can rename later.
// bufToBase64Url base64url-encodes an ArrayBuffer using a byte-by-byte loop.
// Spreading a Uint8Array into String.fromCharCode passes each byte as a
// separate function argument; attestationObject for FIDO U2F security keys
// carries a ~2KB X.509 cert chain and can grow past V8's ~64K argument limit,
// at which point spread throws "Maximum call stack size exceeded".
function bufToBase64Url(buf: ArrayBuffer): string {
  const bytes = new Uint8Array(buf)
  let binary = ""
  for (let i = 0; i < bytes.length; i++) binary += String.fromCharCode(bytes[i]!)
  return btoa(binary)
    .replace(/\+/g, "-")
    .replace(/\//g, "_")
    .replace(/=/g, "")
}

function detectPasskeyPlatform(): string {
  if (typeof navigator === "undefined") return "Unknown"
  const ua = navigator.userAgent
  if (/iPad/.test(ua)) return "iPad"
  if (/iPhone/.test(ua)) return "iPhone"
  if (/Macintosh/.test(ua)) return "Mac"
  if (/Android/.test(ua)) return "Android"
  if (/Windows/.test(ua)) return "Windows"
  if (/CrOS/.test(ua)) return "ChromeOS"
  if (/Linux/.test(ua)) return "Linux"
  return "Unknown"
}

type Step = "idle" | "scan" | "verify" | "backup"

type PasskeyItem = {
  id: string
  device_name: string
  created_at: string
  last_used_at?: string | null
}

export function SecurityClient() {
  const t = useTranslations("settings")
  const locale = useLocale()
  const bcp47 = bcp47Of(locale)
  const [enabled, setEnabled] = useState(false)
  // Passkey delete is irreversible (no soft-delete), so gate behind a typed
  // confirmation dialog instead of one-click. `null` means no pending delete.
  const [passkeyDeleteTarget, setPasskeyDeleteTarget] = useState<string | null>(null)
  const [step, setStep] = useState<Step>("idle")
  const [code, setCode] = useState("")
  const [codeError, setCodeError] = useState<string | null>(null)
  const [disableCode, setDisableCode] = useState("")
  const [disableError, setDisableError] = useState<string | null>(null)
  const [showDisableDialog, setShowDisableDialog] = useState(false)

  const [secretData, setSecretData] = useState<{ secret: string; otpauth_uri: string } | null>(null)
  // qrDataURL holds a locally-rendered TOTP QR code (data: URL). Previously we
  // shipped the otpauth:// secret to api.qrserver.com — a third-party origin —
  // which is equivalent to leaking the TOTP private key. Rendering in-browser
  // keeps the secret on-device.
  const [qrDataURL, setQrDataURL] = useState<string>("")
  const [backupCodes, setBackupCodes] = useState<string[]>([])
  const [setupLoading, setSetupLoading] = useState(false)
  const [verifyLoading, setVerifyLoading] = useState(false)
  const [disableLoading, setDisableLoading] = useState(false)

  const [passkeys, setPasskeys] = useState<PasskeyItem[]>([])
  const [passkeyLoading, setPasskeyLoading] = useState(false)
  const [passkeyAdding, setPasskeyAdding] = useState(false)
  const [passkeyError, setPasskeyError] = useState<string | null>(null)

  useEffect(() => {
    apiRequest<{ data: { enabled: boolean } }>("/v1/account/2fa/status")
      .then((res) => setEnabled(res.data.enabled))
      .catch(() => {
        // Silently ignore — leave default false; server may be unavailable during dev/test
      })
  }, [])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- 初次挂载重置 loading，随后异步 fetch
    setPasskeyLoading(true)
    apiRequest<{ data: { passkeys: PasskeyItem[] } }>("/v1/account/passkeys")
      .then((res) => setPasskeys(res.data.passkeys ?? []))
      .catch(() => {
        // Silently ignore — leave empty list; server may be unavailable during dev/test
      })
      .finally(() => setPasskeyLoading(false))
  }, [])

  async function openSetup() {
    setStep("scan")
    setCode("")
    setCodeError(null)
    setSecretData(null)
    setQrDataURL("")
    setSetupLoading(true)
    try {
      const res = await apiRequest<{ data: { secret: string; otpauth_uri: string } }>("/v1/account/2fa/setup", {
        method: "POST",
      })
      setSecretData(res.data)
      // Render QR locally so the otpauth_uri (containing the TOTP secret) never
      // leaves the browser. errorCorrectionLevel: 'M' keeps the QR readable
      // even when the secret string is long.
      try {
        const dataURL = await QRCode.toDataURL(res.data.otpauth_uri, {
          errorCorrectionLevel: "M",
          margin: 1,
          width: 200,
        })
        setQrDataURL(dataURL)
      } catch {
        // QR rendering failed; user can still type the secret manually.
      }
    } catch (err) {
      setCodeError(err instanceof Error ? err.message : t("security.setupFailed"))
    } finally {
      setSetupLoading(false)
    }
  }

  async function handleVerify() {
    if (code.length !== 6 || !/^\d+$/.test(code)) {
      setCodeError(t("security.codeError"))
      return
    }
    setVerifyLoading(true)
    setCodeError(null)
    try {
      const res = await apiRequest<{ data: { backup_codes: string[] } }>("/v1/account/2fa/verify", {
        method: "POST",
        body: JSON.stringify({ code }),
      })
      setBackupCodes(res.data.backup_codes)
      setStep("backup")
    } catch (err) {
      setCodeError(err instanceof Error ? err.message : t("security.verifyFailed"))
    } finally {
      setVerifyLoading(false)
    }
  }

  function handleFinish() {
    setEnabled(true)
    setStep("idle")
    setCode("")
    setCodeError(null)
    setBackupCodes([])
    setSecretData(null)
  }

  async function handleDisable() {
    if (disableCode.length !== 6 || !/^\d+$/.test(disableCode)) {
      setDisableError(t("security.codeError"))
      return
    }
    setDisableLoading(true)
    setDisableError(null)
    try {
      await apiRequest("/v1/account/2fa/disable", {
        method: "POST",
        body: JSON.stringify({ code: disableCode }),
      })
      setEnabled(false)
      setShowDisableDialog(false)
      setDisableCode("")
      setDisableError(null)
    } catch (err) {
      setDisableError(err instanceof Error ? err.message : t("security.disableFailed"))
    } finally {
      setDisableLoading(false)
    }
  }

  async function handleAddPasskey() {
    setPasskeyAdding(true)
    setPasskeyError(null)
    try {
      const { data } = await apiRequest<{ data: { options: { challenge: string; user: { id: string; [key: string]: unknown }; [key: string]: unknown }; challenge_id: string; [key: string]: unknown } }>("/v1/account/passkeys/register/begin", {
        method: "POST",
      })

      if (typeof window === "undefined" || !window.PublicKeyCredential) {
        throw new Error(t("security.passkeyNotSupported"))
      }

      const credential = await navigator.credentials.create({
        publicKey: {
          ...data.options,
          challenge: Uint8Array.from(atob(data.options.challenge.replace(/-/g, "+").replace(/_/g, "/")), c => c.charCodeAt(0)),
          user: {
            ...data.options.user,
            id: Uint8Array.from(atob(data.options.user.id.replace(/-/g, "+").replace(/_/g, "/")), c => c.charCodeAt(0)),
          },
        } as unknown as PublicKeyCredentialCreationOptions,
      }) as PublicKeyCredential | null

      if (!credential) throw new Error(t("security.passkeyRegisterCancelled"))

      const response = credential.response as AuthenticatorAttestationResponse
      const { data: result } = await apiRequest<{ data: { credential_id: string; device_name: string } }>("/v1/account/passkeys/register/complete", {
        method: "POST",
        body: JSON.stringify({
          challenge: data.challenge_id,
          response: {
            id: credential.id,
            rawId: bufToBase64Url(credential.rawId),
            response: {
              clientDataJSON: bufToBase64Url(response.clientDataJSON),
              attestationObject: bufToBase64Url(response.attestationObject),
            },
          },
          device_name: t("security.passkeyDefaultName", { platform: detectPasskeyPlatform() }),
        }),
      })
      setPasskeys(prev => [...prev, {
        id: result.credential_id,
        device_name: result.device_name,
        created_at: new Date().toISOString(),
        last_used_at: null,
      }])
    } catch (err) {
      setPasskeyError(err instanceof Error ? err.message : t("security.passkeyAddFailed"))
    } finally {
      setPasskeyAdding(false)
    }
  }

  async function handleDeletePasskey(id: string) {
    setPasskeyError(null)
    try {
      await apiRequest(`/v1/account/passkeys/${id}`, { method: "DELETE" })
      setPasskeys(prev => prev.filter(p => p.id !== id))
    } catch (err) {
      setPasskeyError(err instanceof Error ? err.message : t("security.passkeyDeleteFailed"))
    }
  }

  return (
    <div data-testid="security-page" className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">{t("security.title")}</h1>
        <p className="text-muted-foreground text-sm mt-1">{t("security.desc")}</p>
      </div>

      <Card data-testid="2fa-card">
        <CardHeader>
          <CardTitle>{t("security.twoFactor")}</CardTitle>
          <CardDescription>
            {t("security.twoFactorDesc")}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center gap-3">
            <span className="text-sm text-muted-foreground">{t("security.currentStatus")}</span>
            {enabled ? (
              <Badge data-testid="2fa-status-badge">{t("security.twoFactorEnabled")}</Badge>
            ) : (
              <Badge variant="secondary" data-testid="2fa-status-badge">{t("security.twoFactorDisabled")}</Badge>
            )}
          </div>

          {!enabled ? (
            <Button
              variant="outline"
              data-testid="btn-enable-2fa"
              onClick={openSetup}
              disabled={setupLoading}
            >
              {setupLoading ? t("security.loading") : t("security.enable2fa")}
            </Button>
          ) : (
            <Button
              variant="destructive"
              data-testid="btn-disable-2fa"
              onClick={() => {
                setDisableCode("")
                setDisableError(null)
                setShowDisableDialog(true)
              }}
            >
              {t("security.disable2fa")}
            </Button>
          )}
        </CardContent>
      </Card>

      <Card data-testid="passkey-card">
        <CardHeader>
          <CardTitle>{t("security.passkey")}</CardTitle>
          <CardDescription>
            {t("security.passkeyDesc")}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {passkeyError && (
            <Alert variant="destructive" data-testid="passkey-error">
              <AlertDescription>{passkeyError}</AlertDescription>
            </Alert>
          )}
          <Button
            variant="outline"
            data-testid="btn-add-passkey"
            onClick={handleAddPasskey}
            disabled={passkeyAdding || passkeyLoading}
          >
            {passkeyAdding ? t("security.addingPasskey") : t("security.addPasskey")}
          </Button>
          {passkeyLoading && (
            <div className="space-y-2" data-testid="passkey-loading">
              {[1, 2].map((i) => (
                <div key={i} className="flex items-center justify-between py-2">
                  <div className="space-y-1.5">
                    <Skeleton className="h-3 w-36" />
                    <Skeleton className="h-2 w-24" />
                  </div>
                  <Skeleton className="h-8 w-12" />
                </div>
              ))}
            </div>
          )}
          {!passkeyLoading && passkeys.length > 0 && (
            <div className="space-y-2" data-testid="passkey-list">
              {passkeys.map((pk) => (
                <Card key={pk.id} data-testid={`passkey-item-${pk.id}`}>
                  <CardContent className="flex items-center justify-between py-3 px-4">
                    <div className="space-y-0.5">
                      <p className="text-sm font-medium">{pk.device_name}</p>
                      <p className="text-xs text-muted-foreground">
                        {t("security.addedAt")} {new Date(pk.created_at).toLocaleDateString(bcp47)}
                      </p>
                    </div>
                    <Button
                      variant="ghost"
                      size="sm"
                      data-testid={`btn-delete-passkey-${pk.id}`}
                      onClick={() => setPasskeyDeleteTarget(pk.id)}
                    >
                      {t("security.deletePasskey")}
                    </Button>
                  </CardContent>
                </Card>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Passkey delete confirmation. Mirrors the AlertDialog used elsewhere
          (sessions revoke-all etc.) — passkey deletion is irreversible. */}
      <AlertDialog
        open={passkeyDeleteTarget !== null}
        onOpenChange={(open) => { if (!open) setPasskeyDeleteTarget(null) }}
      >
        <AlertDialogContent data-testid="passkey-delete-confirm-dialog">
          <AlertDialogHeader>
            <AlertDialogTitle>{t("security.passkeyConfirmDeleteTitle")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("security.passkeyConfirmDeleteDesc")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel data-testid="btn-cancel-passkey-delete">
              {t("security.passkeyCancelDelete")}
            </AlertDialogCancel>
            <AlertDialogAction
              data-testid="btn-confirm-passkey-delete"
              onClick={() => {
                const id = passkeyDeleteTarget
                setPasskeyDeleteTarget(null)
                if (id) void handleDeletePasskey(id)
              }}
            >
              {t("security.passkeyConfirmDelete")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Setup Dialog — 3 steps */}
      <Dialog open={step !== "idle"} onOpenChange={(open) => { if (!open) setStep("idle") }}>
        <DialogContent data-testid="2fa-setup-dialog">
          {step === "scan" && (
            <>
              <DialogHeader>
                <DialogTitle>{t("security.step1Title")}</DialogTitle>
                <DialogDescription>
                  {t("security.step1Desc")}
                </DialogDescription>
              </DialogHeader>
              <div className="flex flex-col items-center gap-4 py-2">
                {setupLoading || !qrDataURL ? (
                  <div className="flex items-center justify-center w-[200px] h-[200px] text-sm text-muted-foreground">
                    {t("security.step1Loading")}
                  </div>
                ) : (
                  // eslint-disable-next-line @next/next/no-img-element -- 本地生成的 data: URL，不走 next/image 优化
                  <img
                    src={qrDataURL}
                    alt="TOTP QR code"
                    width={200}
                    height={200}
                    data-testid="2fa-qr-image"
                  />
                )}
                <div className="text-xs text-muted-foreground break-all text-center">
                  {t("security.secret")}<span data-testid="2fa-secret" className="font-mono">{secretData?.secret ?? ""}</span>
                </div>
              </div>
              {codeError && (
                <Alert variant="destructive" data-testid="2fa-code-error">
                  <AlertDescription>{codeError}</AlertDescription>
                </Alert>
              )}
              <DialogFooter>
                <Button data-testid="btn-scanned" onClick={() => setStep("verify")} disabled={setupLoading}>
                  {t("security.scanned")}
                </Button>
              </DialogFooter>
            </>
          )}

          {step === "verify" && (
            <>
              <DialogHeader>
                <DialogTitle>{t("security.step2Title")}</DialogTitle>
                <DialogDescription>
                  {t("security.step2Desc")}
                </DialogDescription>
              </DialogHeader>
              <div className="space-y-3 py-2">
                {codeError && (
                  <Alert variant="destructive" data-testid="2fa-code-error">
                    <AlertDescription>{codeError}</AlertDescription>
                  </Alert>
                )}
                <Input
                  placeholder="000000"
                  value={code}
                  onChange={(e) => {
                    setCode(e.target.value)
                    setCodeError(null)
                  }}
                  maxLength={6}
                  data-testid="input-totp-code"
                />
              </div>
              <DialogFooter>
                <Button variant="outline" onClick={() => setStep("scan")}>{t("security.back")}</Button>
                <Button data-testid="btn-verify-code" onClick={handleVerify} disabled={verifyLoading}>
                  {verifyLoading ? t("security.verifying") : t("security.verifyAndEnable")}
                </Button>
              </DialogFooter>
            </>
          )}

          {step === "backup" && (
            <>
              <DialogHeader>
                <DialogTitle>{t("security.step3Title")}</DialogTitle>
                <DialogDescription>
                  {t("security.step3Desc")}
                </DialogDescription>
              </DialogHeader>
              <div className="grid grid-cols-1 gap-2 py-2 sm:grid-cols-2" data-testid="backup-codes-grid">
                {backupCodes.map((c) => (
                  <code
                    key={c}
                    className="rounded bg-muted px-2 py-1 text-sm font-mono text-center"
                    data-testid={`backup-code-${c}`}
                  >
                    {c}
                  </code>
                ))}
              </div>
              <DialogFooter>
                <Button data-testid="btn-finish-2fa" onClick={handleFinish}>
                  {t("security.finish")}
                </Button>
              </DialogFooter>
            </>
          )}
        </DialogContent>
      </Dialog>

      {/* Disable Dialog */}
      <Dialog open={showDisableDialog} onOpenChange={setShowDisableDialog}>
        <DialogContent data-testid="2fa-disable-dialog">
          <DialogHeader>
            <DialogTitle>{t("security.disableTitle")}</DialogTitle>
            <DialogDescription>
              {t("security.disableDesc")}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-3 py-2">
            {disableError && (
              <Alert variant="destructive" data-testid="2fa-disable-error">
                <AlertDescription>{disableError}</AlertDescription>
              </Alert>
            )}
            <Input
              placeholder="000000"
              value={disableCode}
              onChange={(e) => {
                setDisableCode(e.target.value)
                setDisableError(null)
              }}
              maxLength={6}
              data-testid="input-disable-code"
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowDisableDialog(false)}>{t("security.disableCancel")}</Button>
            <Button variant="destructive" data-testid="btn-confirm-disable" onClick={handleDisable} disabled={disableLoading}>
              {disableLoading ? t("security.disableProcessing") : t("security.disableConfirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
