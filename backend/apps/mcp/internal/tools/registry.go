package tools

import (
	"github.com/kite365/idcd/apps/mcp/internal/apiclient"
	"github.com/kite365/idcd/apps/mcp/internal/protocol"
)

func RegisterAll(srv *protocol.Server, client *apiclient.Client) {
	srv.Register(pingDef(), handlePingFunc(client))
	srv.Register(httpDef(), handleHTTPFunc(client))
	srv.Register(dnsDef(), handleDNSFunc(client))
	srv.Register(tracerouteDef(), handleTracerouteFunc(client))
	srv.Register(sslDef(), handleSSLFunc(client))
	srv.Register(diagnoseDef(), handleDiagnoseFunc(client))
	srv.Register(ipDef(), handleIPFunc(client))
	srv.Register(whoisDef(), handleWhoisFunc(client))
}
