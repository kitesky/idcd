# Build stage
FROM golang:1.26-alpine3.22 AS builder

WORKDIR /app

# Copy workspace files
COPY go.work go.work.sum ./

# Pre-fetch dependencies in a cacheable layer (see apps/agent/Dockerfile for rationale).
COPY apps/notifier/go.mod apps/notifier/go.sum apps/notifier/
COPY lib/ lib/
COPY packages/ packages/
RUN cd apps/notifier && GOWORK=off go mod download

# Copy packages (workspace deps) + remaining notifier source
COPY packages/ packages/
COPY apps/notifier/ apps/notifier/

# Build the binary
WORKDIR /app/apps/notifier
RUN GOWORK=off go build -o /bin/notifier ./cmd/notifier/

# Runtime stage
FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata \
 && adduser -D -u 10001 idcd

COPY --from=builder /bin/notifier /bin/notifier
COPY config/locales.json /config/locales.json
ENV IDCD_REGISTRY_PATH=/config/locales.json

USER 10001:10001

# EXPOSE is informational only; actual port is set via runtime config
# (Observability.PrometheusPort, default 9092). Notifier has no business HTTP
# API — only the Prometheus /metrics listener.
EXPOSE 9092

# Notifier is an Asynq worker (no HTTP listener) — fall back to process liveness check.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD pidof notifier >/dev/null 2>&1 || exit 1

CMD ["/bin/notifier"]
