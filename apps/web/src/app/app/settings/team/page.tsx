import type { Metadata } from "next"
import { getT } from "@/i18n/getT"
import { TeamClient } from "./team-client"

export async function generateMetadata(): Promise<Metadata> {
  const t = await getT("settings.team")
  return { title: `${t("title")} — idcd` }
}

export default function TeamPage() {
  return (
    <main className="flex-1 container max-w-4xl">
      <TeamClient />
    </main>
  )
}
