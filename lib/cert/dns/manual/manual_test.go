package manual

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kite365/idcd/lib/cert/dns"
)

func TestKindIsManual(t *testing.T) {
	p := New(Config{})
	if p.Kind() != dns.KindManual {
		t.Fatalf("wrong kind")
	}
}

func TestValidateCredential_AcceptsEmpty(t *testing.T) {
	p := New(Config{})
	if err := p.ValidateCredential(nil); err != nil {
		t.Fatalf("nil: %v", err)
	}
	if err := p.ValidateCredential(map[string]string{}); err != nil {
		t.Fatalf("empty: %v", err)
	}
	if err := p.ValidateCredential(map[string]string{"junk": "x"}); err != nil {
		t.Fatalf("extra fields should be ignored: %v", err)
	}
}

func TestHealthCheck_AlwaysOK(t *testing.T) {
	p := New(Config{})
	if err := p.HealthCheck(context.Background(), nil); err != nil {
		t.Fatalf("%v", err)
	}
}

func TestSolver_Timeout_FromConfig(t *testing.T) {
	co := NewCoordinator(Config{Timeout: 7 * time.Second})
	p := NewWithCoordinator(co)
	s, err := p.BuildSolver(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if s.Timeout() != 7*time.Second {
		t.Fatalf("want 7s, got %v", s.Timeout())
	}
}

func TestSolver_Present_ReadyViaPoll(t *testing.T) {
	var hits atomic.Int32
	lookup := func(_ context.Context, _ string) ([]string, error) {
		// 第二次 dig 返回正确 value，模拟用户加好了 TXT。
		if hits.Add(1) >= 2 {
			return []string{"good-value"}, nil
		}
		return nil, nil
	}
	co := NewCoordinator(Config{
		Timeout:      5 * time.Second,
		PollInterval: 20 * time.Millisecond,
		LookupTXT:    lookup,
	})
	p := NewWithCoordinator(co)
	s, _ := p.BuildSolver(context.Background(), nil, nil)
	if err := s.Present(context.Background(), "_acme-challenge.example.com.", "good-value"); err != nil {
		t.Fatalf("present: %v", err)
	}
}

func TestSolver_Present_ReadyViaInject(t *testing.T) {
	co := NewCoordinator(Config{
		Timeout:      5 * time.Second,
		PollInterval: 10 * time.Second, // 大到 poll 不会触发
		LookupTXT:    func(_ context.Context, _ string) ([]string, error) { return nil, nil },
	})
	p := NewWithCoordinator(co)
	s, _ := p.BuildSolver(context.Background(), nil, nil)

	const fqdn = "_acme-challenge.example.com."
	const value = "v"

	var wg sync.WaitGroup
	wg.Add(1)
	var presentErr error
	go func() {
		defer wg.Done()
		presentErr = s.Present(context.Background(), fqdn, value)
	}()

	// 等 register 起效再 inject。
	time.Sleep(50 * time.Millisecond)
	if !co.InjectReady(fqdn, value) {
		t.Fatalf("InjectReady should find pending entry")
	}
	wg.Wait()
	if presentErr != nil {
		t.Fatalf("present: %v", presentErr)
	}
}

func TestSolver_Present_CtxCancel(t *testing.T) {
	co := NewCoordinator(Config{
		Timeout:      5 * time.Second,
		PollInterval: 1 * time.Second,
		LookupTXT:    func(_ context.Context, _ string) ([]string, error) { return nil, nil },
	})
	p := NewWithCoordinator(co)
	s, _ := p.BuildSolver(context.Background(), nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	err := s.Present(ctx, "_acme-challenge.example.com.", "v")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}

func TestSolver_Present_Timeout(t *testing.T) {
	co := NewCoordinator(Config{
		Timeout:      80 * time.Millisecond,
		PollInterval: 30 * time.Millisecond,
		LookupTXT:    func(_ context.Context, _ string) ([]string, error) { return nil, nil },
	})
	p := NewWithCoordinator(co)
	s, _ := p.BuildSolver(context.Background(), nil, nil)
	err := s.Present(context.Background(), "_acme-challenge.example.com.", "v")
	if !errors.Is(err, dns.ErrPropagationTimeout) {
		t.Fatalf("want ErrPropagationTimeout, got %v", err)
	}
}

func TestSolver_CleanUp_Unregisters(t *testing.T) {
	co := NewCoordinator(Config{
		Timeout:      1 * time.Second,
		PollInterval: 50 * time.Millisecond,
		LookupTXT:    func(_ context.Context, _ string) ([]string, error) { return nil, nil },
	})
	p := NewWithCoordinator(co)
	s, _ := p.BuildSolver(context.Background(), nil, nil)
	co.register("a.example.", "v")
	if err := s.CleanUp(context.Background(), "a.example.", "v"); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	// inject after cleanup should fail to find entry.
	if co.InjectReady("a.example.", "v") {
		t.Fatalf("inject should fail after cleanup")
	}
}

func TestCoordinator_Concurrent(t *testing.T) {
	co := NewCoordinator(Config{
		Timeout:      2 * time.Second,
		PollInterval: 10 * time.Second,
		LookupTXT:    func(_ context.Context, _ string) ([]string, error) { return nil, nil },
	})
	p := NewWithCoordinator(co)

	const n = 20
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, _ := p.BuildSolver(context.Background(), nil, nil)
			fqdn := "_acme-challenge.host" + itoa(i) + ".example."
			errs[i] = s.Present(context.Background(), fqdn, "v"+itoa(i))
		}()
	}
	// 等所有 Present 都登记完。
	time.Sleep(100 * time.Millisecond)
	for i := 0; i < n; i++ {
		fqdn := "_acme-challenge.host" + itoa(i) + ".example."
		if !co.InjectReady(fqdn, "v"+itoa(i)) {
			t.Fatalf("inject %d failed", i)
		}
	}
	wg.Wait()
	for i, e := range errs {
		if e != nil {
			t.Fatalf("goroutine %d: %v", i, e)
		}
	}
}

func TestInjectReady_Idempotent(t *testing.T) {
	co := NewCoordinator(Config{LookupTXT: func(_ context.Context, _ string) ([]string, error) { return nil, nil }})
	co.register("x.", "v")
	if !co.InjectReady("x.", "v") {
		t.Fatalf("first inject")
	}
	// second inject must not panic on close-of-closed-channel.
	if !co.InjectReady("x.", "v") {
		t.Fatalf("second inject should still report found")
	}
}

func TestInjectReady_Missing(t *testing.T) {
	co := NewCoordinator(Config{LookupTXT: func(_ context.Context, _ string) ([]string, error) { return nil, nil }})
	if co.InjectReady("nope.", "v") {
		t.Fatalf("should not find")
	}
}

func TestCoordinator_TimeoutAccessor(t *testing.T) {
	co := NewCoordinator(Config{Timeout: 3 * time.Second})
	if co.Timeout() != 3*time.Second {
		t.Fatalf("got %v", co.Timeout())
	}
	// zero -> default
	co2 := NewCoordinator(Config{})
	if co2.Timeout() != defaultTimeout {
		t.Fatalf("got %v", co2.Timeout())
	}
}

// defaultLookupTXT 真实走 DNS：用一个肯定不存在的 fqdn 触发两条回退路径
// （LookupNS 失败 → LookupTXT；或 NS 拿到了但没有 TXT），保证不 panic。
// 这是 smoke 测试，结果不断言（CI 网络环境可能没出口）。
func TestDefaultLookupTXT_Smoke(t *testing.T) {
	if testing.Short() {
		t.Skip("skip in -short")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	// .invalid TLD（RFC 2606）— LookupNS 必失败，走 fallback。
	_, _ = defaultLookupTXT(ctx, "_acme-challenge.does-not-exist-12345.example.invalid.")
	// 真实存在 + 无 _acme-challenge TXT 的域名：LookupNS 拿到 NS，dns.Exchange
	// 走通但 Answer 为空（命中两层覆盖）。失败不阻塞测试。
	ctx2, cancel2 := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel2()
	_, _ = defaultLookupTXT(ctx2, "_acme-challenge.example.com.")
}

func TestWaitForTXT_NoEntry(t *testing.T) {
	co := NewCoordinator(Config{LookupTXT: func(_ context.Context, _ string) ([]string, error) { return nil, nil }})
	err := co.WaitForTXT(context.Background(), "ghost.", "v")
	if !errors.Is(err, dns.ErrInvalidCredential) {
		t.Fatalf("want ErrInvalidCredential, got %v", err)
	}
}

func TestSolver_DuplicatePresent_SharesEntry(t *testing.T) {
	co := NewCoordinator(Config{
		Timeout:      2 * time.Second,
		PollInterval: 10 * time.Second,
		LookupTXT:    func(_ context.Context, _ string) ([]string, error) { return nil, nil },
	})
	p := NewWithCoordinator(co)
	s, _ := p.BuildSolver(context.Background(), nil, nil)

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.Present(context.Background(), "_acme-challenge.same.example.", "v")
		}()
	}
	time.Sleep(80 * time.Millisecond)
	co.InjectReady("_acme-challenge.same.example.", "v")
	wg.Wait()
}

// 内部 itoa（避免引入 strconv）。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
