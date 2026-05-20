import { NextRequest, NextResponse } from "next/server"
import { verifyAdminToken } from "@/lib/admin-auth"

const API_BASE = process.env.API_BASE_URL ?? "http://localhost:8080"
const ADMIN_TOKEN = process.env.ADMIN_TOKEN ?? ""

export async function GET(req: NextRequest) {
  if (!verifyAdminToken(req)) {
    return NextResponse.json({ error: { message: "Unauthorized" } }, { status: 401 })
  }
  try {
    const res = await fetch(`${API_BASE}/v1/admin/refund-failed`, {
      headers: { "X-Admin-Token": ADMIN_TOKEN },
      cache: "no-store",
    })
    const data = await res.json()
    return NextResponse.json(data, { status: res.status })
  } catch {
    return NextResponse.json({ error: { message: "Failed to reach API" } }, { status: 502 })
  }
}
