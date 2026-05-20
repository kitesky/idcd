.PHONY: build-linux-amd64 build-linux-arm64 test clean install

# Binary name
BINARY=idcd-agent
CMD_DIR=./cmd/agent

# Build for Linux AMD64
build-linux-amd64:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o dist/$(BINARY)-linux-amd64 $(CMD_DIR)

# Build for Linux ARM64
build-linux-arm64:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o dist/$(BINARY)-linux-arm64 $(CMD_DIR)

# Build for current platform
build:
	go build -o dist/$(BINARY) $(CMD_DIR)

# Run tests
test:
	go test ./...

# Run tests with coverage
test-coverage:
	go test -v -cover ./...

# Run tests with detailed coverage report
test-coverage-html:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	rm -rf dist/
	rm -f coverage.out coverage.html

# Create dist directory
dist:
	mkdir -p dist

# Build all targets
build-all: dist build-linux-amd64 build-linux-arm64

# Install locally
install: build
	sudo cp dist/$(BINARY) /usr/local/bin/

# Install systemd service
install-service:
	sudo cp scripts/agent.service /etc/systemd/system/idcd-agent.service
	sudo systemctl daemon-reload

# Development helpers
.PHONY: dev-deps lint fmt

# Install development dependencies
dev-deps:
	go install honnef.co/go/tools/cmd/staticcheck@latest
	go install golang.org/x/tools/cmd/goimports@latest

# Run linters
lint:
	go vet ./...
	staticcheck ./...

# Format code
fmt:
	go fmt ./...
	goimports -w .