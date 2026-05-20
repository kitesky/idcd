import type { Metadata } from "next"
import { getT } from "@/i18n/getT"
import { SecurityClient } from "./security-client"

export async function generateMetadata(): Promise<Metadata> {
  const t = await getT("settings.security")
  return { title: `${t("title")} — idcd` }
}

export default function SecurityPage() {
  return (
    <main className="flex-1 container max-w-3xl">
      <SecurityClient />
    </main>
  )
}
