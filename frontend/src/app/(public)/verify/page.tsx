import type { Metadata } from "next"
import { VerifyClient } from "./verify-client"

export const metadata: Metadata = {
  title: "验证证据报告 — idcd",
  description:
    "上传 idcd 签发的 PDF 证据报告，公开验证签名链、TSA 时间戳与内容哈希。无需登录。",
  alternates: { canonical: "https://idcd.com/verify" },
}

export default function VerifyPage() {
  return (
    <main className="mx-auto w-full max-w-3xl px-4 py-12 sm:py-16">
      <div className="mb-8">
        <h1 className="text-3xl font-bold tracking-tight">公开验签</h1>
        <p className="mt-2 text-sm text-muted-foreground">
          上传 idcd 签发的 PDF 报告，系统将解析其中的 PAdES 签名、TSA 时间戳与内容哈希，
          并显示对应的法律声明。该接口无需登录，纯只读，不会保存上传文件。
        </p>
      </div>
      <VerifyClient />
    </main>
  )
}
