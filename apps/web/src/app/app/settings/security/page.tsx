import { SecurityClient } from "./security-client"

export const metadata = {
  title: "安全设置 — idcd",
}

export default function SecurityPage() {
  return (
    <main className="flex-1 container max-w-3xl">
      <SecurityClient />
    </main>
  )
}
