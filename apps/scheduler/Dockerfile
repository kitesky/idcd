# Build stage
FROM golang:1.26-alpine3.20 AS builder

WORKDIR /app

# Copy workspace files
COPY go.work go.work.sum ./

# Pre-fetch dependencies in a cacheable layer (see apps/agent/Dockerfile for rationale).
COPY apps/scheduler/go.mod apps/scheduler/go.sum apps/scheduler/
RUN cd apps/scheduler && GOWORK=off go mod download

# Copy packages (workspace deps) + remaining scheduler source
COPY packages/ packages/
COPY apps/scheduler/ apps/scheduler/

# Build the binary
WORKDIR /app/apps/scheduler
RUN go build -o /bin/scheduler ./cmd/scheduler/

# Runtime stage
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata \
 && adduser -D -u 10001 idcd

COPY --from=builder /bin/scheduler /bin/scheduler

USER 10001:10001

# EXPOSE is informational only; actual port is set via runtime config
# (Observability.PrometheusPort, default 9093). Scheduler is leader-elected
# with no business HTTP API — only the Prometheus /metrics listener.
EXPOSE 9093

# Scheduler is a leader-elected worker (no HTTP listener) — fall back to process liveness check.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD pidof scheduler >/dev/null 2>&1 || exit 1

CMD ["/bin/scheduler"]
