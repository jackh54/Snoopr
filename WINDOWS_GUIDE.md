# Snoopr Windows Guide 🪟

Special instructions for running Snoopr on Windows systems.

## Quick Fix for Your Issue

The problems you experienced are now fixed! Here's what was causing them:

1. **Empty Admin Command Prompt:** The old code was incorrectly trying to elevate privileges by opening a new admin window
2. **Socket Error:** Better port checking and error handling has been added
3. **Server Exiting:** Improved error recovery and Windows-specific messaging

## Running the Server on Windows

### Option 1: Regular User (Recommended for Testing)
```cmd
go build -o bin/snoopr-server.exe cmd/server/main.go
bin/snoopr-server.exe
```

You'll see a message about admin privileges, but the server will still work for local testing.

### Option 2: Administrator (Full Functionality)
1. **Right-click Command Prompt → "Run as Administrator"**
2. Navigate to your Snoopr directory
3. Run the server:
```cmd
go build -o bin/snoopr-server.exe cmd/server/main.go
bin/snoopr-server.exe
```

This enables automatic firewall configuration.

## Windows-Specific Features

### Automatic Firewall Configuration
When running as Administrator, Snoopr will:
- ✅ Check if port 8080 is available
- ✅ Remove any existing firewall rules for Snoopr
- ✅ Add a new inbound rule allowing port 8080
- ✅ Provide detailed error messages if something fails

### Port Availability Check
The server now checks if port 8080 is in use before starting and provides helpful error messages.

## Troubleshooting Windows Issues

### Problem: "Port 8080 is already in use"
**Solution:**
```cmd
# Check what's using port 8080
netstat -ano | findstr :8080

# Kill the process (replace PID with actual process ID)
taskkill /F /PID <PID>
```

**Common programs that use port 8080:**
- Other web servers (Apache, Nginx, IIS)
- Development tools (Node.js apps, Docker containers)
- Some Windows services

### Problem: "Access denied" or firewall issues
**Solutions:**
1. **Run as Administrator** (recommended)
2. **Manually configure Windows Firewall:**
   ```cmd
   # Run as Administrator
   netsh advfirewall firewall add rule name="Snoopr Server" dir=in action=allow protocol=TCP localport=8080
   ```
3. **Temporarily disable Windows Firewall** (testing only):
   - Windows Security → Firewall & network protection
   - Turn off for private networks

### Problem: Clients can't connect from other machines
**Solutions:**
1. **Check Windows Firewall** - ensure port 8080 is allowed
2. **Check network connectivity:**
   ```cmd
   # From client machine, test if server port is reachable
   telnet [SERVER_IP] 8080
   ```
3. **Verify server IP:** Make sure you're using the correct IP address in client build

### Problem: Server starts but dashboard won't load
**Solutions:**
1. **Try different browsers** (Chrome, Firefox, Edge)
2. **Check URL:** Use `http://localhost:8080` (not https)
3. **Clear browser cache**
4. **Check if antivirus is blocking** the connection

## Windows Security Considerations

### Antivirus Software
Some antivirus programs may flag the client executable:
- **Windows Defender:** May flag keylogging behavior
- **Third-party AV:** Often more aggressive with unknown executables

**Solutions:**
- Add exclusions for your Snoopr directory
- Use "Allow on device" when prompted
- Temporarily disable real-time protection for testing

### User Account Control (UAC)
- Server runs fine without admin privileges for local testing
- Admin privileges only needed for automatic firewall configuration
- Client requires no special privileges

## Network Configuration

### Local Testing (Same Machine)
- Server: `http://localhost:8080`
- Client: Build with `127.0.0.1` or `localhost`

### Network Testing (Multiple Machines)
1. **Find your Windows IP:**
   ```cmd
   ipconfig | findstr IPv4
   ```

2. **Build client with your IP:**
   ```cmd
   buildclient.bat 192.168.1.100 8080
   ```

3. **Test connectivity:**
   ```cmd
   # From client machine
   ping 192.168.1.100
   telnet 192.168.1.100 8080
   ```

## Performance Tips

### Windows-Specific Optimizations
- **Disable Windows Indexing** for Snoopr directory (faster file operations)
- **Add PowerShell execution policy** if using scripts:
  ```cmd
  powershell -Command "Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser"
  ```

### Resource Monitoring
```cmd
# Monitor server resource usage
tasklist | findstr snoopr-server
wmic process where name="snoopr-server.exe" get ProcessId,PageFileUsage,WorkingSetSize
```

## Development on Windows

### Building from Source
```cmd
# Install Go (if not already installed)
# Download from: https://golang.org/dl/

# Clone and build
git clone <repository>
cd snoopr
go mod tidy
go build -o bin/snoopr-server.exe cmd/server/main.go
```

### Cross-compilation for Windows (from other platforms)
```bash
# From Linux/macOS
GOOS=windows GOARCH=amd64 go build -o bin/snoopr-server.exe cmd/server/main.go
GOOS=windows GOARCH=amd64 go build -ldflags="-H windowsgui" -o bin/snoopr-client.exe cmd/client/main.go
```

## Quick Commands Reference

```cmd
# Check if server is running
netstat -ano | findstr :8080

# Kill server process
taskkill /F /PID <PID>

# Test firewall rule
netsh advfirewall firewall show rule name="Snoopr Server"

# Remove firewall rule
netsh advfirewall firewall delete rule name="Snoopr Server"

# Check Windows version
ver

# Check Go installation
go version
```

## Still Having Issues?

1. **Check the main error message** - the new version provides detailed feedback
2. **Run from Command Prompt** (not PowerShell) for better compatibility
3. **Try a different port** by modifying the source code
4. **Verify Go installation:** `go version`
5. **Check Windows updates** - some older versions have networking issues

The updated code should resolve the specific issues you encountered. Try rebuilding and running the server again! 