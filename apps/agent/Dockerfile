# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Copy workspace files
COPY go.work go.work.sum ./

# Copy all packages
COPY packages/ packages/

# Copy agent app
COPY apps/agent/ apps/agent/

# Build the binary
WORKDIR /app/apps/agent
RUN go build -o /bin/agent ./cmd/agent/

# Runtime stage
FROM alpine:latest

# wget for HEALTHCHECK on agent's loopback /health endpoint (port 9101).
RUN apk add --no-cache ca-certificates tzdata wget \
 && adduser -D -u 10001 idcd

COPY --from=builder /bin/agent /bin/agent

USER 10001:10001

EXPOSE 8085

# Agent exposes /health on 127.0.0.1:9101 (or AGENT_HEALTH_PORT). See startHealthServer().
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -q -O- http://127.0.0.1:9101/health || exit 1

CMD ["/bin/agent"]
