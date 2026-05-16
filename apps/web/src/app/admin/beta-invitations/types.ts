export type InvitationStatus = "pending" | "approved" | "used" | "revoked"

export interface BetaInvitation {
  id: string; code: string; email: string | null; status: InvitationStatus
  requested_by: string | null; used_by?: string | null
  expires_at: string | null; created_at: string
}
