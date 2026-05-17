# Build stage
FROM golang:1.26-alpine3.20 AS builder

WORKDIR /app

# Copy workspace files
COPY go.work go.work.sum ./

# Copy all packages
COPY packages/ packages/

# Copy gateway app
COPY apps/gateway/ apps/gateway/

# Build the binary
WORKDIR /app/apps/gateway
RUN go build -o /bin/gateway ./cmd/gateway/

# Runtime stage
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata \
 && adduser -D -u 10001 idcd

COPY --from=builder /bin/gateway /bin/gateway

USER 10001:10001

EXPOSE 8443

# Gateway terminates TLS on 8443 with client-cert mTLS — wget-based health probe is
# unreliable (would need cert + SNI). Fall back to a process liveness check (BusyBox pidof).
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD pidof gateway >/dev/null 2>&1 || exit 1

CMD ["/bin/gateway"]
