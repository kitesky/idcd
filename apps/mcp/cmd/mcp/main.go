package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/kite365/idcd/apps/mcp/internal/apiclient"
	"github.com/kite365/idcd/apps/mcp/internal/protocol"
	"github.com/kite365/idcd/apps/mcp/internal/tools"
)

func main() {
	apiKey := flag.String("api-key", "", "API key (overrides IDCD_API_KEY)")
	apiURL := flag.String("api-url", "", "API base URL (overrides IDCD_API_URL)")
	transport := flag.String("transport", "stdio", "Transport: stdio or http")
	port := flag.Int("port", 8082, "HTTP port (only for --transport http)")
	flag.Parse()

	key := *apiKey
	if key == "" {
		key = os.Getenv("IDCD_API_KEY")
	}

	baseURL := *apiURL
	if baseURL == "" {
		baseURL = os.Getenv("IDCD_API_URL")
	}
	if baseURL == "" {
		baseURL = "https://api.idcd.com"
	}

	client := apiclient.New(baseURL, key)
	srv := protocol.NewServer()
	tools.RegisterAll(srv, client)

	switch *transport {
	case "http":
		addr := fmt.Sprintf(":%d", *port)
		mux := http.NewServeMux()
		mux.Handle("/sse", protocol.SSEHandler(srv))
		mux.Handle("/messages", protocol.MessagesHandler(srv))
		if err := http.ListenAndServe(addr, mux); err != nil {
			os.Exit(1)
		}
	default:
		if err := srv.RunStdio(context.Background()); err != nil {
			os.Exit(1)
		}
	}
}
