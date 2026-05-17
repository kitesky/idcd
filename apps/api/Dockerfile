# Build stage
FROM golang:1.26-alpine3.20 AS builder

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
FROM alpine:3.20

# wget for HEALTHCHECK; ca-certificates / tzdata for TLS + Asia/Shanghai TZ.
RUN apk add --no-cache ca-certificates tzdata wget \
 && adduser -D -u 10001 idcd

COPY --from=builder /bin/api /bin/api

# Drop root: container escape no longer escalates to host root.
USER 10001:10001

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -q -O- http://localhost:8080/health || exit 1

CMD ["/bin/api"]
