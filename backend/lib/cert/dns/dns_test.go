package dns

import (
	"context"
	"errors"
	"testing"

	"github.com/kite365/idcd/lib/cert/ca"
)

type fakeProvider struct {
	kind ProviderKind
}

func (f *fakeProvider) Kind() ProviderKind { return f.kind }
func (f *fakeProvider) BuildSolver(_ context.Context, _ map[string]string, _ []string) (ca.DnsSolver, error) {
	return nil, nil
}
func (f *fakeProvider) ValidateCredential(_ map[string]string) error      { return nil }
func (f *fakeProvider) HealthCheck(_ context.Context, _ map[string]string) error { return nil }

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()
	p := &fakeProvider{kind: KindCloudflare}
	if err := r.Register(p); err != nil {
		t.Fatalf("register: %v", err)
	}
	got, err := r.Get(KindCloudflare)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got != p {
		t.Fatalf("got %v, want %v", got, p)
	}
}

func TestRegistry_GetUnknown(t *testing.T) {
	r := NewRegistry()
	_, err := r.Get(KindCloudflare)
	if !errors.Is(err, ErrProviderNotRegistered) {
		t.Fatalf("want ErrProviderNotRegistered, got %v", err)
	}
}

func TestRegistry_DuplicateRegister(t *testing.T) {
	r := NewRegistry()
	p1 := &fakeProvider{kind: KindManual}
	p2 := &fakeProvider{kind: KindManual}
	if err := r.Register(p1); err != nil {
		t.Fatalf("register first: %v", err)
	}
	err := r.Register(p2)
	if !errors.Is(err, ErrProviderAlreadyRegistered) {
		t.Fatalf("want ErrProviderAlreadyRegistered, got %v", err)
	}
}

func TestRegistry_RegisterNil(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(nil); !errors.Is(err, ErrInvalidCredential) {
		t.Fatalf("want ErrInvalidCredential for nil, got %v", err)
	}
}

func TestRegistry_RegisterEmptyKind(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(&fakeProvider{kind: ""}); !errors.Is(err, ErrInvalidCredential) {
		t.Fatalf("want ErrInvalidCredential for empty kind, got %v", err)
	}
}

func TestRegistry_Kinds(t *testing.T) {
	r := NewRegistry()
	if got := r.Kinds(); len(got) != 0 {
		t.Fatalf("want empty kinds, got %v", got)
	}
	if err := r.Register(&fakeProvider{kind: KindCloudflare}); err != nil {
		t.Fatalf("register cf: %v", err)
	}
	if err := r.Register(&fakeProvider{kind: KindManual}); err != nil {
		t.Fatalf("register manual: %v", err)
	}
	got := r.Kinds()
	if len(got) != 2 {
		t.Fatalf("want 2 kinds, got %d (%v)", len(got), got)
	}
	seen := map[ProviderKind]bool{}
	for _, k := range got {
		seen[k] = true
	}
	if !seen[KindCloudflare] || !seen[KindManual] {
		t.Fatalf("missing kinds in %v", got)
	}
}

// 编译期断言：sentinel error 都被定义且互不相同。
func TestSentinels_Distinct(t *testing.T) {
	all := []error{
		ErrProviderNotRegistered,
		ErrProviderAlreadyRegistered,
		ErrInvalidCredential,
		ErrZoneNotFound,
		ErrUpstreamUnavailable,
		ErrPropagationTimeout,
	}
	for i := range all {
		for j := i + 1; j < len(all); j++ {
			if errors.Is(all[i], all[j]) || errors.Is(all[j], all[i]) {
				t.Fatalf("sentinel collision: %v == %v", all[i], all[j])
			}
		}
	}
}
