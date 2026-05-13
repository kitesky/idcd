module github.com/kite365/idcd/apps/agent

go 1.26

require (
	github.com/kite365/idcd/packages/shared v0.0.0
)

replace (
	github.com/kite365/idcd/packages/shared => ../../packages/shared
)
