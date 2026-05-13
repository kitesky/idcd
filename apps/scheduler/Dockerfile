# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Copy workspace files
COPY go.work go.work.sum ./

# Copy all packages
COPY packages/ packages/

# Copy scheduler app
COPY apps/scheduler/ apps/scheduler/

# Build the binary
WORKDIR /app/apps/scheduler
RUN go build -o /bin/scheduler ./cmd/scheduler/

# Runtime stage
FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /bin/scheduler /bin/scheduler

EXPOSE 8084

CMD ["/bin/scheduler"]
