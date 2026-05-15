"use client"

import { useState, useEffect } from "react"
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
} from "@/components/ui"
import { apiRequest } from "@/lib/api"

type Step = "idle" | "scan" | "verify" | "backup"

type PasskeyItem = {
  id: string
  device_name: string
  created_at: string
  last_used_at?: string | null
}

export function SecurityClient() {
  const [enabled, setEnabled] = useState(false)
  const [step, setStep] = useState<Step>("idle")
  const [code, setCode] = useState("")
  const [codeError, setCodeError] = useState<string | null>(null)
  const [disableCode, setDisableCode] = useState("")
  const [disableError, setDisableError] = useState<string | null>(null)
  const [showDisableDialog, setShowDisableDialog] = useState(false)

  const [secretData, setSecretData] = useState<{ secret: string; otpauth_uri: string } | null>(null)
  const [backupCodes, setBackupCodes] = useState<string[]>([])
  const [setupLoading, setSetupLoading] = useState(false)
  const [verifyLoading, setVerifyLoading] = useState(false)
  const [disableLoading, setDisableLoading] = useState(false)

  const [passkeys, setPasskeys] = useState<PasskeyItem[]>([])
  const [passkeyLoading, setPasskeyLoading] = useState(false)
  const [passkeyAdding, setPasskeyAdding] = useState(false)
  const [passkeyError, setPasskeyError] = useState<string | null>(null)

  useEffect(() => {
    apiRequest<{ enabled: boolean }>("/v1/account/2fa/status")
      .then((res) => setEnabled(res.enabled))
      .catch(() => {
        // Silently ignore — leave default false; server may be unavailable during dev/test
      })
  }, [])

  useEffect(() => {
    setPasskeyLoading(true)
    apiRequest<{ data: { passkeys: PasskeyItem[] } }>("/v1/account/passkeys")
      .then((res) => setPasskeys(res.data.passkeys))
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
    setSetupLoading(true)
    try {
      const res = await apiRequest<{ data: { secret: string; otpauth_uri: string } }>("/v1/account/2fa/setup", {
        method: "POST",
      })
      setSecretData(res.data)
    } catch (err) {
      setCodeError(err instanceof Error ? err.message : "获取 2FA 配置失败")
    } finally {
      setSetupLoading(false)
    }
  }

  async function handleVerify() {
    if (code.length !== 6 || !/^\d+$/.test(code)) {
      setCodeError("请输入 6 位数字验证码")
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
      setCodeError(err instanceof Error ? err.message : "验证失败，请重试")
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
      setDisableError("请输入 6 位数字验证码")
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
      setDisableError(err instanceof Error ? err.message : "关闭失败，请重试")
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
        throw new Error("此浏览器不支持 Passkey")
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

      if (!credential) throw new Error("注册被取消")

      const response = credential.response as AuthenticatorAttestationResponse
      const { data: result } = await apiRequest<{ data: { credential_id: string; device_name: string } }>("/v1/account/passkeys/register/complete", {
        method: "POST",
        body: JSON.stringify({
          challenge: data.challenge_id,
          response: {
            id: credential.id,
            rawId: btoa(String.fromCharCode(...new Uint8Array(credential.rawId))).replace(/\+/g, "-").replace(/\//g, "_").replace(/=/g, ""),
            response: {
              clientDataJSON: btoa(String.fromCharCode(...new Uint8Array(response.clientDataJSON))).replace(/\+/g, "-").replace(/\//g, "_").replace(/=/g, ""),
              attestationObject: btoa(String.fromCharCode(...new Uint8Array(response.attestationObject))).replace(/\+/g, "-").replace(/\//g, "_").replace(/=/g, ""),
            },
          },
          device_name: "My Passkey",
        }),
      })
      setPasskeys(prev => [...prev, {
        id: result.credential_id,
        device_name: result.device_name,
        created_at: new Date().toISOString(),
        last_used_at: null,
      }])
    } catch (err) {
      setPasskeyError(err instanceof Error ? err.message : "添加 Passkey 失败")
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
      setPasskeyError(err instanceof Error ? err.message : "删除 Passkey 失败")
    }
  }

  const qrURL = secretData
    ? `https://api.qrserver.com/v1/create-qr-code/?size=200x200&data=${encodeURIComponent(secretData.otpauth_uri)}`
    : ""

  return (
    <div data-testid="security-page" className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">安全设置</h1>
        <p className="text-muted-foreground text-sm mt-1">管理账号安全选项</p>
      </div>

      <Card data-testid="2fa-card">
        <CardHeader>
          <CardTitle>两步验证（2FA）</CardTitle>
          <CardDescription>
            使用 Google Authenticator、Authy 或 1Password 等应用扫码开启两步验证，为账号添加额外保护。
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center gap-3">
            <span className="text-sm text-muted-foreground">当前状态：</span>
            {enabled ? (
              <Badge data-testid="2fa-status-badge">已启用</Badge>
            ) : (
              <Badge variant="secondary" data-testid="2fa-status-badge">未启用</Badge>
            )}
          </div>

          {!enabled ? (
            <Button
              variant="outline"
              data-testid="btn-enable-2fa"
              onClick={openSetup}
              disabled={setupLoading}
            >
              {setupLoading ? "加载中..." : "启用 2FA"}
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
              关闭 2FA
            </Button>
          )}
        </CardContent>
      </Card>

      <Card data-testid="passkey-card">
        <CardHeader>
          <CardTitle>Passkey</CardTitle>
          <CardDescription>
            使用设备生物识别（Touch ID / Face ID）无密码登录
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
            {passkeyAdding ? "添加中..." : "添加 Passkey"}
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
                        添加于 {new Date(pk.created_at).toLocaleDateString("zh-CN")}
                      </p>
                    </div>
                    <Button
                      variant="ghost"
                      size="sm"
                      data-testid={`btn-delete-passkey-${pk.id}`}
                      onClick={() => handleDeletePasskey(pk.id)}
                    >
                      删除
                    </Button>
                  </CardContent>
                </Card>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Setup Dialog — 3 steps */}
      <Dialog open={step !== "idle"} onOpenChange={(open) => { if (!open) setStep("idle") }}>
        <DialogContent data-testid="2fa-setup-dialog">
          {step === "scan" && (
            <>
              <DialogHeader>
                <DialogTitle>第 1 步：扫描二维码</DialogTitle>
                <DialogDescription>
                  使用 Google Authenticator、Authy 或 1Password 扫描下方二维码，或手动输入密钥。
                </DialogDescription>
              </DialogHeader>
              <div className="flex flex-col items-center gap-4 py-2">
                {setupLoading ? (
                  <div className="flex items-center justify-center w-[200px] h-[200px] text-sm text-muted-foreground">
                    加载中...
                  </div>
                ) : (
                  <img
                    src={qrURL}
                    alt="TOTP QR code"
                    width={200}
                    height={200}
                    data-testid="2fa-qr-image"
                  />
                )}
                <div className="text-xs text-muted-foreground break-all text-center">
                  密钥：<span data-testid="2fa-secret" className="font-mono">{secretData?.secret ?? ""}</span>
                </div>
              </div>
              {codeError && (
                <Alert variant="destructive" data-testid="2fa-code-error">
                  <AlertDescription>{codeError}</AlertDescription>
                </Alert>
              )}
              <DialogFooter>
                <Button data-testid="btn-scanned" onClick={() => setStep("verify")} disabled={setupLoading}>
                  我已扫描
                </Button>
              </DialogFooter>
            </>
          )}

          {step === "verify" && (
            <>
              <DialogHeader>
                <DialogTitle>第 2 步：输入验证码</DialogTitle>
                <DialogDescription>
                  请输入验证器 App 中显示的 6 位数字验证码。
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
                <Button variant="outline" onClick={() => setStep("scan")}>返回</Button>
                <Button data-testid="btn-verify-code" onClick={handleVerify} disabled={verifyLoading}>
                  {verifyLoading ? "验证中..." : "验证并启用"}
                </Button>
              </DialogFooter>
            </>
          )}

          {step === "backup" && (
            <>
              <DialogHeader>
                <DialogTitle>第 3 步：保存备用码</DialogTitle>
                <DialogDescription>
                  请将以下备用码保存在安全的地方。每个备用码只能使用一次，丢失后无法恢复。
                </DialogDescription>
              </DialogHeader>
              <div className="grid grid-cols-2 gap-2 py-2" data-testid="backup-codes-grid">
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
                  已保存，关闭
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
            <DialogTitle>关闭两步验证</DialogTitle>
            <DialogDescription>
              请输入当前验证器中的 6 位验证码以确认关闭 2FA。
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
            <Button variant="outline" onClick={() => setShowDisableDialog(false)}>取消</Button>
            <Button variant="destructive" data-testid="btn-confirm-disable" onClick={handleDisable} disabled={disableLoading}>
              {disableLoading ? "处理中..." : "确认关闭"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
