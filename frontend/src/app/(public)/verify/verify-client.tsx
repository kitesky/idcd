"use client"

import { useRef, useState } from "react"
import { AlertCircle, CheckCircle2, ShieldAlert, Upload } from "lucide-react"

import {
  Alert,
  AlertDescription,
  AlertTitle,
  Badge,
  Button,
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  Input,
  Label,
} from "@/components/ui"
import { verifyPdf, type AttestVerifyResult } from "@/lib/api/verdict"

// Cloudflare Workers / Next route handlers cap multipart uploads to ~25 MB
// in our deploy target; the backend itself is more permissive. Guard at the
// client to give a friendly error before the user wastes a slow upload.
const MAX_UPLOAD_BYTES = 20 * 1024 * 1024 // 20 MB

export function VerifyClient() {
  const [file, setFile] = useState<File | null>(null)
  const [result, setResult] = useState<AttestVerifyResult | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)

  function handlePick(event: React.ChangeEvent<HTMLInputElement>) {
    setError(null)
    setResult(null)
    const f = event.target.files?.[0] ?? null
    if (!f) {
      setFile(null)
      return
    }
    if (f.size > MAX_UPLOAD_BYTES) {
      setFile(null)
      setError(
        `文件超过 ${Math.round(MAX_UPLOAD_BYTES / 1024 / 1024)} MB 限制；如需验签大尺寸报告，请联系客服。`,
      )
      event.target.value = ""
      return
    }
    setFile(f)
  }

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!file) {
      setError("请先选择要验证的 PDF 文件。")
      return
    }
    setSubmitting(true)
    setError(null)
    setResult(null)
    try {
      const data = await verifyPdf(file)
      setResult(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : "验签失败")
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle className="text-base">上传 PDF</CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4" data-testid="verify-form">
            <div className="space-y-2">
              <Label htmlFor="verify-pdf">证据报告 PDF</Label>
              <Input
                id="verify-pdf"
                ref={inputRef}
                type="file"
                accept="application/pdf"
                onChange={handlePick}
                data-testid="verify-file-input"
                disabled={submitting}
              />
              {file && (
                <p className="text-xs text-muted-foreground" data-testid="verify-file-name">
                  已选择：{file.name}（{(file.size / 1024).toFixed(1)} KB）
                </p>
              )}
            </div>

            <div className="flex items-center justify-end gap-3">
              <Button
                type="submit"
                disabled={!file || submitting}
                data-testid="verify-submit-btn"
              >
                <Upload className="mr-2 h-4 w-4" />
                {submitting ? "验签中…" : "开始验证"}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>

      {error && (
        <Alert variant="destructive" data-testid="verify-error">
          <AlertCircle className="h-4 w-4" />
          <AlertTitle>无法完成验签</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      {result && <VerifyResultView result={result} />}
    </div>
  )
}

function VerifyResultView({ result }: { result: AttestVerifyResult }) {
  return (
    <div className="space-y-4" data-testid="verify-result">
      {result.valid ? (
        <Alert data-testid="verify-result-valid">
          <CheckCircle2 className="h-4 w-4" />
          <AlertTitle>签名校验通过</AlertTitle>
          <AlertDescription>
            该 PDF 中的 PAdES 签名链与 TSA 时间戳均通过校验，文件未被篡改。
          </AlertDescription>
        </Alert>
      ) : (
        <Alert variant="destructive" data-testid="verify-result-invalid">
          <ShieldAlert className="h-4 w-4" />
          <AlertTitle>签名校验未通过</AlertTitle>
          <AlertDescription>
            该 PDF 可能已被修改、签名已过期或签发机构无法识别，请勿将其作为证据使用。
          </AlertDescription>
        </Alert>
      )}

      <Card>
        <CardHeader>
          <CardTitle className="text-base">签名详情</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3 text-sm">
          <ResultRow label="报告类型">
            <Badge variant="outline" data-testid="result-report-type">
              {result.report_type}
            </Badge>
          </ResultRow>
          <ResultRow label="签名链">
            <span className="font-mono text-[11px] break-all">{result.signature_chain}</span>
          </ResultRow>
          <ResultRow label="公钥指纹">
            <span className="font-mono text-[11px] break-all">
              {result.public_key_fingerprint}
            </span>
          </ResultRow>
          <ResultRow label="签发时间">
            <span className="text-xs">{result.signed_at}</span>
          </ResultRow>
          <ResultRow label="TSA 提供方">
            <span className="text-xs">{result.tsa_provider}</span>
          </ResultRow>
          <ResultRow label="内容哈希">
            <span className="font-mono text-[11px] break-all">{result.content_hash}</span>
          </ResultRow>
        </CardContent>
      </Card>

      {/*
        Per v2 D-Concern1: the verify response MUST surface the legal
        disclaimer verbatim — third parties must see that idcd reports are
        "一手观测数据，不构成司法鉴定结论".
      */}
      <Alert data-testid="verify-disclaimer">
        <AlertTitle>法律声明</AlertTitle>
        <AlertDescription className="whitespace-pre-line">
          {result.legal_disclaimer}
        </AlertDescription>
      </Alert>
    </div>
  )
}

function ResultRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-start justify-between gap-4">
      <span className="shrink-0 text-muted-foreground">{label}</span>
      <div className="min-w-0 max-w-[70%] text-right">{children}</div>
    </div>
  )
}
