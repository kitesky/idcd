# Build stage
FROM golang:1.26-alpine3.20 AS builder

WORKDIR /app

COPY go.work go.work.sum ./

# Pre-fetch dependencies in a cacheable layer (see apps/agent/Dockerfile for rationale).
COPY apps/cert-svc/go.mod apps/cert-svc/go.sum apps/cert-svc/
RUN cd apps/cert-svc && GOWORK=off go mod download

# Copy workspace deps + remaining cert-svc source
COPY packages/ packages/
COPY lib/ lib/
COPY apps/cert-svc/ apps/cert-svc/

WORKDIR /app/apps/cert-svc
RUN go build -o /bin/cert-svc ./cmd/server/ \
 && go build -o /bin/cert-worker ./cmd/worker/ \
 && go build -o /bin/cert-renewer ./cmd/renewer/

# Runtime stage
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata wget \
 && adduser -D -u 10001 idcd

COPY --from=builder /bin/cert-svc /bin/cert-svc
COPY --from=builder /bin/cert-worker /bin/cert-worker
COPY --from=builder /bin/cert-renewer /bin/cert-renewer

USER 10001:10001

# EXPOSE is informational only; actual ports are set via runtime config
# (CERT_SVC_PORT default 8080, CERT_SVC_METRICS_PORT default 9090 — see
# apps/cert-svc/internal/config/config.go).
# 8080 = /v1/cert HTTP API ; 9090 = Prometheus /metrics (separate listener).
EXPOSE 8080 9090

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -q -O- http://localhost:8080/health || exit 1

# Default to the server entrypoint; compose overrides command to switch
# binaries for the worker / renewer replicas.
CMD ["/bin/cert-svc"]
