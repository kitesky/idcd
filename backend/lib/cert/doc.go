// Package cert is the umbrella package for the idcd certificate platform.
//
// Subpackages:
//   - ca:    CA adapter layer (ACME + Reseller). AcmeCA for free CAs (Let's Encrypt,
//           ZeroSSL, Buypass, GTS), ResellerCA reserved for S3 paid channels.
//   - dns:   DNS provider adapter layer for DNS-01 challenge writes. Wraps lego
//           providers plus manual mode.
//   - vault: Encryption layer for private keys and DNS credentials.
//
// See docs/prd/20-free-cert.md for the full module spec.
package cert
