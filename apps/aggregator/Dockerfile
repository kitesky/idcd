# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Copy workspace files
COPY go.work go.work.sum ./

# Copy all packages
COPY packages/ packages/

# Copy aggregator app
COPY apps/aggregator/ apps/aggregator/

# Build the binary
WORKDIR /app/apps/aggregator
RUN go build -o /bin/aggregator ./cmd/aggregator/

# Runtime stage
FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata \
 && adduser -D -u 10001 idcd

COPY --from=builder /bin/aggregator /bin/aggregator

USER 10001:10001

EXPOSE 8081

# Aggregator is a pure worker (no HTTP listener) — fall back to a process liveness check.
# pidof is provided by BusyBox in alpine and matches by binary basename.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD pidof aggregator >/dev/null 2>&1 || exit 1

CMD ["/bin/aggregator"]
