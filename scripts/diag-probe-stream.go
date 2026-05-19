//go:build ignore

// One-shot dev script: inspect probe.tasks + probe.results stream health
// to diagnose why a probe task is stuck in "queued" state.
//
// Usage:
//
//	DEV_REDIS_ADDR=host:port DEV_REDIS_PASSWORD=xxx go run scripts/diag-probe-stream.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

func main() {
	addr := os.Getenv("DEV_REDIS_ADDR")
	pw := os.Getenv("DEV_REDIS_PASSWORD")
	if addr == "" || pw == "" {
		log.Fatal("set DEV_REDIS_ADDR and DEV_REDIS_PASSWORD (see config/dev.env.yaml)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: pw,
		DB:       0,
	})
	defer rdb.Close()

	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatal(err)
	}

	for _, stream := range []string{"probe.tasks", "probe.results"} {
		fmt.Printf("\n=== stream: %s ===\n", stream)
		ln, _ := rdb.XLen(ctx, stream).Result()
		fmt.Printf("XLEN: %d\n", ln)

		groups, err := rdb.XInfoGroups(ctx, stream).Result()
		if err != nil {
			fmt.Printf("XINFO GROUPS error: %v\n", err)
			continue
		}
		for _, g := range groups {
			fmt.Printf("  group=%s consumers=%d pending=%d last-delivered-id=%s\n",
				g.Name, g.Consumers, g.Pending, g.LastDeliveredID)
		}

		consumers, _ := rdb.XInfoConsumers(ctx, stream, groups[0].Name).Result()
		for _, c := range consumers {
			fmt.Printf("  consumer=%s pending=%d idle=%dms\n", c.Name, c.Pending, c.Idle)
		}

		// Pending detail — list up to 10 stuck messages
		pending, err := rdb.XPending(ctx, stream, groups[0].Name).Result()
		if err == nil && pending.Count > 0 {
			ext, err2 := rdb.XPendingExt(ctx, &redis.XPendingExtArgs{
				Stream: stream,
				Group:  groups[0].Name,
				Start:  "-",
				End:    "+",
				Count:  10,
			}).Result()
			if err2 == nil {
				for _, e := range ext {
					fmt.Printf("  PEL: id=%s consumer=%s idle=%dms delivered=%d\n",
						e.ID, e.Consumer, e.Idle, e.RetryCount)
				}
			}
		}

		// Last 3 messages content
		last, _ := rdb.XRevRangeN(ctx, stream, "+", "-", 3).Result()
		for _, m := range last {
			fmt.Printf("  recent id=%s values=%v\n", m.ID, m.Values)
		}
	}

	// Check enrolled_nodes for last_seen
	fmt.Println("\n=== agent connectivity (gateway hub key) ===")
	hubKeys, _ := rdb.Keys(ctx, "gateway:hub:*").Result()
	for _, k := range hubKeys {
		v, _ := rdb.Get(ctx, k).Result()
		ttl, _ := rdb.TTL(ctx, k).Result()
		fmt.Printf("  %s = %s (ttl=%v)\n", k, v, ttl)
	}
}
