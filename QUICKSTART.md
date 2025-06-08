# Snoopr Quick Start Guide 🚀

Get Snoopr running in 5 minutes!

## Prerequisites
- Go 1.21+ installed
- Network access between server and target machines

## Step 1: Download Dependencies
```bash
go mod tidy
```

## Step 2: Start the Server
```bash
# Option A: Using Makefile
make server

# Option B: Manual build and run
go build -o bin/snoopr-server cmd/server/main.go
./bin/snoopr-server
```

The server will start on `http://0.0.0.0:8080`

## Step 3: Access Dashboard

### Local Access
1. Open your browser to `http://localhost:8080`
2. Login with:
   - **Username:** `admin`
   - **Password:** `admin`

### Network Access (NEW!)
1. **Find your server IP:** 
   ```bash
   # On Windows: ipconfig | findstr IPv4
   # On Linux/macOS: ifconfig | grep inet
   ```
2. **Access from any device on the network:** `http://[SERVER_IP]:8080`
   - Example: `http://192.168.1.100:8080`
3. **Works on:** Laptops, phones, tablets, other computers
4. **Same login credentials:** admin / admin

## Step 4: Build Client for Target Machine
Replace `SERVER_IP` with your actual server IP address:

### Windows:
```cmd
buildclient.bat 192.168.1.100 8080
```

### Unix/Linux/macOS:
```bash
./buildclient.sh 192.168.1.100 8080
```

### Using Makefile:
```bash
make client SERVER_IP=192.168.1.100 SERVER_PORT=8080
```

This creates `bin/snoopr-client.exe` configured for your server.

## Step 5: Deploy Client
1. Copy `bin/snoopr-client.exe` to target Windows machine
2. Run the executable
3. Client will automatically:
   - Connect to your server
   - Add itself to Windows startup
   - Begin activity monitoring

### 🔄 Client Updates (NEW!)
- **Running the client again** will update the existing connection
- **No duplicate clients** - smart reconnection system
- **Maintains same client ID** for consistent tracking
- **Seamless updates** without losing connection history

## Step 6: Monitor Activity
In the dashboard you can:
- ✅ View connected clients in real-time
- ✅ **NEW:** Live screen viewing with real-time capture
- ✅ Monitor activity logs (keystrokes, window changes)
- ✅ Send pop-up messages to clients
- ✅ Start/stop logging
- ✅ **NEW:** Start/stop screen sharing
- ✅ Execute remote commands

## Dashboard Features

### Connected Clients Panel
- Shows all active client connections
- Displays hostname, IP, and online status
- Click to select a client for operations

### Controls
- **Send Message:** Pop up message on client screen
- **Start/Stop Logging:** Control activity monitoring
- **Start/Stop Screen Share:** Enable live screen viewing
- **Message Input:** Type custom messages

### Live Screen View
- **Real-time screen capture** from selected client
- **Automatic updates** every 500ms when active
- **JPEG compressed** for efficient transmission
- **🔍 Fullscreen toggle** for detailed viewing
- **Enhanced UI** with modern glassmorphism design
- **Click client + Start Screen Share** to begin viewing

### Activity Logs
- Real-time keystroke logging
- Window activity tracking  
- Command execution results
- Timestamps for all events

## Network Requirements
- Server runs on port 8080 (configurable)
- Clients connect via WebSocket to server
- Ensure firewall allows connections

## Security Notes
⚠️ **Educational Use Only**
- Default credentials: `admin` / `admin` 
- Change immediately in production
- Only use on authorized systems
- Network traffic is unencrypted

## Troubleshooting

**Server won't start:**
- Check port 8080 availability: `netstat -tulpn | grep 8080`
- Try different port in source code

**Client won't connect:**
- Verify server IP/port in client build
- Check network connectivity: `ping SERVER_IP`
- Ensure Windows firewall allows connection

**Build errors:**
- Update Go: `go version` (need 1.21+)
- Clean and retry: `make clean && make deps`

## Quick Commands Reference

```bash
# Build everything
make deps

# Start server
make server

# Build client 
make client SERVER_IP=192.168.1.100 SERVER_PORT=8080

# Clean builds
make clean

# Show help
make help
```

## Next Steps
- Read full [README.md](README.md) for detailed documentation
- Customize dashboard styling in server code
- Add additional monitoring features
- Implement encryption for production use

---
**Remember: This is an educational tool. Use responsibly and only where authorized!** 