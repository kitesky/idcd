// Package idgen generates human-readable prefixed IDs.
// Format: {prefix}{nanoid(12, alphanumeric)} — e.g. "u_aBcD1234efGH".
// All prefixes follow the ID prefix table in docs/prd/15-data-model.md §2.
package idgen

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	nanoid "github.com/matoous/go-nanoid/v2"
)

// alphabet: URL-safe alphanumeric (no ambiguous chars like 0/O, 1/l/I).
const alphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz"
const size = 12

// New returns prefix + 12-char nanoid, e.g. "u_k7mNpQr2xZ9T".
func New(prefix string) string {
	id, err := nanoid.Generate(alphabet, size)
	if err != nil {
		// nanoid only fails on entropy issues — treat as fatal
		panic(fmt.Sprintf("idgen.New: entropy failure: %v", err))
	}
	return prefix + id
}

// --- S1 entity IDs (see 15-data-model §2) ---

func User() string        { return New("u_") }
func Team() string        { return New("t_") }
func APIKey() string      { return New("ak_") }
func Session() string     { return New("s_") }
func Monitor() string     { return New("m_") }
func MonitorCheck() string { return New("mc_") }
func AlertEvent() string  { return New("ae_") }
func AlertPolicy() string { return New("ap_") }
func Channel() string     { return New("ch_") }
func StatusPage() string         { return New("sp_") }
func StatusComponent() string    { return New("sc_") }
func StatusIncident() string     { return New("inc_") }
func StatusSubscription() string { return New("ssub_") }
func ProbeTask() string   { return New("pt_") }
func Report() string      { return New("r_") }
func Order() string       { return New("ord_") }
func Invoice() string     { return New("inv_") }
func Subscription() string { return New("sub_") }
func PaymentMethod() string { return New("pm_") }
func Refund() string      { return New("rf_") }
func Ticket() string      { return New("tk_") }
func AuditLog() string    { return New("al_") }
func WebhookEndpoint() string { return New("we_") }
func Dashboard() string   { return New("db_") }
func UserOTP() string     { return New("otp_") }

// --- S2/S3 entity IDs (v2) ---

func VerdictOrder() string         { return New("v_") }
func VerdictReport() string        { return New("vr_") }
func AttestationRecord() string    { return New("att_") }
func TSAResponse() string          { return New("tsa_") }
func KeyCeremonyLog() string       { return New("kc_") }
func MCPSession() string           { return New("mcps_") }
func MCPToolCall() string          { return New("mctc_") }
func MCPToken() string             { return New("mcpt_") }
func AgentObsMonitor() string      { return New("aom_") }
func AgentObsEvent() string        { return New("aoe_") }
func ComplianceSubscription() string { return New("cs_") }

func OncallSchedule() string    { return New("sch_") }
func OncallParticipant() string { return New("par_") }
func OncallOverride() string    { return New("ovr_") }

func NodeUpgradeRollout() string { return New("oru_") }

// --- Semantic IDs (not nanoid) ---

// Node returns a semantic node ID.
// Format: nd_{countryCode}_{regionCode}_{seq:02d}_{provider}
// Example: "nd_jp_tk_01_vultr"
func Node(countryCode, regionCode string, seq int, provider string) string {
	return fmt.Sprintf("nd_%s_%s_%02d_%s", countryCode, regionCode, seq, provider)
}

// --- Secret values (not IDs, longer and cryptographically random) ---

// APISecret returns a full API secret in idc_live_xxx format.
// The secret is 32 random bytes encoded as hex (64 hex chars).
// Only shown to the user once; store SHA-256(secret) in DB.
func APISecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("idgen.APISecret: %w", err)
	}
	return "idc_live_" + hex.EncodeToString(b), nil
}

// RawSecret returns 32 cryptographically random bytes as a 64-char lowercase hex string.
// Use when you need a plain random secret without a prefix (e.g. node secret keys).
func RawSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("idgen.RawSecret: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// SHA256Hex returns the lowercase hex SHA-256 digest of s.
// Use for hashing secrets before storing them in the database.
func SHA256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// APIKeyPrefix returns the displayable prefix (first 8 hex chars of secret).
// Stored in DB for prefix-based lookup without exposing the full secret.
func APIKeyPrefix(secret string) string {
	// secret format: "idc_live_" + 64 hex chars
	const headerLen = len("idc_live_")
	if len(secret) < headerLen+8 {
		return secret
	}
	return secret[:headerLen+8]
}
