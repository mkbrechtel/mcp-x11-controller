package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	x "github.com/linuxdeepin/go-x11-client"
	"github.com/modelcontextprotocol/go-sdk/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type X11Controller struct {
	conn   *x.Conn
	screen *x.Screen
	root   x.Window
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

	return &X11Controller{
		conn:   conn,
		screen: screen,
		root:   screen.Root,
	}, nil
}

func (xc *X11Controller) Close() {
	if xc.conn != nil {
		xc.conn.Close()
	}
}

func (xc *X11Controller) GetScreenInfo() (map[string]interface{}, error) {
	return map[string]interface{}{
		"width":  xc.screen.WidthInPixels,
		"height": xc.screen.HeightInPixels,
		"root":   uint32(xc.root),
	}, nil
}

func (xc *X11Controller) MoveMouse(xPos, yPos int16) error {
	cookie := x.WarpPointerChecked(xc.conn, 0, xc.root, 0, 0, 0, 0, xPos, yPos)
	return cookie.Check(xc.conn)
}

func (xc *X11Controller) Click(button uint8) error {
	// Get current pointer position
	reply, err := x.QueryPointer(xc.conn, xc.root).Reply(xc.conn)
	if err != nil {
		return err
	}

	// Send button press event
	pressEvent := make([]byte, 32)
	pressEvent[0] = x.ButtonPressEventCode
	pressEvent[1] = button
	x.Put32(pressEvent[4:], uint32(x.TimeCurrentTime))
	x.Put32(pressEvent[8:], uint32(xc.root))
	x.Put32(pressEvent[12:], uint32(xc.root))
	x.Put16(pressEvent[20:], uint16(reply.RootX))
	x.Put16(pressEvent[22:], uint16(reply.RootY))
	x.Put16(pressEvent[24:], uint16(reply.RootX))
	x.Put16(pressEvent[26:], uint16(reply.RootY))
	x.Put16(pressEvent[28:], 0)
	pressEvent[30] = 1 // same_screen

	err = x.SendEventChecked(xc.conn, false, xc.root, 
		x.EventMaskButtonPress, pressEvent).Check(xc.conn)
	if err != nil {
		return err
	}

	// Send button release event
	releaseEvent := make([]byte, 32)
	releaseEvent[0] = x.ButtonReleaseEventCode
	releaseEvent[1] = button
	x.Put32(releaseEvent[4:], uint32(x.TimeCurrentTime))
	x.Put32(releaseEvent[8:], uint32(xc.root))
	x.Put32(releaseEvent[12:], uint32(xc.root))
	x.Put16(releaseEvent[20:], uint16(reply.RootX))
	x.Put16(releaseEvent[22:], uint16(reply.RootY))
	x.Put16(releaseEvent[24:], uint16(reply.RootX))
	x.Put16(releaseEvent[26:], uint16(reply.RootY))
	x.Put16(releaseEvent[28:], 0)
	releaseEvent[30] = 1 // same_screen

	return x.SendEventChecked(xc.conn, false, xc.root,
		x.EventMaskButtonRelease, releaseEvent).Check(xc.conn)
}

func (xc *X11Controller) TypeText(text string) error {
	for _, char := range text {
		keycode := xc.charToKeycode(char)
		if keycode == 0 {
			continue
		}
		
		// Send key press
		pressEvent := make([]byte, 32)
		pressEvent[0] = x.KeyPressEventCode
		pressEvent[1] = byte(keycode)
		x.Put32(pressEvent[4:], uint32(x.TimeCurrentTime))
		x.Put32(pressEvent[8:], uint32(xc.root))
		x.Put32(pressEvent[12:], uint32(xc.root))
		pressEvent[30] = 1 // same_screen

		err := x.SendEventChecked(xc.conn, false, xc.root,
			x.EventMaskKeyPress, pressEvent).Check(xc.conn)
		if err != nil {
			return err
		}

		// Send key release
		releaseEvent := make([]byte, 32)
		releaseEvent[0] = x.KeyReleaseEventCode
		releaseEvent[1] = byte(keycode)
		x.Put32(releaseEvent[4:], uint32(x.TimeCurrentTime))
		x.Put32(releaseEvent[8:], uint32(xc.root))
		x.Put32(releaseEvent[12:], uint32(xc.root))
		releaseEvent[30] = 1 // same_screen

		err = x.SendEventChecked(xc.conn, false, xc.root,
			x.EventMaskKeyRelease, releaseEvent).Check(xc.conn)
		if err != nil {
			return err
		}
	}
	return nil
}

func (xc *X11Controller) charToKeycode(char rune) x.Keycode {
	// Basic ASCII to keycode mapping
	switch char {
	case 'a', 'A':
		return 38
	case 'h', 'H':
		return 43
	case 'e', 'E':
		return 26
	case 'l', 'L':
		return 46
	case 'o', 'O':
		return 32
	case ' ':
		return 65
	case '\n':
		return 36
	default:
		return 0
	}
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
	flag.StringVar(&config.WindowManager, "wm", "i3", "Window manager to use (e.g., i3, openbox)")
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

	// Move mouse tool
	moveMouseTool := &mcp.Tool{
		Name:        "move_mouse",
		Description: "Move the mouse cursor to specific coordinates",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"x": {Type: "number", Description: "X coordinate"},
				"y": {Type: "number", Description: "Y coordinate"},
			},
			Required: []string{"x", "y"},
		},
	}
	
	server.AddTool(moveMouseTool, func(ctx context.Context, session *mcp.ServerSession, params *mcp.CallToolParamsFor[map[string]any]) (*mcp.CallToolResultFor[any], error) {
		args := params.Arguments
		
		x := int16(args["x"].(float64))
		y := int16(args["y"].(float64))
		
		if err := xc.MoveMouse(x, y); err != nil {
			return nil, err
		}
		
		return &mcp.CallToolResultFor[any]{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Moved mouse to (%d, %d)", x, y),
				},
			},
		}, nil
	})

	// Click tool
	clickTool := &mcp.Tool{
		Name:        "click",
		Description: "Click a mouse button (1=left, 2=middle, 3=right)",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"button": {
					Type:        "number",
					Description: "Button number (1=left, 2=middle, 3=right)",
					Default:     json.RawMessage("1"),
				},
			},
		},
	}
	
	server.AddTool(clickTool, func(ctx context.Context, session *mcp.ServerSession, params *mcp.CallToolParamsFor[map[string]any]) (*mcp.CallToolResultFor[any], error) {
		args := params.Arguments
		
		button := uint8(1)
		if b, ok := args["button"].(float64); ok {
			button = uint8(b)
		}
		
		if err := xc.Click(button); err != nil {
			return nil, err
		}
		
		return &mcp.CallToolResultFor[any]{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Clicked button %d", button),
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
		
		filename := "screenshot.png"
		if f, ok := args["filename"].(string); ok && f != "" {
			filename = f
		}
		
		// Use import command to take screenshot
		cmd := exec.Command("import", "-window", "root", filename)
		cmd.Env = append(os.Environ(), "DISPLAY="+config.Display)
		
		if err := cmd.Run(); err != nil {
			// Try alternative: xwd + convert
			xwdCmd := exec.Command("xwd", "-root", "-out", "/tmp/screenshot.xwd")
			xwdCmd.Env = append(os.Environ(), "DISPLAY="+config.Display)
			if err := xwdCmd.Run(); err != nil {
				return nil, fmt.Errorf("failed to take screenshot: %w", err)
			}
			
			convertCmd := exec.Command("convert", "/tmp/screenshot.xwd", filename)
			if err := convertCmd.Run(); err != nil {
				return nil, fmt.Errorf("failed to convert screenshot: %w", err)
			}
		}
		
		return &mcp.CallToolResultFor[any]{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Screenshot saved to: %s", filename),
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