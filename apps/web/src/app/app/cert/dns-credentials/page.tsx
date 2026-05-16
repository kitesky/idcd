import type { Metadata } from "next"
import { CredentialsClient } from "./credentials-client"

export const metadata: Metadata = {
  title: "DNS 凭据 - idcd",
  description: "管理用于 DNS-01 自动验证的 DNS 服务商凭据",
}

export default function DnsCredentialsPage() {
  return (
    <>
      <div>
        <h1 className="text-2xl font-bold tracking-tight">DNS 凭据</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          用于 DNS-01 自动写入 TXT 记录的服务商凭据。凭据仅保存指纹，原始 token 不可读出。
        </p>
      </div>
      <CredentialsClient />
    </>
  )
}
