// Package idempcache provides a bounded in-process cache for KMS
// idempotency-key → signature mappings, shared by the awskms and alikms
// adapters.
//
// Why a bounded cache instead of sync.Map:
//
// The D4 WAL guarantees that a verdict_report.id only ever generates a
// finite number of distinct idempotencyKeys (currently 2 — ":signed"
// and ":embed"). In practice the cache stays tiny because each report's
// keys are referenced for at most a few seconds during the pipeline. A
// raw sync.Map would still work correctly, but a long-running worker
// (S2 KMS instance lifecycle is ~90 days per D11) accumulates one entry
// per report forever, which is wasted heap.
//
// The cache holds (idempotencyKey, signature) entries up to a hard cap.
// On overflow we evict the oldest insertion (FIFO). KMS-side idempotency
// windows are 5-15 minutes for AWS / Aliyun; with cap=10000 entries we
// retain at least the most recent ~10k signatures, far longer than the
// KMS dedup window, so an eviction can never cause a duplicate-signing
// audit log entry — by the time we'd re-sign that key, KMS has already
// forgotten it too.
//
// FIFO eviction (not LRU) keeps the implementation to ~50 lines and
// avoids needing a doubly-linked list. On the verdict pipeline a key is
// used twice within seconds and then never again, so FIFO and LRU are
// indistinguishable in practice.
package idempcache

import (
	"container/list"
	"sync"
)

// DefaultCapacity is the cap used when Config.Capacity is zero. Sized
// for ~10k recent signatures — well above the KMS-side idempotency
// window even at peak verdict generation rates.
const DefaultCapacity = 10000

// Cache is a bounded (idempotencyKey → signature) map with FIFO
// eviction. The zero value is not usable; construct via New.
//
// Cache is safe for concurrent use. Both lookups and stores take a
// single mutex; we did benchmark a sync.Map variant but the lock
// contention is negligible (signing latency is dominated by KMS network
// I/O, not the cache lookup).
type Cache struct {
	cap int

	mu    sync.Mutex
	order *list.List               // FIFO of *entry; front = oldest
	index map[string]*list.Element // key → element in `order`
}

// entry pairs the key (so eviction can remove from the index) with the
// signature bytes. We keep our own copy of the bytes so callers cannot
// mutate the cached signature out from under us.
type entry struct {
	key string
	sig []byte
}

// New constructs a Cache. Pass capacity = 0 (or a negative value) to
// use DefaultCapacity. The returned Cache is safe for concurrent use
// from any number of goroutines.
func New(capacity int) *Cache {
	if capacity <= 0 {
		capacity = DefaultCapacity
	}
	return &Cache{
		cap:   capacity,
		order: list.New(),
		index: make(map[string]*list.Element, capacity),
	}
}

// Load returns the cached signature for key, or (nil, false) if the
// key is not present. The returned slice is a defensive copy — callers
// can mutate / append to it freely.
func (c *Cache) Load(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.index[key]
	if !ok {
		return nil, false
	}
	sig := el.Value.(*entry).sig
	cp := make([]byte, len(sig))
	copy(cp, sig)
	return cp, true
}

// LoadOrStore mirrors sync.Map.LoadOrStore. If key already exists the
// stored signature is returned with loaded=true and `sig` is ignored.
// Otherwise sig is stored (as a defensive copy) and returned with
// loaded=false.
//
// When a store causes the cache to exceed its capacity, the oldest
// entry is evicted. The eviction is recorded internally only — callers
// do not need to handle eviction explicitly because KMS-side dedup
// guarantees correctness past the cache horizon (see package doc).
func (c *Cache) LoadOrStore(key string, sig []byte) (stored []byte, loaded bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.index[key]; ok {
		existing := el.Value.(*entry).sig
		cp := make([]byte, len(existing))
		copy(cp, existing)
		return cp, true
	}

	cp := make([]byte, len(sig))
	copy(cp, sig)
	el := c.order.PushBack(&entry{key: key, sig: cp})
	c.index[key] = el

	// Evict from the front while we are over capacity. A single store
	// can only push us 1 over, but loop defensively for safety in case
	// callers ever shrink the cap.
	for c.order.Len() > c.cap {
		oldest := c.order.Front()
		if oldest == nil {
			break
		}
		ent := oldest.Value.(*entry)
		delete(c.index, ent.key)
		c.order.Remove(oldest)
	}

	out := make([]byte, len(cp))
	copy(out, cp)
	return out, false
}

// Len returns the current number of entries. Useful for tests and
// observability.
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}

// Capacity returns the configured maximum. Exposed for tests.
func (c *Cache) Capacity() int { return c.cap }
