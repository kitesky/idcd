# Build stage
FROM golang:1.26-alpine3.20 AS builder

WORKDIR /app

# Copy workspace files
COPY go.work go.work.sum ./

# Pre-fetch dependencies in a cacheable layer (see apps/agent/Dockerfile for rationale).
COPY apps/aggregator/go.mod apps/aggregator/go.sum apps/aggregator/
RUN cd apps/aggregator && GOWORK=off go mod download

# Copy packages (workspace deps) + remaining aggregator source
COPY packages/ packages/
COPY apps/aggregator/ apps/aggregator/

# Build the binary
WORKDIR /app/apps/aggregator
RUN go build -o /bin/aggregator ./cmd/aggregator/

# Runtime stage
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata \
 && adduser -D -u 10001 idcd

COPY --from=builder /bin/aggregator /bin/aggregator

USER 10001:10001

# EXPOSE is informational only; actual port is set via runtime config
# (Observability.PrometheusPort, default 9091). Aggregator only exposes a
# Prometheus /metrics listener — no business HTTP API.
EXPOSE 9091

# Aggregator is a pure worker (no HTTP listener) — fall back to a process liveness check.
# pidof is provided by BusyBox in alpine and matches by binary basename.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD pidof aggregator >/dev/null 2>&1 || exit 1

CMD ["/bin/aggregator"]
