import { NextResponse } from "next/server"

const API_BASE = process.env.API_BASE_URL ?? "http://localhost:8080"

export async function GET() {
  try {
    const res = await fetch(`${API_BASE}/v1/admin/refund-failed`, { cache: "no-store" })
    const data = await res.json()
    return NextResponse.json(data, { status: res.status })
  } catch {
    return NextResponse.json({ error: { message: "Failed to reach API" } }, { status: 502 })
  }
}
