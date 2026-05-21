package redisutil_test

import (
	"testing"
	"github.com/kite365/idcd/lib/shared/config"
	"github.com/kite365/idcd/lib/shared/redisutil"
	"github.com/redis/go-redis/v9"
)

func TestNewClientFromConfig_SingleNode(t *testing.T) {
	cfg := config.RedisConfig{Addr: "127.0.0.1:6379", Password: "pw", DB: 1}
	client := redisutil.NewClientFromConfig(cfg)
	defer client.Close()
	rc, ok := client.(*redis.Client)
	if !ok {
		t.Fatalf("expected *redis.Client, got %T", client)
	}
	if rc.Options().Addr != "127.0.0.1:6379" {
		t.Errorf("addr = %q, want 127.0.0.1:6379", rc.Options().Addr)
	}
}

func TestNewClientFromConfig_Sentinel(t *testing.T) {
	cfg := config.RedisConfig{
		MasterName:    "idcd-master",
		SentinelAddrs: []string{"127.0.0.1:26379"},
		Password:      "pw",
		DB:            2,
	}
	client := redisutil.NewClientFromConfig(cfg)
	defer client.Close()
	if _, ok := client.(*redis.Client); !ok {
		t.Fatalf("expected *redis.Client, got %T", client)
	}
}

func TestNewClientFromConfig_SentinelFallbackWhenAddrsEmpty(t *testing.T) {
	cfg := config.RedisConfig{MasterName: "idcd-master", Addr: "127.0.0.1:6379"}
	client := redisutil.NewClientFromConfig(cfg)
	defer client.Close()
	rc, ok := client.(*redis.Client)
	if !ok {
		t.Fatalf("expected *redis.Client, got %T", client)
	}
	if rc.Options().Addr != "127.0.0.1:6379" {
		t.Errorf("expected single-node fallback, got addr %q", rc.Options().Addr)
	}
}
