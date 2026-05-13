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

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /bin/notifier /bin/notifier

EXPOSE 8082

CMD ["/bin/notifier"]
