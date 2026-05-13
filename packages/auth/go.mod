module github.com/kite365/idcd/packages/auth

go 1.26

require (
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/kite365/idcd/packages/shared v0.0.0-00010101000000-000000000000
)

require github.com/stretchr/testify v1.11.0

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/crypto v0.51.0 // indirect
	golang.org/x/sys v0.44.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/kite365/idcd/packages/shared => ../shared
