// challtestsrv DNS solver helpers.
//
// The Harness methods SetTXT / ClearTXT (in pebble.go) speak the
// challtestsrv management HTTP API directly. This file just documents
// the wire format in one place so future maintainers don't have to
// re-derive it from the upstream Pebble docs.
//
// challtestsrv management API (default port 8055):
//
//	POST /set-txt          {"host": "_acme-challenge.example.com.", "value": "<token>"}
//	POST /clear-txt        {"host": "_acme-challenge.example.com."}
//	POST /add-a            {"host": "...", "addresses": ["1.2.3.4"]}     // unused here
//	POST /add-aaaa         {"host": "...", "addresses": ["::1"]}         // unused here
//
// challtestsrv DNS server (default port 8053) answers queries for
// records set via the management API; Pebble's VA is configured
// (-dnsserver 127.0.0.1:8053) to point at it for DNS-01 validation.

package pebble
