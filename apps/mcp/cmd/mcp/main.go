package main

import (
	"context"
	"os"

	"github.com/kite365/idcd/apps/mcp/internal/protocol"
	"github.com/kite365/idcd/apps/mcp/internal/tools"
)

func main() {
	srv := protocol.NewServer()
	tools.RegisterAll(srv)
	if err := srv.RunStdio(context.Background()); err != nil {
		os.Exit(1)
	}
}
