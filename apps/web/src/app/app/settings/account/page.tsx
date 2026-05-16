import { AccountClient } from "./account-client"

export const metadata = {
  title: "账号设置 — idcd",
}

export default function AccountPage() {
  return (
    <main className="flex-1 container max-w-3xl">
      <AccountClient />
    </main>
  )
}
