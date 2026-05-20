//go:build ignore

// One-shot dev maintenance: destroys the local-dev gateway-dispatch consumer
// group on Redis so the next gateway start recreates it from "$" — letting
// fresh tool-page probes through without the existing PEL backlog from
// other gateways' nodes blocking them. Safe to re-run; idempotent.
package main

import (
  "context"
  "fmt"
  "os"
  "github.com/redis/go-redis/v9"
)

func main() {
  rdb := redis.NewClient(&redis.Options{
    Addr: "8.163.70.123:6379",
    Password: "Year2025",
  })
  defer rdb.Close()
  ctx := context.Background()
  if _, err := rdb.Ping(ctx).Result(); err != nil {
    fmt.Fprintln(os.Stderr, "ping fail:", err); os.Exit(1)
  }
  n, err := rdb.XGroupDestroy(ctx, "probe.tasks", "gateway-dispatch-localdev").Result()
  fmt.Println("destroyed groups:", n, "err:", err)
}
