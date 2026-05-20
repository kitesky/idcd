package idempcache

import (
	"bytes"
	"fmt"
	"sync"
	"testing"
)

func TestNew_DefaultCapacity(t *testing.T) {
	c := New(0)
	if c.Capacity() != DefaultCapacity {
		t.Fatalf("Capacity() = %d, want %d", c.Capacity(), DefaultCapacity)
	}
	c2 := New(-5)
	if c2.Capacity() != DefaultCapacity {
		t.Fatalf("negative cap: Capacity() = %d, want %d", c2.Capacity(), DefaultCapacity)
	}
}

func TestNew_CustomCapacity(t *testing.T) {
	c := New(42)
	if c.Capacity() != 42 {
		t.Fatalf("Capacity() = %d, want 42", c.Capacity())
	}
	if c.Len() != 0 {
		t.Fatalf("Len() = %d, want 0", c.Len())
	}
}

func TestLoad_Missing(t *testing.T) {
	c := New(10)
	if _, ok := c.Load("absent"); ok {
		t.Fatal("Load missing key returned ok=true")
	}
}

func TestLoadOrStore_FreshKey(t *testing.T) {
	c := New(10)
	sig := []byte{1, 2, 3}
	got, loaded := c.LoadOrStore("k1", sig)
	if loaded {
		t.Fatal("loaded=true for fresh key")
	}
	if !bytes.Equal(got, sig) {
		t.Fatalf("got %x, want %x", got, sig)
	}
	if c.Len() != 1 {
		t.Fatalf("Len = %d, want 1", c.Len())
	}
}

func TestLoadOrStore_ExistingKeyReturnsOriginal(t *testing.T) {
	c := New(10)
	first := []byte{1, 2, 3}
	c.LoadOrStore("k1", first)
	second := []byte{9, 9, 9}
	got, loaded := c.LoadOrStore("k1", second)
	if !loaded {
		t.Fatal("loaded=false for existing key")
	}
	if !bytes.Equal(got, first) {
		t.Fatalf("got %x, want %x (the original)", got, first)
	}
}

func TestLoad_DefensiveCopy(t *testing.T) {
	c := New(10)
	stored := []byte{1, 2, 3}
	c.LoadOrStore("k", stored)
	stored[0] = 99 // mutate caller's slice

	got, ok := c.Load("k")
	if !ok {
		t.Fatal("Load failed")
	}
	if got[0] == 99 {
		t.Fatalf("cache was mutated through caller's slice: got %x", got)
	}

	// Mutate the returned slice; subsequent Load must still be clean.
	got[0] = 77
	again, _ := c.Load("k")
	if again[0] == 77 {
		t.Fatalf("cache was mutated through returned slice: got %x", again)
	}
}

func TestEviction_FIFO(t *testing.T) {
	c := New(3)
	c.LoadOrStore("a", []byte{1})
	c.LoadOrStore("b", []byte{2})
	c.LoadOrStore("c", []byte{3})
	if c.Len() != 3 {
		t.Fatalf("Len = %d, want 3", c.Len())
	}
	// Inserting d should evict a (oldest).
	c.LoadOrStore("d", []byte{4})
	if c.Len() != 3 {
		t.Fatalf("after evict Len = %d, want 3", c.Len())
	}
	if _, ok := c.Load("a"); ok {
		t.Fatal("oldest key 'a' was not evicted")
	}
	for _, k := range []string{"b", "c", "d"} {
		if _, ok := c.Load(k); !ok {
			t.Fatalf("key %q missing after eviction", k)
		}
	}
}

func TestEviction_DoesNotResetOnRead(t *testing.T) {
	// Confirm FIFO (not LRU): reading 'a' does not save it from eviction.
	c := New(2)
	c.LoadOrStore("a", []byte{1})
	c.LoadOrStore("b", []byte{2})
	_, _ = c.Load("a") // touch
	c.LoadOrStore("c", []byte{3})
	if _, ok := c.Load("a"); ok {
		t.Fatal("FIFO violated: 'a' survived even though it was the oldest insertion")
	}
}

func TestConcurrent_LoadOrStoreSingleKey(t *testing.T) {
	c := New(100)
	const N = 50
	var wg sync.WaitGroup
	wg.Add(N)
	var loadedCount, freshCount int32
	var mu sync.Mutex
	for i := 0; i < N; i++ {
		go func(i int) {
			defer wg.Done()
			_, loaded := c.LoadOrStore("hot", []byte{byte(i)})
			mu.Lock()
			if loaded {
				loadedCount++
			} else {
				freshCount++
			}
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	// Exactly one goroutine should have stored; the rest see the cached value.
	if freshCount != 1 {
		t.Fatalf("freshCount = %d, want 1", freshCount)
	}
	if loadedCount != N-1 {
		t.Fatalf("loadedCount = %d, want %d", loadedCount, N-1)
	}
}

func TestConcurrent_FillBeyondCapacity(t *testing.T) {
	const cap = 16
	c := New(cap)
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c.LoadOrStore(fmt.Sprintf("k%d", i), []byte{byte(i)})
		}(i)
	}
	wg.Wait()
	if c.Len() != cap {
		t.Fatalf("after 200 inserts at cap=%d, Len = %d", cap, c.Len())
	}
}

func TestLen_TracksCorrectly(t *testing.T) {
	c := New(100)
	for i := 0; i < 50; i++ {
		c.LoadOrStore(fmt.Sprintf("k%d", i), []byte{1})
	}
	if c.Len() != 50 {
		t.Fatalf("Len = %d, want 50", c.Len())
	}
	// Re-stores don't grow Len.
	for i := 0; i < 50; i++ {
		c.LoadOrStore(fmt.Sprintf("k%d", i), []byte{2})
	}
	if c.Len() != 50 {
		t.Fatalf("after re-store Len = %d, want 50", c.Len())
	}
}
