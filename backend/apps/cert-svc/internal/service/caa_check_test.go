package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// fakeCAALookup returns a deterministic CAA fixture per zone. Tests
// install it via SetCAALookupForTest. Each entry is "tag:value" so we
// avoid surfacing the caaRecord type to tests.
type fakeCAALookup map[string][]string

func (f fakeCAALookup) ToFunc() caaLookupFunc {
	return func(_ context.Context, zone string) ([]caaRecord, bool, error) {
		zone = strings.TrimSuffix(zone, ".")
		entries, ok := f[zone]
		if !ok {
			return nil, false, nil
		}
		out := make([]caaRecord, 0, len(entries))
		for _, e := range entries {
			parts := strings.SplitN(e, ":", 2)
			if len(parts) != 2 {
				continue
			}
			out = append(out, caaRecord{tag: parts[0], value: parts[1]})
		}
		return out, len(out) > 0, nil
	}
}

func newServiceForCAATest(t *testing.T) (*Service, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	svc := New(Config{Redis: rdb})
	return svc, mr
}

func TestCheckCAA_AllowsLetsEncrypt(t *testing.T) {
	svc, _ := newServiceForCAATest(t)
	restore := SetCAALookupForTest(fakeCAALookup{
		"example.com": {"issue:letsencrypt.org"},
	}.ToFunc())
	defer restore()

	err := svc.CheckCAA(context.Background(), []string{"foo.example.com"}, "letsencrypt")
	require.NoError(t, err)
}

func TestCheckCAA_DeniesWhenOnlyDigiCertAllowed(t *testing.T) {
	svc, _ := newServiceForCAATest(t)
	restore := SetCAALookupForTest(fakeCAALookup{
		"example.com": {"issue:digicert.com"},
	}.ToFunc())
	defer restore()

	err := svc.CheckCAA(context.Background(), []string{"foo.example.com"}, "letsencrypt")
	require.ErrorIs(t, err, ErrCAAForbidden)
}

func TestCheckCAA_NoRecordsAllows(t *testing.T) {
	svc, _ := newServiceForCAATest(t)
	restore := SetCAALookupForTest(fakeCAALookup{}.ToFunc())
	defer restore()

	require.NoError(t, svc.CheckCAA(context.Background(), []string{"foo.example.com"}, "letsencrypt"))
}

func TestCheckCAA_WildcardUsesIssuewild(t *testing.T) {
	svc, _ := newServiceForCAATest(t)
	// Wildcard: issuewild governs; issue ignored.
	restore := SetCAALookupForTest(fakeCAALookup{
		"example.com": {
			"issue:digicert.com",
			"issuewild:letsencrypt.org",
		},
	}.ToFunc())
	defer restore()

	require.NoError(t, svc.CheckCAA(context.Background(), []string{"*.example.com"}, "letsencrypt"))
}

func TestCheckCAA_WildcardFallsBackToIssue(t *testing.T) {
	svc, _ := newServiceForCAATest(t)
	restore := SetCAALookupForTest(fakeCAALookup{
		"example.com": {"issue:letsencrypt.org"},
	}.ToFunc())
	defer restore()

	require.NoError(t, svc.CheckCAA(context.Background(), []string{"*.example.com"}, "letsencrypt"))
}

func TestCheckCAA_WildcardForbiddenByIssuewild(t *testing.T) {
	svc, _ := newServiceForCAATest(t)
	restore := SetCAALookupForTest(fakeCAALookup{
		"example.com": {
			"issue:letsencrypt.org",
			"issuewild:digicert.com",
		},
	}.ToFunc())
	defer restore()

	err := svc.CheckCAA(context.Background(), []string{"*.example.com"}, "letsencrypt")
	require.ErrorIs(t, err, ErrCAAForbidden)
}

func TestCheckCAA_BottomUpStopsAtFirstHit(t *testing.T) {
	// foo.bar.example.com has its own CAA record allowing LE; the parent
	// example.com forbids LE. The bottom-up walk must use foo.bar's
	// rrset and ignore example.com.
	svc, _ := newServiceForCAATest(t)
	restore := SetCAALookupForTest(fakeCAALookup{
		"foo.bar.example.com": {"issue:letsencrypt.org"},
		"example.com":         {"issue:digicert.com"},
	}.ToFunc())
	defer restore()

	require.NoError(t, svc.CheckCAA(context.Background(), []string{"foo.bar.example.com"}, "letsencrypt"))
}

func TestCheckCAA_BottomUpFindsParent(t *testing.T) {
	svc, _ := newServiceForCAATest(t)
	restore := SetCAALookupForTest(fakeCAALookup{
		"example.com": {"issue:digicert.com"},
	}.ToFunc())
	defer restore()

	err := svc.CheckCAA(context.Background(), []string{"deep.sub.example.com"}, "letsencrypt")
	require.ErrorIs(t, err, ErrCAAForbidden)
}

func TestCheckCAA_DNSErrorReturnsCheckFailed(t *testing.T) {
	svc, _ := newServiceForCAATest(t)
	boom := errors.New("dns boom")
	restore := SetCAALookupForTest(func(_ context.Context, _ string) ([]caaRecord, bool, error) {
		return nil, false, boom
	})
	defer restore()

	err := svc.CheckCAA(context.Background(), []string{"foo.example.com"}, "letsencrypt")
	require.ErrorIs(t, err, ErrCAACheckFailed)
}

func TestCheckCAA_CacheHitSkipsLookup(t *testing.T) {
	svc, _ := newServiceForCAATest(t)
	calls := 0
	restore := SetCAALookupForTest(func(_ context.Context, _ string) ([]caaRecord, bool, error) {
		calls++
		return []caaRecord{{tag: "issue", value: "letsencrypt.org"}}, true, nil
	})
	defer restore()

	require.NoError(t, svc.CheckCAA(context.Background(), []string{"example.com"}, "letsencrypt"))
	first := calls
	require.NoError(t, svc.CheckCAA(context.Background(), []string{"example.com"}, "letsencrypt"))
	require.Equal(t, first, calls, "second call must serve from cache")
}

func TestCheckCAA_CacheStoresForbidden(t *testing.T) {
	svc, _ := newServiceForCAATest(t)
	calls := 0
	restore := SetCAALookupForTest(func(_ context.Context, _ string) ([]caaRecord, bool, error) {
		calls++
		return []caaRecord{{tag: "issue", value: "digicert.com"}}, true, nil
	})
	defer restore()

	err1 := svc.CheckCAA(context.Background(), []string{"example.com"}, "letsencrypt")
	require.ErrorIs(t, err1, ErrCAAForbidden)
	first := calls
	err2 := svc.CheckCAA(context.Background(), []string{"example.com"}, "letsencrypt")
	require.ErrorIs(t, err2, ErrCAAForbidden)
	require.Equal(t, first, calls, "second call must serve from cache")
}

func TestCheckCAA_UnknownCAReturnsCheckFailed(t *testing.T) {
	svc, _ := newServiceForCAATest(t)
	restore := SetCAALookupForTest(fakeCAALookup{}.ToFunc())
	defer restore()
	err := svc.CheckCAA(context.Background(), []string{"foo.example.com"}, "novelca")
	require.ErrorIs(t, err, ErrCAACheckFailed)
}

func TestCheckCAA_EmptySAN(t *testing.T) {
	svc, _ := newServiceForCAATest(t)
	require.NoError(t, svc.CheckCAA(context.Background(), nil, "letsencrypt"))
}

func TestCheckCAA_EmptyIssueValueDenies(t *testing.T) {
	// Per RFC 8659 §4.2, "issue ;" with empty CA value means no CA is
	// permitted at all.
	svc, _ := newServiceForCAATest(t)
	restore := SetCAALookupForTest(fakeCAALookup{
		"example.com": {"issue:"},
	}.ToFunc())
	defer restore()

	err := svc.CheckCAA(context.Background(), []string{"foo.example.com"}, "letsencrypt")
	require.ErrorIs(t, err, ErrCAAForbidden)
}

func TestCAACandidates(t *testing.T) {
	require.Equal(t, []string{"a.b.c.com", "b.c.com", "c.com"}, caaCandidates("a.b.c.com"))
	require.Equal(t, []string{"example.com"}, caaCandidates("example.com"))
	require.Nil(t, caaCandidates(""))
}

func TestCheckCAA_RedisDownDoesNotPanic(t *testing.T) {
	// Service without Redis still works; verdict is recomputed every call.
	svc := New(Config{})
	restore := SetCAALookupForTest(fakeCAALookup{
		"example.com": {"issue:letsencrypt.org"},
	}.ToFunc())
	defer restore()
	require.NoError(t, svc.CheckCAA(context.Background(), []string{"foo.example.com"}, "letsencrypt"))
}

func TestCheckCAA_CacheValuesPersistAcrossCalls(t *testing.T) {
	svc, mr := newServiceForCAATest(t)
	restore := SetCAALookupForTest(fakeCAALookup{
		"example.com": {"issue:letsencrypt.org"},
	}.ToFunc())
	defer restore()
	require.NoError(t, svc.CheckCAA(context.Background(), []string{"example.com"}, "letsencrypt"))

	// Walk miniredis keys to confirm the cache row landed.
	keys := mr.Keys()
	var found bool
	for _, k := range keys {
		if strings.HasPrefix(k, "cert:caa:example.com:") {
			found = true
		}
	}
	require.True(t, found, "expected cache key, got %v", keys)
	_ = time.Second
}
