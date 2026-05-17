module github.com/kite365/idcd/apps/attest

go 1.26

require (
	github.com/digitorus/pkcs7 v0.0.0-20230818184609-3a137a874352
	github.com/digitorus/timestamp v0.0.0-20250524132541-c45532741eea
	github.com/jackc/pgx/v5 v5.9.2
	github.com/kite365/idcd/lib/attest v0.0.0
	github.com/pashagolub/pgxmock/v4 v4.9.0
	github.com/stretchr/testify v1.11.1
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/digitorus/pdf v0.1.2 // indirect
	github.com/digitorus/pdfsign v0.0.0-20260407063256-85ede6424a74 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/mattetti/filebuffer v1.0.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/crypto v0.49.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/text v0.35.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/kite365/idcd/lib/attest => ../../lib/attest
