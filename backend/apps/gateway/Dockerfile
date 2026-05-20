# Build stage
FROM golang:1.26-alpine3.22 AS builder

WORKDIR /app

# Copy workspace files
COPY go.work go.work.sum ./

# Pre-fetch dependencies in a cacheable layer (see apps/agent/Dockerfile for rationale).
COPY apps/gateway/go.mod apps/gateway/go.sum apps/gateway/
COPY lib/ lib/
RUN cd apps/gateway && GOWORK=off go mod download

# Copy packages (workspace deps) + remaining gateway source
COPY packages/ packages/
COPY apps/gateway/ apps/gateway/

# Build the binary
WORKDIR /app/apps/gateway
RUN GOWORK=off go build -o /bin/gateway ./cmd/gateway/

# Runtime stage
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata \
 && adduser -D -u 10001 idcd

COPY --from=builder /bin/gateway /bin/gateway

USER 10001:10001

# EXPOSE is informational only; actual ports are set via runtime config
# (listen_addr default ":8081" — see apps/gateway/internal/config/config.go;
#  Observability.PrometheusPort default 9094). Production typically front-ends
# this on :8443 via the reverse proxy, but the in-container listener is :8081.
EXPOSE 8081 9094

# Gateway terminates TLS with client-cert mTLS — wget-based health probe is
# unreliable (would need cert + SNI). Fall back to a process liveness check (BusyBox pidof).
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD pidof gateway >/dev/null 2>&1 || exit 1

CMD ["/bin/gateway"]
