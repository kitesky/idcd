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

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /bin/agent /bin/agent

EXPOSE 8085

CMD ["/bin/agent"]
