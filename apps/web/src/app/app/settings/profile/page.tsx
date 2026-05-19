import type { Metadata } from "next"
import { getT } from "@/i18n/getT"
import { ProfileClient } from "./profile-client"

export async function generateMetadata(): Promise<Metadata> {
  const t = await getT("settings.profile")
  return { title: `${t("title")} — idcd` }
}

export default function ProfilePage() {
  return (
    <main className="flex-1 container max-w-3xl">
      <ProfileClient />
    </main>
  )
}
