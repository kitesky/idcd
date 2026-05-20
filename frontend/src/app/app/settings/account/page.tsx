import type { Metadata } from "next"
import { getT } from "@/i18n/getT"
import { AccountClient } from "./account-client"

export async function generateMetadata(): Promise<Metadata> {
  const t = await getT("settings.account")
  return { title: `${t("title")} — idcd` }
}

export default function AccountPage() {
  return (
    <main className="flex-1 container max-w-3xl">
      <AccountClient />
    </main>
  )
}
