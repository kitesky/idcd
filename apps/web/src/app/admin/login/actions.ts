"use server"

import { cookies } from "next/headers"
import { redirect } from "next/navigation"
import { ADMIN_SESSION_COOKIE, timingSafeEqual } from "@/lib/admin-auth"

const SESSION_MAX_AGE = 60 * 60 * 8 // 8h

export async function loginAction(formData: FormData): Promise<void> {
  const token = String(formData.get("token") ?? "")
  const next = String(formData.get("next") ?? "/admin")
  const expected = process.env.ADMIN_PORTAL_TOKEN ?? ""

  // Failures redirect back to /admin/login with an error reason — keeping the
  // action signature `Promise<void>` so it's usable directly as `<form action>`.
  if (!expected) redirect("/admin/login?reason=not_configured")
  if (!token) redirect(`/admin/login?reason=missing_token&next=${encodeURIComponent(next)}`)
  if (!timingSafeEqual(token, expected)) {
    redirect(`/admin/login?reason=invalid_token&next=${encodeURIComponent(next)}`)
  }

  const safeNext = next.startsWith("/admin") && !next.startsWith("/admin/login") ? next : "/admin"

  const store = await cookies()
  store.set({
    name: ADMIN_SESSION_COOKIE,
    value: token,
    httpOnly: true,
    secure: process.env.NODE_ENV !== "development",
    sameSite: "lax",
    path: "/admin",
    maxAge: SESSION_MAX_AGE,
  })
  redirect(safeNext)
}

export async function logoutAction(): Promise<void> {
  const store = await cookies()
  store.set({
    name: ADMIN_SESSION_COOKIE,
    value: "",
    httpOnly: true,
    secure: process.env.NODE_ENV !== "development",
    sameSite: "lax",
    path: "/admin",
    maxAge: 0,
  })
  redirect("/admin/login")
}
