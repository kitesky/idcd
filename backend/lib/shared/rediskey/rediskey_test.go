package rediskey_test

import (
	"testing"

	"github.com/kite365/idcd/lib/shared/rediskey"
)

func TestCertExpiringNotifiedKey(t *testing.T) {
	got := rediskey.CertExpiringNotifiedKey(11, 30)
	want := "cert:expiring:notified:11:30"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestCertRenewalFailedNotifiedKey(t *testing.T) {
	got := rediskey.CertRenewalFailedNotifiedKey(500)
	want := "cert:renewal_failed:notified:500"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// 防止后续 schema 漂移 — 这两个 prefix 是 cert-svc + notifier + 监控脚本
// 共享的"事实标准"，变更需要协调所有读取方。
func TestKeyPrefixesStable(t *testing.T) {
	if k := rediskey.CertExpiringNotifiedKey(1, 1); k[:len("cert:expiring:notified:")] != "cert:expiring:notified:" {
		t.Fatalf("expiring key prefix changed: %q", k)
	}
	if k := rediskey.CertRenewalFailedNotifiedKey(1); k[:len("cert:renewal_failed:notified:")] != "cert:renewal_failed:notified:" {
		t.Fatalf("renewal_failed key prefix changed: %q", k)
	}
}
