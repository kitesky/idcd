import { NextRequest, NextResponse } from "next/server"

const API_BASE = process.env.API_BASE_URL ?? "http://localhost:8080"

// POST /api/admin/refund-failed/:id/retry → proxy to Go API
export async function POST(
  _req: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  const { id } = await params
  try {
    const res = await fetch(
      `${API_BASE}/v1/admin/refund-failed/${encodeURIComponent(id)}/retry`,
      { method: "POST" }
    )
    const data = await res.json()
    return NextResponse.json(data, { status: res.status })
  } catch {
    return NextResponse.json(
      { error: { message: "Failed to reach API" } },
      { status: 502 }
    )
  }
}
