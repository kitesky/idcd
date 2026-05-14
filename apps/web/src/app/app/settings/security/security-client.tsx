"use client"

import { useState } from "react"
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
} from "@/components/ui"

type Step = "idle" | "scan" | "verify" | "backup"

const MOCK_SECRET = "JBSWY3DPEHPK3PXP"
const MOCK_OTPAUTH = `otpauth://totp/idcd:user@example.com?secret=${MOCK_SECRET}&issuer=idcd`
const MOCK_BACKUP_CODES = [
  "ABCD1234", "EFGH5678", "IJKL9012", "MNOP3456",
  "QRST7890", "UVWX1234", "YZAB5678", "CDEF9012",
]

export function SecurityClient() {
  const [enabled, setEnabled] = useState(false)
  const [step, setStep] = useState<Step>("idle")
  const [code, setCode] = useState("")
  const [codeError, setCodeError] = useState<string | null>(null)
  const [disableCode, setDisableCode] = useState("")
  const [disableError, setDisableError] = useState<string | null>(null)
  const [showDisableDialog, setShowDisableDialog] = useState(false)

  function openSetup() {
    setStep("scan")
    setCode("")
    setCodeError(null)
  }

  function handleVerify() {
    if (code.length !== 6 || !/^\d+$/.test(code)) {
      setCodeError("请输入 6 位数字验证码")
      return
    }
    setStep("backup")
  }

  function handleFinish() {
    setEnabled(true)
    setStep("idle")
    setCode("")
    setCodeError(null)
  }

  function handleDisable() {
    if (disableCode.length !== 6 || !/^\d+$/.test(disableCode)) {
      setDisableError("请输入 6 位数字验证码")
      return
    }
    setEnabled(false)
    setShowDisableDialog(false)
    setDisableCode("")
    setDisableError(null)
  }

  const qrURL = `https://api.qrserver.com/v1/create-qr-code/?size=200x200&data=${encodeURIComponent(MOCK_OTPAUTH)}`

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
            >
              启用 2FA
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
                <img
                  src={qrURL}
                  alt="TOTP QR code"
                  width={200}
                  height={200}
                  data-testid="2fa-qr-image"
                />
                <div className="text-xs text-muted-foreground break-all text-center">
                  密钥：<span data-testid="2fa-secret" className="font-mono">{MOCK_SECRET}</span>
                </div>
              </div>
              <DialogFooter>
                <Button data-testid="btn-scanned" onClick={() => setStep("verify")}>
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
                <Button data-testid="btn-verify-code" onClick={handleVerify}>
                  验证并启用
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
                {MOCK_BACKUP_CODES.map((c) => (
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
            <Button variant="destructive" data-testid="btn-confirm-disable" onClick={handleDisable}>
              确认关闭
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
