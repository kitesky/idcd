import { cookies } from "next/headers"
import { redirect } from "next/navigation"
import { AppShell } from "./_components/app-shell"

const SESSION_COOKIE = "access_token"

interface Profile {
  email?: string
  display_name?: string | null
  avatar_url?: string | null
}

async function loadProfile(): Promise<Profile | null> {
  const store = await cookies()
  const token = store.get(SESSION_COOKIE)?.value
  if (!token) return null

  const base = process.env.INTERNAL_API_URL ?? process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"
  try {
    const res = await fetch(`${base}/v1/account/profile`, {
      headers: {
        // Forward the HttpOnly cookie so the API can validate the session
        // server-side (browsers don't send cookies for cross-origin SSR fetches).
        cookie: `${SESSION_COOKIE}=${token}`,
      },
      cache: "no-store",
    })
    if (!res.ok) return null
    const body = (await res.json().catch(() => null)) as { data?: Profile } | Profile | null
    if (!body) return null
    return ("data" in (body as object) ? (body as { data: Profile }).data : (body as Profile)) ?? null
  } catch {
    return null
  }
}

export default async function AppLayout({ children }: { children: React.ReactNode }) {
  const profile = await loadProfile()
  if (!profile) {
    redirect("/auth/login")
  }
  return (
    <AppShell
      email={profile.email ?? "user@example.com"}
      displayName={profile.display_name ?? null}
      avatarUrl={profile.avatar_url ?? null}
    >
      {children}
    </AppShell>
  )
}
