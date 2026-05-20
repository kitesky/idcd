import { NextRequest } from "next/server"

const ADMIN_TOKEN = process.env.ADMIN_TOKEN ?? ""

export const ADMIN_SESSION_COOKIE = "admin_session"

/**
 * Constant-time string comparison. Returns false immediately on length
 * mismatch (acceptable: length itself is not the secret here).
 */
export function timingSafeEqual(a: string, b: string): boolean {
  if (a.length !== b.length) return false
  let diff = 0
  for (let i = 0; i < a.length; i++) diff |= a.charCodeAt(i) ^ b.charCodeAt(i)
  return diff === 0
}

/**
 * Verifies the X-Admin-Token request header against ADMIN_TOKEN. Returns false
 * when either value is empty.
 */
export function verifyAdminToken(req: NextRequest): boolean {
  const provided = req.headers.get("X-Admin-Token") ?? ""
  if (!ADMIN_TOKEN || !provided) return false
  return timingSafeEqual(provided, ADMIN_TOKEN)
}
