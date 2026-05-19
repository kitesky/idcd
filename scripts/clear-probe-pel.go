//go:build ignore

// One-shot dev script: clear the probe.tasks stream + its consumer group so
// the agent stops grinding through a backlog of stale tasks.
//
// Reads credentials from env (matches the layout of config/dev.env.yaml so
// no separate secret management is needed):
//
//	DEV_REDIS_ADDR     — e.g. 8.163.70.123:6379
//	DEV_REDIS_PASSWORD — e.g. ${REDIS_PASSWORD from dev.env.yaml}
//
// Usage:
//
//	DEV_REDIS_ADDR=host:port DEV_REDIS_PASSWORD=xxx go run scripts/clear-probe-pel.go
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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

	streams := []string{"probe.tasks", "probe.results"}
	for _, s := range streams {
		before, _ := rdb.XLen(ctx, s).Result()
		if err := rdb.Del(ctx, s).Err(); err != nil {
			fmt.Printf("DEL %s: %v\n", s, err)
		} else {
			fmt.Printf("DEL %s (had %d entries)\n", s, before)
		}
	}

	// probe_task table — leave it; new tasks will get fresh rows.
	fmt.Println("done. agent will recreate the streams + consumer groups on next message.")
}
