import { NextRequest } from "next/server"

/**
 * The admin token read once at module init.
 * Shared by all admin API routes via this module.
 */
const ADMIN_TOKEN = process.env.ADMIN_TOKEN ?? ""

/**
 * Constant-time comparison of the X-Admin-Token request header against the
 * configured ADMIN_TOKEN environment variable. Returns false immediately when
 * either value is empty, or when the lengths differ.
 */
export function verifyAdminToken(req: NextRequest): boolean {
  const provided = req.headers.get("X-Admin-Token") ?? ""
  if (!ADMIN_TOKEN || !provided) return false
  // Constant-time comparison to prevent timing attacks
  if (provided.length !== ADMIN_TOKEN.length) return false
  let diff = 0
  for (let i = 0; i < provided.length; i++) {
    diff |= provided.charCodeAt(i) ^ ADMIN_TOKEN.charCodeAt(i)
  }
  return diff === 0
}
