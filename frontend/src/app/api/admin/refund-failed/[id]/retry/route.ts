import { NextRequest, NextResponse } from "next/server"
import { verifyAdminToken } from "@/lib/admin-auth"

const API_BASE = process.env.API_BASE_URL ?? "http://localhost:8080"
const ADMIN_TOKEN = process.env.ADMIN_TOKEN ?? ""

export async function POST(req: NextRequest, { params }: { params: Promise<{ id: string }> }) {
  if (!verifyAdminToken(req)) {
    return NextResponse.json({ error: { message: "Unauthorized" } }, { status: 401 })
  }
  const { id } = await params
  try {
    const res = await fetch(`${API_BASE}/v1/admin/refund-failed/${encodeURIComponent(id)}/retry`, {
      method: "POST",
      headers: { "X-Admin-Token": ADMIN_TOKEN },
    })
    const data = await res.json()
    return NextResponse.json(data, { status: res.status })
  } catch {
    return NextResponse.json({ error: { message: "Failed to reach API" } }, { status: 502 })
  }
}
