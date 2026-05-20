# Build stage
FROM golang:1.26-alpine3.22 AS builder

WORKDIR /app

# Copy workspace files
COPY go.work go.work.sum ./

# Pre-fetch dependencies in a cacheable layer (see apps/agent/Dockerfile for rationale).
COPY apps/api/go.mod apps/api/go.sum apps/api/
COPY lib/ lib/
COPY packages/ packages/
RUN cd apps/api && GOWORK=off go mod download

# Copy packages (workspace deps) + remaining api source
COPY packages/ packages/
COPY apps/api/ apps/api/

# Build the binary
WORKDIR /app/apps/api
RUN GOWORK=off go build -o /bin/api ./cmd/api/

# Runtime stage
FROM alpine:3.20

# wget for HEALTHCHECK; ca-certificates / tzdata for TLS + Asia/Shanghai TZ.
RUN apk add --no-cache ca-certificates tzdata wget \
 && adduser -D -u 10001 idcd

COPY --from=builder /bin/api /bin/api
COPY config/locales.json /config/locales.json
ENV IDCD_REGISTRY_PATH=/config/locales.json

# Drop root: container escape no longer escalates to host root.
USER 10001:10001

# EXPOSE is informational only; actual ports are set via runtime config.
# 8080 = HTTP API (Server.Port default; see apps/api/internal/server/server.go).
# 9091 = Prometheus /metrics on an internal-only listener (VPC / loopback ACL).
EXPOSE 8080 9091

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -q -O- http://localhost:8080/health || exit 1

CMD ["/bin/api"]
