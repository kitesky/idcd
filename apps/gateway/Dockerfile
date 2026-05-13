# Build stage
FROM golang:1.26-alpine AS builder

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
FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /bin/gateway /bin/gateway

EXPOSE 8083

CMD ["/bin/gateway"]
