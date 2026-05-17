import type { Metadata } from "next"
import { WizardClient } from "./wizard-client"

export const metadata: Metadata = {
  title: "申请证书 - idcd",
  description: "通过 4 步向导申请一张新的 TLS 证书",
}

export default function NewCertPage() {
  return (
    <>
      <div>
        <h1 className="text-2xl font-bold tracking-tight">申请证书</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          按 4 步向导填写：域名 → CA → 验证方式 → 确认。
        </p>
      </div>
      <WizardClient />
    </>
  )
}
