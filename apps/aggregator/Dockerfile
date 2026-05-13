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

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /bin/aggregator /bin/aggregator

EXPOSE 8081

CMD ["/bin/aggregator"]
