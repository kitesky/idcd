package channel

import (
	"testing"
)

func TestValidateWebhookURL(t *testing.T) {
	t.Run("rejects empty", func(t *testing.T) {
		if err := validateWebhookURL(""); err == nil {
			t.Fatal("expected error for empty URL")
		}
	})

	t.Run("rejects http scheme", func(t *testing.T) {
		if err := validateWebhookURL("http://example.com/hook"); err == nil {
			t.Fatal("expected error for http:// scheme")
		}
	})

	t.Run("rejects loopback", func(t *testing.T) {
		if err := validateWebhookURL("https://127.0.0.1/hook"); err == nil {
			t.Fatal("expected error for loopback address")
		}
	})

	t.Run("rejects link-local metadata", func(t *testing.T) {
		if err := validateWebhookURL("https://169.254.169.254/latest/meta-data/"); err == nil {
			t.Fatal("expected error for AWS metadata address")
		}
	})

	t.Run("rejects RFC-1918 class A", func(t *testing.T) {
		if err := validateWebhookURL("https://10.1.2.3/hook"); err == nil {
			t.Fatal("expected error for 10.x.x.x")
		}
	})

	t.Run("rejects RFC-1918 class B", func(t *testing.T) {
		if err := validateWebhookURL("https://172.20.0.1/hook"); err == nil {
			t.Fatal("expected error for 172.16-31.x.x")
		}
	})

	t.Run("rejects RFC-1918 class C", func(t *testing.T) {
		if err := validateWebhookURL("https://192.168.1.100/hook"); err == nil {
			t.Fatal("expected error for 192.168.x.x")
		}
	})

	t.Run("rejects CGNAT", func(t *testing.T) {
		if err := validateWebhookURL("https://100.64.0.1/hook"); err == nil {
			t.Fatal("expected error for CGNAT 100.64.0.0/10")
		}
	})

	t.Run("rejects localhost by name", func(t *testing.T) {
		if err := validateWebhookURL("https://localhost/hook"); err == nil {
			t.Fatal("expected error for localhost hostname")
		}
	})

	t.Run("rejects GCP metadata by name", func(t *testing.T) {
		if err := validateWebhookURL("https://metadata.google.internal/hook"); err == nil {
			t.Fatal("expected error for GCP metadata hostname")
		}
	})

	t.Run("accepts public https", func(t *testing.T) {
		if err := validateWebhookURL("https://hooks.example.com/notify"); err != nil {
			t.Errorf("expected no error for public https, got: %v", err)
		}
	})

	t.Run("accepts public https with path and query", func(t *testing.T) {
		if err := validateWebhookURL("https://open.feishu.cn/open-apis/bot/v2/hook/abc?foo=bar"); err != nil {
			t.Errorf("expected no error for public https with query, got: %v", err)
		}
	})

	t.Run("rejects malformed url", func(t *testing.T) {
		if err := validateWebhookURL("://bad-url"); err == nil {
			t.Fatal("expected error for malformed URL")
		}
	})

	t.Run("rejects missing host", func(t *testing.T) {
		if err := validateWebhookURL("https:///path"); err == nil {
			t.Fatal("expected error for URL with missing host")
		}
	})
}
