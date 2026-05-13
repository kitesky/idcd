# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Copy workspace files
COPY go.work go.work.sum ./

# Copy all packages
COPY packages/ packages/

# Copy api app
COPY apps/api/ apps/api/

# Build the binary
WORKDIR /app/apps/api
RUN go build -o /bin/api ./cmd/api/

# Runtime stage
FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /bin/api /bin/api

EXPOSE 8080

CMD ["/bin/api"]
