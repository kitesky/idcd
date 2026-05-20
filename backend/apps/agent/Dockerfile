# Build stage
FROM golang:1.26-alpine3.22 AS builder

WORKDIR /app

# Copy workspace files
COPY go.work go.work.sum ./

# Pre-fetch dependencies in a cacheable layer.
# Copying only go.mod / go.sum first means a source-only change in apps/agent
# (cmd/ internal/ etc.) doesn't bust `go mod download` (~30s saved).
# Use GOWORK=off so the per-module download doesn't require every workspace
# `use` directory to be present yet.
COPY apps/agent/go.mod apps/agent/go.sum apps/agent/
COPY lib/ lib/
RUN cd apps/agent && GOWORK=off go mod download

# Copy packages (workspace deps) + remaining agent source
COPY packages/ packages/
COPY apps/agent/ apps/agent/

# Build the binary
WORKDIR /app/apps/agent
RUN GOWORK=off go build -o /bin/agent ./cmd/agent/

# Runtime stage
FROM alpine:3.20

# wget for HEALTHCHECK on agent's loopback /health endpoint (port 9101).
# libcap supplies setcap; needed to grant CAP_NET_RAW on /bin/agent so the
# agent can open raw ICMP sockets without running as root. Linux's
# unprivileged SOCK_DGRAM+IPPROTO_ICMP path is not enough — the kernel
# rewrites echo.id on send (breaking reply matching) and does not deliver
# ICMP TimeExceeded to dgram sockets (breaking traceroute outright).
# See M8/M9 in docs/MCP-TEST-REPORT-2026-05-20.md.
RUN apk add --no-cache ca-certificates tzdata wget libcap \
 && adduser -D -u 10001 idcd

COPY --from=builder /bin/agent /bin/agent

# File capabilities are dropped by COPY --from, so setcap MUST run after the
# COPY. cap_net_raw=eip = effective+inheritable+permitted, taking effect for
# any process that exec's this binary — including the non-root uid 10001.
RUN setcap cap_net_raw=eip /bin/agent

USER 10001:10001

# EXPOSE is informational only; actual ports are bound at runtime by the agent.
# The agent has no public listener — /health is bound to 127.0.0.1:9101 (or
# AGENT_HEALTH_PORT) for local tooling (idcd-agent-ctl) and the in-container
# HEALTHCHECK below. Nothing should be `docker -p` published.
# (No EXPOSE directive — declaring 8085 was incorrect; agent does not listen there.)

# Agent exposes /health on 127.0.0.1:9101 (or AGENT_HEALTH_PORT). See startHealthServer().
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -q -O- http://127.0.0.1:9101/health || exit 1

CMD ["/bin/agent"]
