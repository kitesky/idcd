module github.com/kite365/idcd/apps/notifier

go 1.26

require (
	github.com/hibiken/asynq v0.25.0
	github.com/kite365/idcd/packages/shared v0.0.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/redis/go-redis/v9 v9.19.0 // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	github.com/spf13/cast v1.7.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
	golang.org/x/time v0.7.0 // indirect
	google.golang.org/protobuf v1.35.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/kite365/idcd/packages/shared => ../../packages/shared
