import { NextRequest, NextResponse } from "next/server"
import { ADMIN_SESSION_COOKIE, timingSafeEqual } from "@/lib/admin-auth"

const INTERNAL_API_URL = process.env.INTERNAL_API_URL ?? "http://localhost:8080"
const ADMIN_TOKEN = process.env.ADMIN_TOKEN ?? ""
const ADMIN_PORTAL_TOKEN = process.env.ADMIN_PORTAL_TOKEN ?? ""

function isAdminSession(req: NextRequest): boolean {
  if (!ADMIN_PORTAL_TOKEN) return false
  const tok = req.cookies.get(ADMIN_SESSION_COOKIE)?.value ?? ""
  return Boolean(tok) && timingSafeEqual(tok, ADMIN_PORTAL_TOKEN)
}

export async function PATCH(
  req: NextRequest,
  { params }: { params: Promise<{ id: string }> },
) {
  if (!isAdminSession(req)) {
    return NextResponse.json({ error: { message: "Unauthorized" } }, { status: 401 })
  }
  const { id } = await params
  try {
    const body = await req.json()
    const res = await fetch(
      `${INTERNAL_API_URL}/v1/admin/status-incidents/${encodeURIComponent(id)}`,
      {
        method: "PATCH",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${ADMIN_TOKEN}` },
        body: JSON.stringify(body),
      },
    )
    const data = await res.json()
    return NextResponse.json(data, { status: res.status })
  } catch {
    return NextResponse.json({ error: { message: "Failed to reach API" } }, { status: 502 })
  }
}

export async function DELETE(
  req: NextRequest,
  { params }: { params: Promise<{ id: string }> },
) {
  if (!isAdminSession(req)) {
    return NextResponse.json({ error: { message: "Unauthorized" } }, { status: 401 })
  }
  const { id } = await params
  try {
    const res = await fetch(
      `${INTERNAL_API_URL}/v1/admin/status-incidents/${encodeURIComponent(id)}`,
      {
        method: "DELETE",
        headers: { Authorization: `Bearer ${ADMIN_TOKEN}` },
      },
    )
    const data = await res.json()
    return NextResponse.json(data, { status: res.status })
  } catch {
    return NextResponse.json({ error: { message: "Failed to reach API" } }, { status: 502 })
  }
}
