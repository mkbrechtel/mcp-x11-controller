package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/ext/test"
	"github.com/linuxdeepin/go-x11-client/util/keysyms"
	"github.com/modelcontextprotocol/go-sdk/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type X11Controller struct {
	conn        *x.Conn
	screen      *x.Screen
	root        x.Window
	keySymbols  *keysyms.KeySymbols
	connAlive   bool
	errorHandler func(error)
}

type Config struct {
	UseXvfb      bool
	Display      string
	Resolution   string
	WindowManager string
	Program      string
}

var (
	xvfbProcess *exec.Cmd
	wmProcess   *exec.Cmd
	appProcess  *exec.Cmd
)

func NewX11Controller() (*X11Controller, error) {
	conn, err := x.NewConn()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to X11: %w", err)
	}

	setup := conn.GetSetup()
	if len(setup.Roots) == 0 {
		return nil, fmt.Errorf("no screens found")
	}
	screen := &setup.Roots[0]

	// Initialize keyboard symbols
	keySymbols := keysyms.NewKeySymbols(conn)

	// Initialize XTEST extension
	extReply, err := x.QueryExtension(conn, "XTEST").Reply(conn)
	if err != nil {
		return nil, fmt.Errorf("failed to query XTEST extension: %w", err)
	}
	if !extReply.Present {
		return nil, fmt.Errorf("XTEST extension not present")
	}

	// Query XTEST version
	cookie := test.GetVersion(conn, test.MajorVersion, test.MinorVersion)
	reply, err := cookie.Reply(conn)
	if err != nil {
		return nil, fmt.Errorf("failed to get XTEST version: %w", err)
	}
	log.Printf("XTEST extension initialized: v%d.%d", reply.MajorVersion, reply.MinorVersion)

	xc := &X11Controller{
		conn:       conn,
		screen:     screen,
		root:       screen.Root,
		keySymbols: keySymbols,
		connAlive:  true,
	}

	// Set up error handling
	xc.errorHandler = func(err error) {
		log.Printf("X11 error: %v", err)
		xc.connAlive = false
	}

	// Start connection monitor
	go xc.monitorConnection()

	return xc, nil
}

func (xc *X11Controller) Close() {
	xc.connAlive = false
	if xc.conn != nil {
		xc.conn.Close()
	}
}

func (xc *X11Controller) monitorConnection() {
	for xc.connAlive {
		time.Sleep(5 * time.Second)
		// Try a simple operation to check connection health
		_, err := x.QueryPointer(xc.conn, xc.root).Reply(xc.conn)
		if err != nil {
			if xc.errorHandler != nil {
				xc.errorHandler(err)
			}
			return
		}
	}
}

func (xc *X11Controller) checkConnection() error {
	if !xc.connAlive {
		return fmt.Errorf("X11 connection lost")
	}
	return nil
}

func (xc *X11Controller) GetScreenInfo() (map[string]interface{}, error) {
	if err := xc.checkConnection(); err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"width":  xc.screen.WidthInPixels,
		"height": xc.screen.HeightInPixels,
		"root":   uint32(xc.root),
	}, nil
}

func (xc *X11Controller) MoveMouse(xPos, yPos int16) error {
	if err := xc.checkConnection(); err != nil {
		return err
	}
	cookie := x.WarpPointerChecked(xc.conn, 0, xc.root, 0, 0, 0, 0, xPos, yPos)
	return cookie.Check(xc.conn)
}

func (xc *X11Controller) Click(button uint8) error {
	if err := xc.checkConnection(); err != nil {
		return err
	}
	
	// Use XTEST FakeInput for button press
	cookie1 := test.FakeInputChecked(xc.conn, x.ButtonPressEventCode, button, x.TimeCurrentTime, xc.root, 0, 0, 0)
	if err := cookie1.Check(xc.conn); err != nil {
		return fmt.Errorf("failed to send button press: %w", err)
	}
	
	// Small delay between press and release
	time.Sleep(10 * time.Millisecond)
	
	// Use XTEST FakeInput for button release
	cookie2 := test.FakeInputChecked(xc.conn, x.ButtonReleaseEventCode, button, x.TimeCurrentTime, xc.root, 0, 0, 0)
	if err := cookie2.Check(xc.conn); err != nil {
		return fmt.Errorf("failed to send button release: %w", err)
	}
	
	return nil
}

func (xc *X11Controller) ClickAt(xPos, yPos int16, button uint8) error {
	if err := xc.checkConnection(); err != nil {
		return err
	}
	
	// Move mouse to position
	if err := xc.MoveMouse(xPos, yPos); err != nil {
		return err
	}
	
	// Small delay to ensure mouse has moved
	time.Sleep(10 * time.Millisecond)
	
	// Click at the position
	return xc.Click(button)
}

func (xc *X11Controller) TypeText(text string) error {
	if err := xc.checkConnection(); err != nil {
		return err
	}

	// Get current input focus
	focusReply, err := x.GetInputFocus(xc.conn).Reply(xc.conn)
	if err != nil {
		return fmt.Errorf("failed to get input focus: %w", err)
	}
	targetWindow := focusReply.Focus
	if targetWindow == x.None || targetWindow == 1 { // PointerRoot = 1
		targetWindow = xc.root
	}

	// Parse the text for special sequences
	i := 0
	for i < len(text) {
		// Check for modifier sequences like Ctrl+C
		if i+2 < len(text) && text[i+1] == '+' {
			modifier := ""
			if i >= 4 && text[i-4:i] == "Ctrl" {
				modifier = "Ctrl"
				i -= 4
			} else if i >= 3 && text[i-3:i] == "Alt" {
				modifier = "Alt"
				i -= 3
			} else if i >= 5 && text[i-5:i] == "Super" {
				modifier = "Super"
				i -= 5
			}
			
			if modifier != "" && i+2 < len(text) {
				// Skip the '+' and get the next character
				char := rune(text[i+2])
				if err := xc.typeCharWithModifier(targetWindow, char, modifier); err != nil {
					return fmt.Errorf("failed to type %s+%c: %w", modifier, char, err)
				}
				i += 3 // Skip past the modifier key combo
				continue
			}
		}
		
		// Regular character
		if err := xc.typeChar(targetWindow, rune(text[i])); err != nil {
			return fmt.Errorf("failed to type character %c: %w", text[i], err)
		}
		i++
		// Small delay between keystrokes
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}

func (xc *X11Controller) typeChar(window x.Window, char rune) error {
	// Handle special characters
	var keysym x.Keysym
	var needShift bool

	switch char {
	case '\n':
		keysym = keysyms.XK_Return
	case '\t':
		keysym = keysyms.XK_Tab
	case '\b':
		keysym = keysyms.XK_BackSpace
	default:
		// For regular characters, check if we need shift
		keysym = x.Keysym(char)
		// Check if this character requires shift
		lower, _ := keysyms.ConvertCase(keysym)
		if char >= 'A' && char <= 'Z' {
			needShift = true
			keysym = lower // Use lowercase keycode with shift
		} else if char == '!' || char == '@' || char == '#' || char == '$' || 
				  char == '%' || char == '^' || char == '&' || char == '*' ||
				  char == '(' || char == ')' || char == '_' || char == '+' ||
				  char == '{' || char == '}' || char == '|' || char == ':' ||
				  char == '"' || char == '<' || char == '>' || char == '?' ||
				  char == '~' {
			needShift = true
			// Map shifted characters to their base keys
			switch char {
			case '!': keysym = x.Keysym('1')
			case '@': keysym = x.Keysym('2')
			case '#': keysym = x.Keysym('3')
			case '$': keysym = x.Keysym('4')
			case '%': keysym = x.Keysym('5')
			case '^': keysym = x.Keysym('6')
			case '&': keysym = x.Keysym('7')
			case '*': keysym = x.Keysym('8')
			case '(': keysym = x.Keysym('9')
			case ')': keysym = x.Keysym('0')
			case '_': keysym = x.Keysym('-')
			case '+': keysym = x.Keysym('=')
			case '{': keysym = x.Keysym('[')
			case '}': keysym = x.Keysym(']')
			case '|': keysym = x.Keysym('\\')
			case ':': keysym = x.Keysym(';')
			case '"': keysym = x.Keysym('\'')
			case '<': keysym = x.Keysym(',')
			case '>': keysym = x.Keysym('.')
			case '?': keysym = x.Keysym('/')
			case '~': keysym = x.Keysym('`')
			}
		}
	}

	// Get keycodes for this keysym
	keycodes := xc.keySymbols.GetKeycodes(keysym)
	if len(keycodes) == 0 {
		// Try StringToKeycodes as fallback
		var err error
		keycodes, err = xc.keySymbols.StringToKeycodes(string(char))
		if err != nil || len(keycodes) == 0 {
			log.Printf("Warning: no keycode found for character %c (keysym 0x%X)", char, keysym)
			return nil // Skip unmapped characters
		}
	}

	keycode := keycodes[0]

	// Press shift if needed
	if needShift {
		// Get proper shift keycode
		shiftKeycodes := xc.keySymbols.GetKeycodes(keysyms.XK_Shift_L)
		if len(shiftKeycodes) == 0 {
			shiftKeycodes = []x.Keycode{50} // Fallback to common value
		}
		if err := xc.sendKeyEvent(window, shiftKeycodes[0], true); err != nil {
			return err
		}
	}

	// Send key press
	if err := xc.sendKeyEvent(window, keycode, true); err != nil {
		return err
	}

	// Send key release
	if err := xc.sendKeyEvent(window, keycode, false); err != nil {
		return err
	}

	// Release shift if it was pressed
	if needShift {
		// Get proper shift keycode
		shiftKeycodes := xc.keySymbols.GetKeycodes(keysyms.XK_Shift_L)
		if len(shiftKeycodes) == 0 {
			shiftKeycodes = []x.Keycode{50} // Fallback to common value
		}
		if err := xc.sendKeyEvent(window, shiftKeycodes[0], false); err != nil {
			return err
		}
	}

	return nil
}

func (xc *X11Controller) sendKeyEvent(window x.Window, keycode x.Keycode, press bool) error {
	// Use XTEST FakeInput for key events
	eventType := x.KeyPressEventCode
	if !press {
		eventType = x.KeyReleaseEventCode
	}
	
	cookie := test.FakeInputChecked(xc.conn, uint8(eventType), uint8(keycode), x.TimeCurrentTime, xc.root, 0, 0, 0)
	if err := cookie.Check(xc.conn); err != nil {
		return fmt.Errorf("failed to send key event: %w", err)
	}
	
	return nil
}

func (xc *X11Controller) typeCharWithModifier(window x.Window, char rune, modifier string) error {
	// Get modifier keycode
	var modKeysym x.Keysym
	switch modifier {
	case "Ctrl":
		modKeysym = keysyms.XK_Control_L
	case "Alt":
		modKeysym = keysyms.XK_Alt_L
	case "Super":
		modKeysym = keysyms.XK_Super_L
	default:
		return fmt.Errorf("unknown modifier: %s", modifier)
	}

	modKeycodes := xc.keySymbols.GetKeycodes(modKeysym)
	if len(modKeycodes) == 0 {
		return fmt.Errorf("no keycode found for modifier %s", modifier)
	}
	modKeycode := modKeycodes[0]

	// Get character keycode
	keysym := x.Keysym(char)
	// For Ctrl combinations, use lowercase
	if modifier == "Ctrl" && char >= 'A' && char <= 'Z' {
		keysym = x.Keysym(char + 32) // Convert to lowercase
	}

	keycodes := xc.keySymbols.GetKeycodes(keysym)
	if len(keycodes) == 0 {
		return fmt.Errorf("no keycode found for character %c", char)
	}
	keycode := keycodes[0]

	// Press modifier
	if err := xc.sendKeyEvent(window, modKeycode, true); err != nil {
		return err
	}

	// Press key
	if err := xc.sendKeyEvent(window, keycode, true); err != nil {
		return err
	}

	// Release key
	if err := xc.sendKeyEvent(window, keycode, false); err != nil {
		return err
	}

	// Release modifier
	if err := xc.sendKeyEvent(window, modKeycode, false); err != nil {
		return err
	}

	return nil
}


func (xc *X11Controller) captureScreen() (*image.RGBA, error) {
	if err := xc.checkConnection(); err != nil {
		return nil, err
	}
	// Get the image from X11
	width := xc.screen.WidthInPixels
	height := xc.screen.HeightInPixels
	
	// Get image from root window (ZPixmap format = 2)
	cookie := x.GetImage(xc.conn, x.ImageFormatZPixmap, x.Drawable(xc.root), 
		0, 0, width, height, 0xffffffff)
	
	reply, err := cookie.Reply(xc.conn)
	if err != nil {
		return nil, fmt.Errorf("failed to get image: %w", err)
	}
	
	// Create RGBA image
	img := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))
	
	// Convert the image data based on depth
	if reply.Depth == 24 || reply.Depth == 32 {
		// X11 usually returns data in BGRA format with 32-bit alignment
		// Even for 24-bit depth, rows are often padded to 32-bit boundaries
		bytesPerPixel := 4 // Always use 4 bytes per pixel for simplicity
		expectedLen := int(width) * int(height) * bytesPerPixel
		
		if len(reply.Data) < expectedLen {
			// Try with 3 bytes per pixel if data is shorter
			bytesPerPixel = 3
			expectedLen = int(width) * int(height) * bytesPerPixel
		}
		
		for y := 0; y < int(height); y++ {
			for x := 0; x < int(width); x++ {
				offset := (y*int(width) + x) * bytesPerPixel
				if offset+2 < len(reply.Data) {
					// BGRA/BGR to RGBA conversion
					b := reply.Data[offset]
					g := reply.Data[offset+1]
					r := reply.Data[offset+2]
					a := uint8(255)
					if bytesPerPixel == 4 && offset+3 < len(reply.Data) {
						a = reply.Data[offset+3]
					}
					img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: a})
				}
			}
		}
	} else {
		return nil, fmt.Errorf("unsupported image depth: %d", reply.Depth)
	}
	
	return img, nil
}

func (xc *X11Controller) TakeScreenshot(filename string) error {
	if err := xc.checkConnection(); err != nil {
		return err
	}
	img, err := xc.captureScreen()
	if err != nil {
		return err
	}
	
	// Save to file
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()
	
	if err := png.Encode(file, img); err != nil {
		return fmt.Errorf("failed to encode PNG: %w", err)
	}
	
	return nil
}

func (xc *X11Controller) GetScreenshotData() ([]byte, error) {
	if err := xc.checkConnection(); err != nil {
		return nil, err
	}
	img, err := xc.captureScreen()
	if err != nil {
		return nil, err
	}
	
	// Encode to PNG in memory
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("failed to encode PNG: %w", err)
	}
	
	return buf.Bytes(), nil
}

func (xc *X11Controller) StartProgram(program string, args []string) error {
	if err := xc.checkConnection(); err != nil {
		return err
	}
	
	// Check if program exists
	if _, err := exec.LookPath(program); err != nil {
		return fmt.Errorf("program not found: %s", program)
	}
	
	// Start the program in the background
	cmd := exec.Command(program, args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("DISPLAY=%s", os.Getenv("DISPLAY")))
	
	// Detach from parent process
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start program: %w", err)
	}
	
	// Detach the process
	go func() {
		cmd.Wait()
	}()
	
	return nil
}

func startXvfb(config *Config) error {
	// Check if Xvfb is installed
	if _, err := exec.LookPath("Xvfb"); err != nil {
		return fmt.Errorf("Xvfb not found: %w", err)
	}

	// Parse resolution
	parts := strings.Split(config.Resolution, "x")
	if len(parts) != 2 {
		return fmt.Errorf("invalid resolution format: %s", config.Resolution)
	}

	// Start Xvfb
	args := []string{
		config.Display,
		"-screen", "0", config.Resolution + "x24",
		"-ac", // Allow all connections
		"+extension", "GLX",
		"+render",
		"-noreset",
	}

	xvfbProcess = exec.Command("Xvfb", args...)
	xvfbProcess.Stdout = os.Stdout
	xvfbProcess.Stderr = os.Stderr

	if err := xvfbProcess.Start(); err != nil {
		return fmt.Errorf("failed to start Xvfb: %w", err)
	}

	// Set DISPLAY environment variable
	os.Setenv("DISPLAY", config.Display)

	// Wait for Xvfb to start
	time.Sleep(2 * time.Second)

	log.Printf("Started Xvfb on display %s with resolution %s", config.Display, config.Resolution)
	return nil
}

func startWindowManager(config *Config) error {
	if config.WindowManager == "" {
		return nil
	}

	// Check if window manager is installed
	if _, err := exec.LookPath(config.WindowManager); err != nil {
		log.Printf("Window manager %s not found, skipping", config.WindowManager)
		return nil
	}

	wmProcess = exec.Command(config.WindowManager)
	wmProcess.Env = append(os.Environ(), "DISPLAY="+config.Display)
	wmProcess.Stdout = os.Stdout
	wmProcess.Stderr = os.Stderr

	if err := wmProcess.Start(); err != nil {
		return fmt.Errorf("failed to start window manager: %w", err)
	}

	// Wait for window manager to start
	time.Sleep(1 * time.Second)

	log.Printf("Started window manager: %s", config.WindowManager)
	return nil
}

func startProgram(config *Config) error {
	if config.Program == "" {
		return nil
	}

	// Parse program and arguments
	parts := strings.Fields(config.Program)
	if len(parts) == 0 {
		return nil
	}

	// Check if program is installed
	if _, err := exec.LookPath(parts[0]); err != nil {
		log.Printf("Program %s not found, skipping", parts[0])
		return nil
	}

	appProcess = exec.Command(parts[0], parts[1:]...)
	appProcess.Env = append(os.Environ(), "DISPLAY="+config.Display)
	appProcess.Stdout = os.Stdout
	appProcess.Stderr = os.Stderr

	if err := appProcess.Start(); err != nil {
		return fmt.Errorf("failed to start program: %w", err)
	}

	// Wait for program to start
	time.Sleep(2 * time.Second)

	log.Printf("Started program: %s", config.Program)
	return nil
}

func cleanup() {
	// Kill processes in reverse order
	if appProcess != nil && appProcess.Process != nil {
		appProcess.Process.Kill()
		appProcess.Wait()
	}
	if wmProcess != nil && wmProcess.Process != nil {
		wmProcess.Process.Kill()
		wmProcess.Wait()
	}
	if xvfbProcess != nil && xvfbProcess.Process != nil {
		xvfbProcess.Process.Kill()
		xvfbProcess.Wait()
	}
}

func main() {
	// Parse command-line flags
	config := &Config{}
	flag.BoolVar(&config.UseXvfb, "xvfb", true, "Use Xvfb virtual display")
	flag.StringVar(&config.Display, "display", ":99", "X11 display number")
	flag.StringVar(&config.Resolution, "resolution", "1024x768", "Screen resolution (WIDTHxHEIGHT)")
	flag.StringVar(&config.WindowManager, "wm", "openbox", "Window manager to use (e.g., i3, openbox)")
	flag.StringVar(&config.Program, "program", "firefox", "Program to launch")
	flag.Parse()

	// Set up cleanup
	defer cleanup()

	// Handle interrupt signals
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		cleanup()
		os.Exit(0)
	}()

	// Start Xvfb if requested
	if config.UseXvfb {
		if err := startXvfb(config); err != nil {
			log.Fatal(err)
		}
	} else if os.Getenv("DISPLAY") == "" {
		log.Fatal("DISPLAY environment variable not set and -xvfb=false")
	}

	// Start window manager
	if err := startWindowManager(config); err != nil {
		log.Printf("Warning: %v", err)
	}

	// Start program
	if err := startProgram(config); err != nil {
		log.Printf("Warning: %v", err)
	}

	// Connect to X11
	xc, err := NewX11Controller()
	if err != nil {
		log.Fatal(err)
	}
	defer xc.Close()

	// Create MCP server
	impl := &mcp.Implementation{
		Name:    "x11-controller",
		Version: "1.0.0",
	}
	
	server := mcp.NewServer(impl, nil)

	// Get screen info tool
	getScreenInfoTool := &mcp.Tool{
		Name:        "get_screen_info",
		Description: "Get X11 screen information including dimensions",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{},
		},
	}
	
	server.AddTool(getScreenInfoTool, func(ctx context.Context, session *mcp.ServerSession, params *mcp.CallToolParamsFor[map[string]any]) (*mcp.CallToolResultFor[any], error) {
		info, err := xc.GetScreenInfo()
		if err != nil {
			return nil, err
		}
		
		return &mcp.CallToolResultFor[any]{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Screen: %dx%d, Root Window: %d", 
						info["width"], info["height"], info["root"]),
				},
			},
		}, nil
	})

	// Click at coordinates tool (combines move and click)
	clickAtTool := &mcp.Tool{
		Name:        "click_at",
		Description: "Move mouse to coordinates and click",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"x": {Type: "number", Description: "X coordinate"},
				"y": {Type: "number", Description: "Y coordinate"},
				"button": {
					Type:        "number",
					Description: "Button number (1=left, 2=middle, 3=right)",
					Default:     json.RawMessage("1"),
				},
			},
			Required: []string{"x", "y"},
		},
	}
	
	server.AddTool(clickAtTool, func(ctx context.Context, session *mcp.ServerSession, params *mcp.CallToolParamsFor[map[string]any]) (*mcp.CallToolResultFor[any], error) {
		args := params.Arguments
		
		x := int16(args["x"].(float64))
		y := int16(args["y"].(float64))
		button := uint8(1)
		if b, ok := args["button"].(float64); ok {
			button = uint8(b)
		}
		
		if err := xc.ClickAt(x, y, button); err != nil {
			return nil, err
		}
		
		return &mcp.CallToolResultFor[any]{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Clicked button %d at (%d, %d)", button, x, y),
				},
			},
		}, nil
	})

	// Type text tool
	typeTextTool := &mcp.Tool{
		Name:        "type_text",
		Description: "Type text by sending key events",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"text": {Type: "string", Description: "Text to type"},
			},
			Required: []string{"text"},
		},
	}
	
	server.AddTool(typeTextTool, func(ctx context.Context, session *mcp.ServerSession, params *mcp.CallToolParamsFor[map[string]any]) (*mcp.CallToolResultFor[any], error) {
		args := params.Arguments
		
		text := args["text"].(string)
		
		if err := xc.TypeText(text); err != nil {
			return nil, err
		}
		
		return &mcp.CallToolResultFor[any]{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Typed: %s", text),
				},
			},
		}, nil
	})

	// Take screenshot tool
	screenshotTool := &mcp.Tool{
		Name:        "take_screenshot",
		Description: "Take a screenshot of the X11 display",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"filename": {
					Type:        "string",
					Description: "Filename for the screenshot (optional)",
				},
			},
		},
	}
	
	server.AddTool(screenshotTool, func(ctx context.Context, session *mcp.ServerSession, params *mcp.CallToolParamsFor[map[string]any]) (*mcp.CallToolResultFor[any], error) {
		args := params.Arguments
		
		// Get screenshot data
		imageData, err := xc.GetScreenshotData()
		if err != nil {
			return nil, fmt.Errorf("failed to take screenshot: %w", err)
		}
		
		// Also save to file if filename is provided
		if f, ok := args["filename"].(string); ok && f != "" {
			if err := xc.TakeScreenshot(f); err != nil {
				log.Printf("Warning: failed to save screenshot to file: %v", err)
			}
		}
		
		// Return image content directly
		return &mcp.CallToolResultFor[any]{
			Content: []mcp.Content{
				&mcp.ImageContent{
					MIMEType: "image/png",
					Data:     imageData,
				},
			},
		}, nil
	})

	// Start program tool
	startProgramTool := &mcp.Tool{
		Name:        "start_program",
		Description: "Start a desktop program in the background",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"program": {
					Type:        "string",
					Description: "Program name or path to executable",
				},
				"args": {
					Type:        "array",
					Description: "Command line arguments (optional)",
					Items:       &jsonschema.Schema{Type: "string"},
				},
			},
			Required: []string{"program"},
		},
	}
	
	server.AddTool(startProgramTool, func(ctx context.Context, session *mcp.ServerSession, params *mcp.CallToolParamsFor[map[string]any]) (*mcp.CallToolResultFor[any], error) {
		args := params.Arguments
		
		program := args["program"].(string)
		var cmdArgs []string
		
		if argsRaw, ok := args["args"].([]interface{}); ok {
			for _, arg := range argsRaw {
				if s, ok := arg.(string); ok {
					cmdArgs = append(cmdArgs, s)
				}
			}
		}
		
		if err := xc.StartProgram(program, cmdArgs); err != nil {
			return nil, err
		}
		
		return &mcp.CallToolResultFor[any]{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Started program: %s %s", program, strings.Join(cmdArgs, " ")),
				},
			},
		}, nil
	})

	// Start the server
	log.Println("Starting X11 MCP controller...")
	log.Printf("Display: %s, Resolution: %s", config.Display, config.Resolution)
	if config.WindowManager != "" {
		log.Printf("Window Manager: %s", config.WindowManager)
	}
	if config.Program != "" {
		log.Printf("Program: %s", config.Program)
	}
	
	// Use stdio transport
	transport := mcp.NewStdioTransport()
	if err := server.Run(context.Background(), transport); err != nil {
		log.Fatal(err)
	}
}