import { NextRequest, NextResponse } from "next/server"
import { verifyAdminToken } from "@/lib/admin-auth"

const INTERNAL_API_URL = process.env.INTERNAL_API_URL ?? "http://localhost:8080"
const ADMIN_TOKEN = process.env.ADMIN_TOKEN ?? ""

export async function GET(req: NextRequest) {
  if (!verifyAdminToken(req)) {
    return NextResponse.json({ error: { message: "Unauthorized" } }, { status: 401 })
  }
  try {
    const res = await fetch(`${INTERNAL_API_URL}/internal/admin/upgrade-rollouts`, {
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
  if (!verifyAdminToken(req)) {
    return NextResponse.json({ error: { message: "Unauthorized" } }, { status: 401 })
  }
  try {
    const body = await req.json()
    const res = await fetch(`${INTERNAL_API_URL}/internal/admin/upgrade-rollouts`, {
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
