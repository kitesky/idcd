import { NextRequest, NextResponse } from "next/server"
import { ADMIN_SESSION_COOKIE, timingSafeEqual } from "@/lib/admin-auth"

const INTERNAL_API_URL = process.env.INTERNAL_API_URL ?? "http://localhost:8080"
const ADMIN_TOKEN = process.env.ADMIN_TOKEN ?? ""
const ADMIN_PORTAL_TOKEN = process.env.ADMIN_PORTAL_TOKEN ?? ""

// Verify the browser holds a valid admin portal session cookie. Mirrors the
// /admin/* middleware gate so this API route is only callable from a logged-in
// admin tab — never directly from an unauthenticated curl. Distinct from the
// upstream ADMIN_TOKEN we forward to the API as Authorization: Bearer.
function isAdminSession(req: NextRequest): boolean {
  if (!ADMIN_PORTAL_TOKEN) return false
  const tok = req.cookies.get(ADMIN_SESSION_COOKIE)?.value ?? ""
  return Boolean(tok) && timingSafeEqual(tok, ADMIN_PORTAL_TOKEN)
}

export async function GET(req: NextRequest) {
  if (!isAdminSession(req)) {
    return NextResponse.json({ error: { message: "Unauthorized" } }, { status: 401 })
  }
  try {
    const res = await fetch(`${INTERNAL_API_URL}/v1/admin/status-incidents`, {
      headers: { Authorization: `Bearer ${ADMIN_TOKEN}` },
      cache: "no-store",
    })
    const data = await res.json()
    return NextResponse.json(data, { status: res.status })
  } catch {
    return NextResponse.json({ error: { message: "Failed to reach API" } }, { status: 502 })
  }
}

export async function POST(req: NextRequest) {
  if (!isAdminSession(req)) {
    return NextResponse.json({ error: { message: "Unauthorized" } }, { status: 401 })
  }
  try {
    const body = await req.json()
    const res = await fetch(`${INTERNAL_API_URL}/v1/admin/status-incidents`, {
      method: "POST",
      headers: { "Content-Type": "application/json", Authorization: `Bearer ${ADMIN_TOKEN}` },
      body: JSON.stringify(body),
    })
    const data = await res.json()
    return NextResponse.json(data, { status: res.status })
  } catch {
    return NextResponse.json({ error: { message: "Failed to reach API" } }, { status: 502 })
  }
}
