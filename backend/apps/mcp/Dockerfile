# Build stage
FROM golang:1.26-alpine3.22 AS builder

WORKDIR /app

# Copy workspace files
COPY go.work go.work.sum ./

# Pre-fetch dependencies in a cacheable layer (see apps/agent/Dockerfile for rationale).
COPY apps/mcp/go.mod apps/mcp/go.sum apps/mcp/
COPY lib/ lib/
COPY packages/ packages/
RUN cd apps/mcp && GOWORK=off go mod download

# Copy packages (workspace deps) + remaining mcp source
COPY packages/ packages/
COPY apps/mcp/ apps/mcp/

# Build the binary
WORKDIR /app/apps/mcp
RUN GOWORK=off go build -o /bin/mcp ./cmd/mcp/

# Runtime stage
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata wget \
 && adduser -D -u 10001 idcd

COPY --from=builder /bin/mcp /bin/mcp

USER 10001:10001

# 8082 = HTTP transport (SSE + /messages + /healthz)
EXPOSE 8082

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -q -O- http://localhost:8082/healthz || exit 1

# 默认走 http 模式（容器场景），stdio 留给本地 Claude Desktop 接入
CMD ["/bin/mcp", "--transport", "http", "--port", "8082"]
