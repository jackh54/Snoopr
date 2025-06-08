.PHONY: server client deps clean help

# Default target
help:
	@echo "Snoopr - Cybersecurity Educational Tool"
	@echo ""
	@echo "Available targets:"
	@echo "  deps        - Install Go dependencies"
	@echo "  server      - Build and run server"
	@echo "  client      - Build client (requires SERVER_IP and SERVER_PORT)"
	@echo "  clean       - Clean build artifacts"
	@echo "  help        - Show this help message"
	@echo ""
	@echo "Usage examples:"
	@echo "  make deps"
	@echo "  make server"
	@echo "  make client SERVER_IP=192.168.1.100 SERVER_PORT=8080"

# Install dependencies
deps:
	@echo "Installing Go dependencies..."
	go mod tidy
	go mod download

# Build and run server
server: deps
	@echo "Building Snoopr server..."
	go build -o bin/snoopr-server cmd/server/main.go
	@echo "Starting server on port 8080..."
	@echo "Dashboard: http://localhost:8080"
	@echo "Credentials: admin / admin"
	./bin/snoopr-server

# Build client (requires SERVER_IP and SERVER_PORT variables)
client: deps
ifndef SERVER_IP
	$(error SERVER_IP is not set. Usage: make client SERVER_IP=192.168.1.100 SERVER_PORT=8080)
endif
ifndef SERVER_PORT
	$(error SERVER_PORT is not set. Usage: make client SERVER_IP=192.168.1.100 SERVER_PORT=8080)
endif
	@echo "Building client for $(SERVER_IP):$(SERVER_PORT)..."
	@mkdir -p bin
	@cp cmd/client/main.go cmd/client/main_build.go
	@sed -i.bak 's/SERVER_IP_PLACEHOLDER/$(SERVER_IP)/g' cmd/client/main_build.go
	@sed -i.bak 's/SERVER_PORT_PLACEHOLDER/$(SERVER_PORT)/g' cmd/client/main_build.go
	GOOS=windows GOARCH=amd64 go build -ldflags="-H windowsgui -s -w" -o bin/snoopr-client.exe cmd/client/main_build.go
	@rm -f cmd/client/main_build.go cmd/client/main_build.go.bak
	@echo "Client built successfully: bin/snoopr-client.exe"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	rm -f cmd/client/main_build.go*
	go clean

# Create necessary directories
init:
	@mkdir -p bin
	@mkdir -p web/static 