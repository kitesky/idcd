// Package manual 实现 dns.Provider 的 manual 模式：平台不写 DNS，由用户
// 手动在控制台加 TXT 记录；solver 阻塞等待 dig 检测到 TXT 传播到位再放行。
//
// 设计要点：
//   - 平台无需任何 credential（ValidateCredential 接受空 map）。
//   - BuildSolver 返回一个绑定 Coordinator 的 solver；Solver.Present 在
//     Coordinator 上挂一个 pending 等待信号；
//   - 同进程内 Coordinator 持有一个 lookupTXT 函数（默认走 miekg/dns 直接
//     问权威 NS），后台 poller goroutine 周期性 dig，命中则通知挂起的 Present
//     返回 nil；
//   - 测试通过 WithLookupTXT 注入伪 lookup 函数，无需起 DNS server。
//
// 提供一个手动模式专用的 InjectReady(ctx, fqdn, value) 钩子给 unit test 用
// （生产代码不该调）。
package manual

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/kite365/idcd/lib/cert/ca"
	"github.com/kite365/idcd/lib/cert/dns"

	mdns "github.com/miekg/dns"
)

const (
	defaultTimeout      = 30 * time.Minute
	defaultPollInterval = 30 * time.Second
)

// LookupTXTFunc 查指定 fqdn 的 TXT 记录，返回 value 列表。
// 默认实现走 net.LookupNS + miekg/dns 直接查权威 NS（绕过缓存 resolver）。
type LookupTXTFunc func(ctx context.Context, fqdn string) ([]string, error)

// Config 配置 Coordinator。
type Config struct {
	// Timeout 是单个 Present 调用最长等待时间；零值用 defaultTimeout（30min）。
	Timeout time.Duration
	// PollInterval 是 dig 轮询周期；零值用 defaultPollInterval（30s）。
	PollInterval time.Duration
	// LookupTXT 用于查 TXT；nil 时用 defaultLookupTXT（miekg/dns 走权威 NS）。
	LookupTXT LookupTXTFunc
}

// New 构造一个 dns.Provider，每次 BuildSolver 返回的 solver 共享同一 Coordinator
// （即同一个 cert-worker 进程内 manual 模式可见性统一）。
func New(cfg Config) dns.Provider {
	co := NewCoordinator(cfg)
	return &manualProvider{co: co}
}

// NewWithCoordinator 给调用方传入已有的 Coordinator（测试 / 多实例场景）。
func NewWithCoordinator(co *Coordinator) dns.Provider {
	return &manualProvider{co: co}
}

type manualProvider struct {
	co *Coordinator
}

func (p *manualProvider) Kind() dns.ProviderKind { return dns.KindManual }

func (p *manualProvider) ValidateCredential(_ map[string]string) error { return nil }

func (p *manualProvider) HealthCheck(_ context.Context, _ map[string]string) error { return nil }

func (p *manualProvider) BuildSolver(_ context.Context, _ map[string]string, _ []string) (ca.DnsSolver, error) {
	return &manualSolver{co: p.co}, nil
}

// BuildSolverWithCoordinator returns a solver bound to the supplied
// Coordinator, ignoring the provider's built-in one. Used by callers
// (e.g. apps/cert-svc orchestrator) that maintain per-order Coordinators
// so HTTP-side MarkManualChallengeReady can unblock the worker.
//
// The provider-level Coordinator is left untouched; this method is the
// preferred wiring path whenever the caller already has a Coordinator
// reference for the specific request.
func (p *manualProvider) BuildSolverWithCoordinator(_ context.Context, _ map[string]string, _ []string, co *Coordinator) (ca.DnsSolver, error) {
	if co == nil {
		return nil, fmt.Errorf("manual: nil coordinator")
	}
	return &manualSolver{co: co}, nil
}

// SolverFromCoordinator is the package-level helper for callers that only
// hold a Coordinator (e.g. apps/cert-svc Service.ManualCoordinator) and
// don't want to pin a dns.Provider type to construct the solver. Returns
// a ca.DnsSolver that drives the supplied Coordinator directly.
func SolverFromCoordinator(co *Coordinator) (ca.DnsSolver, error) {
	if co == nil {
		return nil, fmt.Errorf("manual: nil coordinator")
	}
	return &manualSolver{co: co}, nil
}

// ---- Coordinator ------------------------------------------------------------

// Coordinator 跟踪所有 pending TXT 期望。线程安全。
type Coordinator struct {
	timeout      time.Duration
	pollInterval time.Duration
	lookup       LookupTXTFunc

	mu      sync.Mutex
	pending map[string]*pendingEntry // key = fqdn+"\x00"+value
}

type pendingEntry struct {
	ready chan struct{} // closed when TXT verified / forced ready
}

// NewCoordinator 构造一个独立 Coordinator。
func NewCoordinator(cfg Config) *Coordinator {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	poll := cfg.PollInterval
	if poll <= 0 {
		poll = defaultPollInterval
	}
	lookup := cfg.LookupTXT
	if lookup == nil {
		lookup = defaultLookupTXT
	}
	return &Coordinator{
		timeout:      timeout,
		pollInterval: poll,
		lookup:       lookup,
		pending:      map[string]*pendingEntry{},
	}
}

// Timeout 返回单次 Present 的超时时长，给 ca.DnsSolver.Timeout() 用。
func (c *Coordinator) Timeout() time.Duration { return c.timeout }

func entryKey(fqdn, value string) string { return fqdn + "\x00" + value }

// register 登记一个期望并启动后台 poller；返回的 chan 在 TXT 就绪时被 close。
func (c *Coordinator) register(fqdn, value string) *pendingEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	k := entryKey(fqdn, value)
	if e, ok := c.pending[k]; ok {
		// 同 fqdn+value 多次 Present（lego 不应该这样，但兜底）：复用现有 chan。
		return e
	}
	e := &pendingEntry{ready: make(chan struct{})}
	c.pending[k] = e
	return e
}

func (c *Coordinator) unregister(fqdn, value string) {
	c.mu.Lock()
	delete(c.pending, entryKey(fqdn, value))
	c.mu.Unlock()
}

// InjectReady 是测试 hook：强制把指定 (fqdn, value) 标记为就绪。
// 生产代码不应该调用；poller 在 dig 命中后内部会调它。
func (c *Coordinator) InjectReady(fqdn, value string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.pending[entryKey(fqdn, value)]
	if !ok {
		return false
	}
	select {
	case <-e.ready:
		// 已 close，幂等。
	default:
		close(e.ready)
	}
	return true
}

// WaitForTXT 阻塞直到 TXT 就绪 / ctx 取消 / Coordinator.timeout 触发。
// 在等待期间周期性 dig；命中则 close ready chan。
func (c *Coordinator) WaitForTXT(ctx context.Context, fqdn, value string) error {
	c.mu.Lock()
	e, ok := c.pending[entryKey(fqdn, value)]
	c.mu.Unlock()
	if !ok {
		return fmt.Errorf("%w: no pending entry for %s", dns.ErrInvalidCredential, fqdn)
	}

	deadline := time.Now().Add(c.timeout)
	// 先立即 dig 一次（用户可能已经加好了）。
	if c.checkOnce(ctx, fqdn, value) {
		c.InjectReady(fqdn, value)
	}

	ticker := time.NewTicker(c.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-e.ready:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Until(deadline)):
			return fmt.Errorf("%w: %s", dns.ErrPropagationTimeout, fqdn)
		case <-ticker.C:
			if c.checkOnce(ctx, fqdn, value) {
				c.InjectReady(fqdn, value)
			}
		}
	}
}

// checkOnce 单次 dig；命中返回 true。任何错误吞掉（等下一轮）。
func (c *Coordinator) checkOnce(ctx context.Context, fqdn, value string) bool {
	values, err := c.lookup(ctx, fqdn)
	if err != nil {
		return false
	}
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}

// ---- solver -----------------------------------------------------------------

type manualSolver struct {
	co *Coordinator
}

func (s *manualSolver) Timeout() time.Duration { return s.co.timeout }

// Present 登记 (fqdn, value) 期望并阻塞等待用户加 TXT；lego 会同步等本调用
// 返回再叫 CA 验证。
func (s *manualSolver) Present(ctx context.Context, fqdn, value string) error {
	s.co.register(fqdn, value)
	if err := s.co.WaitForTXT(ctx, fqdn, value); err != nil {
		// ctx 取消归 ctx 错误；poll 超时归 sentinel。
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return err
	}
	return nil
}

func (s *manualSolver) CleanUp(_ context.Context, fqdn, value string) error {
	s.co.unregister(fqdn, value)
	return nil
}

// ---- default lookup ---------------------------------------------------------

// defaultLookupTXT 走权威 NS 直接 dig TXT，绕过本机 resolver 缓存。
// 失败时返回错误；调用方应吞掉重试。
func defaultLookupTXT(ctx context.Context, fqdn string) ([]string, error) {
	name := strings.TrimSuffix(fqdn, ".")
	// 找 zone：从 _acme-challenge.x.y.z.com 向上找权威 NS。最常见情况下
	// _acme-challenge.example.com 的权威是 example.com 的 NS。
	parent := strings.TrimPrefix(name, "_acme-challenge.")
	nsRecords, err := net.DefaultResolver.LookupNS(ctx, parent)
	if err != nil {
		// 找不到就回退到本地 resolver 查。
		return net.DefaultResolver.LookupTXT(ctx, name)
	}
	if len(nsRecords) == 0 {
		return net.DefaultResolver.LookupTXT(ctx, name)
	}

	m := new(mdns.Msg)
	m.SetQuestion(mdns.Fqdn(name), mdns.TypeTXT)
	c := new(mdns.Client)
	c.Timeout = 5 * time.Second
	var out []string
	for _, ns := range nsRecords {
		server := strings.TrimSuffix(ns.Host, ".") + ":53"
		resp, _, qerr := c.ExchangeContext(ctx, m, server)
		if qerr != nil || resp == nil {
			continue
		}
		for _, ans := range resp.Answer {
			if t, ok := ans.(*mdns.TXT); ok {
				out = append(out, t.Txt...)
			}
		}
		if len(out) > 0 {
			return out, nil
		}
	}
	return out, nil
}

// compile-time interface check.
var _ dns.Provider = (*manualProvider)(nil)
var _ ca.DnsSolver = (*manualSolver)(nil)
