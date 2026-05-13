module github.com/kite365/idcd/apps/notifier

go 1.26

require (
	github.com/kite365/idcd/packages/shared v0.0.0
	github.com/kite365/idcd/packages/db v0.0.0
)

replace (
	github.com/kite365/idcd/packages/shared => ../../packages/shared
	github.com/kite365/idcd/packages/db => ../../packages/db
)
