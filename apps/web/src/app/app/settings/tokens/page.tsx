import { TokensClient } from "./tokens-client"

export const metadata = {
  title: "访问令牌 — idcd",
}

export default function TokensPage() {
  return (
    <main className="flex-1 container max-w-4xl">
      <TokensClient />
    </main>
  )
}
