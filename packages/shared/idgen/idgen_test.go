package idgen_test

import (
	"strings"
	"testing"

	"github.com/kite365/idcd/packages/shared/idgen"
)

func TestNew_prefix(t *testing.T) {
	id := idgen.User()
	if !strings.HasPrefix(id, "u_") {
		t.Fatalf("expected u_ prefix, got %q", id)
	}
	if len(id) != len("u_")+12 {
		t.Fatalf("expected length %d, got %d", len("u_")+12, len(id))
	}
}

func TestNew_unique(t *testing.T) {
	seen := make(map[string]bool, 1000)
	for range 1000 {
		id := idgen.User()
		if seen[id] {
			t.Fatalf("collision on %q", id)
		}
		seen[id] = true
	}
}

func TestAPISecret(t *testing.T) {
	secret, err := idgen.APISecret()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(secret, "idc_live_") {
		t.Fatalf("expected idc_live_ prefix, got %q", secret)
	}
	// idc_live_ (9 chars) + 64 hex chars
	if len(secret) != 9+64 {
		t.Fatalf("expected len %d, got %d", 9+64, len(secret))
	}
}

func TestAPIKeyPrefix(t *testing.T) {
	secret, _ := idgen.APISecret()
	prefix := idgen.APIKeyPrefix(secret)
	if !strings.HasPrefix(prefix, "idc_live_") {
		t.Fatalf("prefix should start with idc_live_, got %q", prefix)
	}
	if len(prefix) != 9+8 {
		t.Fatalf("expected len %d, got %d", 9+8, len(prefix))
	}
}

func TestNode(t *testing.T) {
	id := idgen.Node("jp", "tk", 1, "vultr")
	if id != "nd_jp_tk_01_vultr" {
		t.Fatalf("unexpected node ID: %q", id)
	}
}
