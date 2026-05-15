import { NextRequest, NextResponse } from "next/server"

const INTERNAL_API_URL = process.env.INTERNAL_API_URL ?? "http://localhost:8080"
const ADMIN_TOKEN = process.env.ADMIN_TOKEN ?? ""

export async function GET(req: NextRequest) {
  try {
    const { searchParams } = req.nextUrl
    const res = await fetch(
      `${INTERNAL_API_URL}/v1/admin/beta-invitations?${searchParams.toString()}`,
      {
        headers: { "X-Admin-Token": ADMIN_TOKEN },
        cache: "no-store",
      },
    )
    const data = await res.json()
    return NextResponse.json(data, { status: res.status })
  } catch {
    return NextResponse.json({ error: { message: "Failed to reach API" } }, { status: 502 })
  }
}

export async function POST(req: NextRequest) {
  try {
    const body = await req.json()
    const res = await fetch(`${INTERNAL_API_URL}/v1/admin/beta-invitations`, {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-Admin-Token": ADMIN_TOKEN },
      body: JSON.stringify(body),
    })
    const data = await res.json()
    return NextResponse.json(data, { status: res.status })
  } catch {
    return NextResponse.json({ error: { message: "Failed to reach API" } }, { status: 502 })
  }
}
