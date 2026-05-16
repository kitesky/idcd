import type { Metadata } from "next"
import { getT } from "@/i18n/getT"
import { SessionsClient } from "./sessions-client"

export async function generateMetadata(): Promise<Metadata> {
  const t = await getT("settings.sessions")
  return { title: `${t("title")} — idcd` }
}

export default function SessionsPage() {
  return (
    <main className="flex-1 container max-w-4xl">
      <SessionsClient />
    </main>
  )
}
