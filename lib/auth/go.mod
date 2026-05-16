module github.com/kite365/idcd/lib/auth

go 1.26

require (
	github.com/golang-jwt/jwt/v5 v5.2.3
	github.com/kite365/idcd/lib/shared v0.0.0-00010101000000-000000000000
)

require (
	github.com/alicebob/miniredis/v2 v2.38.0
	github.com/go-webauthn/webauthn v0.13.4
	github.com/google/uuid v1.6.0
	github.com/redis/go-redis/v9 v9.19.0
	github.com/stretchr/testify v1.11.1
	golang.org/x/crypto v0.51.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fxamacker/cbor/v2 v2.9.0 // indirect
	github.com/go-webauthn/x v0.1.23 // indirect
	github.com/google/go-tpm v0.9.5 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/yuin/gopher-lua v1.1.1 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/sys v0.44.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/kite365/idcd/lib/shared => ../shared
