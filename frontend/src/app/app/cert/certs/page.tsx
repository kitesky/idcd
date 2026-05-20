import type { Metadata } from "next"
import { CertsClient } from "./certs-client"

export const metadata: Metadata = {
  title: "已签证书 - idcd",
  description: "查看所有已签发的 TLS 证书",
}

export default function CertsPage() {
  return (
    <>
      <div>
        <h1 className="text-2xl font-bold tracking-tight">已签证书</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          所有已签发的证书，距离到期 30 天内会标红提醒。
        </p>
      </div>
      <CertsClient />
    </>
  )
}
