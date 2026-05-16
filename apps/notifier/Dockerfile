# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Copy workspace files
COPY go.work go.work.sum ./

# Copy all packages
COPY packages/ packages/

# Copy notifier app
COPY apps/notifier/ apps/notifier/

# Build the binary
WORKDIR /app/apps/notifier
RUN go build -o /bin/notifier ./cmd/notifier/

# Runtime stage
FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata \
 && adduser -D -u 10001 idcd

COPY --from=builder /bin/notifier /bin/notifier

USER 10001:10001

EXPOSE 8082

# Notifier is an Asynq worker (no HTTP listener) — fall back to process liveness check.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD pidof notifier >/dev/null 2>&1 || exit 1

CMD ["/bin/notifier"]
