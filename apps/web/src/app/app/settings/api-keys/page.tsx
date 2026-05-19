import type { Metadata } from "next"
import { getT } from "@/i18n/getT"
import { APIKeysClient } from "./api-keys-client"

export async function generateMetadata(): Promise<Metadata> {
  const t = await getT("settings.apiKeys")
  return { title: `${t("title")} — idcd` }
}

export default function APIKeysPage() {
  return (
    <main className="flex-1 container max-w-4xl">
      <APIKeysClient />
    </main>
  )
}
