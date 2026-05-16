module github.com/kite365/idcd/apps/cert-svc

go 1.26

require (
	github.com/alicebob/miniredis/v2 v2.38.0
	github.com/go-chi/chi/v5 v5.2.5
	github.com/jackc/pgx/v5 v5.9.2
	github.com/kite365/idcd/lib/auth v0.0.0
	github.com/kite365/idcd/lib/cert v0.0.0
	github.com/kite365/idcd/lib/shared v0.0.0
	github.com/pashagolub/pgxmock/v4 v4.9.0
	github.com/redis/go-redis/v9 v9.19.0
	github.com/stretchr/testify v1.11.1
	golang.org/x/net v0.53.0
)

require (
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/go-acme/lego/v4 v4.35.2 // indirect
	github.com/go-jose/go-jose/v4 v4.1.4 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/golang-jwt/jwt/v5 v5.2.3 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/miekg/dns v1.1.72 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/yuin/gopher-lua v1.1.1 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel v1.43.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.21.0 // indirect
	go.opentelemetry.io/otel/metric v1.43.0 // indirect
	go.opentelemetry.io/otel/sdk v1.43.0 // indirect
	go.opentelemetry.io/otel/trace v1.43.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/crypto v0.50.0 // indirect
	golang.org/x/mod v0.35.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.44.0 // indirect
	golang.org/x/text v0.36.0 // indirect
	golang.org/x/tools v0.44.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/kite365/idcd/lib/auth => ../../lib/auth
	github.com/kite365/idcd/lib/cert => ../../lib/cert
	github.com/kite365/idcd/lib/shared => ../../lib/shared
)
