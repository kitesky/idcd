import type { Metadata } from "next"
import { getT } from "@/i18n/getT"
import { TokensClient } from "./tokens-client"

export async function generateMetadata(): Promise<Metadata> {
  const t = await getT("settings.tokens")
  return { title: `${t("title")} — idcd` }
}

export default function TokensPage() {
  return (
    <main className="flex-1 container max-w-4xl">
      <TokensClient />
    </main>
  )
}
