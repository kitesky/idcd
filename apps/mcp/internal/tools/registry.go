package tools

import (
	"github.com/kite365/idcd/apps/mcp/internal/protocol"
)

func RegisterAll(srv *protocol.Server) {
	srv.Register(pingDef(), handlePing)
	srv.Register(httpDef(), handleHTTP)
	srv.Register(dnsDef(), handleDNS)
	srv.Register(tracerouteDef(), handleTraceroute)
	srv.Register(sslDef(), handleSSL)
	srv.Register(diagnoseDef(), handleDiagnose)
	srv.Register(ipDef(), handleIP)
	srv.Register(whoisDef(), handleWhois)
}
