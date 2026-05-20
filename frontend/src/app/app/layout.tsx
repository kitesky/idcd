import { cookies } from "next/headers"
import { redirect } from "next/navigation"
import { AppShell } from "./_components/app-shell"

const SESSION_COOKIE = "access_token"

interface Profile {
  email?: string
  display_name?: string | null
  avatar_url?: string | null
}

interface Subscription {
  plan?: string
  status?: string
}

function apiBase(): string {
  return process.env.INTERNAL_API_URL ?? process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"
}

async function fetchWithSession<T>(path: string, token: string): Promise<T | null> {
  try {
    const res = await fetch(`${apiBase()}${path}`, {
      headers: {
        // Forward the HttpOnly cookie so the API can validate the session
        // server-side (browsers don't send cookies for cross-origin SSR fetches).
        cookie: `${SESSION_COOKIE}=${token}`,
      },
      cache: "no-store",
    })
    if (!res.ok) return null
    const body = (await res.json().catch(() => null)) as { data?: T } | T | null
    if (!body) return null
    return ("data" in (body as object) ? (body as { data: T }).data : (body as T)) ?? null
  } catch {
    return null
  }
}

function planLabel(sub: Subscription | null): string {
  if (!sub) return "Free"
  // Only an active or past_due subscription should show as paid; cancelled/pending fall back to Free.
  const status = (sub.status ?? "").toLowerCase()
  if (status !== "active" && status !== "past_due") return "Free"
  const plan = (sub.plan ?? "").toLowerCase()
  if (!plan || plan === "free") return "Free"
  return plan.charAt(0).toUpperCase() + plan.slice(1)
}

export default async function AppLayout({ children }: { children: React.ReactNode }) {
  const store = await cookies()
  const token = store.get(SESSION_COOKIE)?.value
  if (!token) {
    redirect("/auth/login")
  }

  const [profile, subscription] = await Promise.all([
    fetchWithSession<Profile>("/v1/account/profile", token),
    fetchWithSession<Subscription>("/v1/billing/subscription", token),
  ])

  if (!profile) {
    redirect("/auth/login")
  }
  return (
    <AppShell
      email={profile.email ?? "user@example.com"}
      displayName={profile.display_name ?? null}
      avatarUrl={profile.avatar_url ?? null}
      plan={planLabel(subscription)}
    >
      {children}
    </AppShell>
  )
}
