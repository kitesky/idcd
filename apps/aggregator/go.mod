module github.com/kite365/idcd/apps/aggregator

go 1.26

require (
	github.com/alicebob/miniredis/v2 v2.38.0
	github.com/jackc/pgx/v5 v5.9.2
	github.com/kite365/idcd/lib/db v0.0.0
	github.com/kite365/idcd/lib/shared v0.0.0
	github.com/redis/go-redis/v9 v9.19.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/matoous/go-nanoid/v2 v2.1.0 // indirect
	github.com/yuin/gopher-lua v1.1.1 // indirect
	go.opentelemetry.io/otel v1.21.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.21.0 // indirect
	go.opentelemetry.io/otel/metric v1.21.0 // indirect
	go.opentelemetry.io/otel/sdk v1.21.0 // indirect
	go.opentelemetry.io/otel/trace v1.21.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.44.0 // indirect
	golang.org/x/text v0.37.0 // indirect
)

replace (
	github.com/kite365/idcd/lib/db => ../../lib/db
	github.com/kite365/idcd/lib/shared => ../../lib/shared
)
