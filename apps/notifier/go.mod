module github.com/kite365/idcd/apps/notifier

go 1.26

require (
	github.com/hibiken/asynq v0.25.0
	github.com/jackc/pgx/v5 v5.9.2
	github.com/kite365/idcd/lib/db v0.0.0
	github.com/kite365/idcd/lib/shared v0.0.0
	github.com/wangzheng/payment-go-sdk v0.0.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/matoous/go-nanoid/v2 v2.1.0 // indirect
	github.com/redis/go-redis/v9 v9.19.0 // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	github.com/spf13/cast v1.7.0 // indirect
	go.opentelemetry.io/otel v1.21.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.21.0 // indirect
	go.opentelemetry.io/otel/metric v1.21.0 // indirect
	go.opentelemetry.io/otel/sdk v1.21.0 // indirect
	go.opentelemetry.io/otel/trace v1.21.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.44.0 // indirect
	golang.org/x/text v0.37.0 // indirect
	golang.org/x/time v0.7.0 // indirect
	google.golang.org/protobuf v1.36.8 // indirect
)

replace (
	github.com/kite365/idcd/lib/db => ../../lib/db
	github.com/kite365/idcd/lib/shared => ../../lib/shared
	github.com/wangzheng/payment-go-sdk => ../../packages/payment-go-sdk
)
