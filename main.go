package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"mcp-x11-controller/x11"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var client *x11.Client

// Tool input types
type GetScreenInfoInput struct{}

type TakeScreenshotInput struct{}

type ClickAtInput struct {
	X      float64 `json:"x" jsonschema:"required"`
	Y      float64 `json:"y" jsonschema:"required"`
	Button int     `json:"button,omitempty"`
	Delay  int     `json:"delay,omitempty"`
}

type TypeTextInput struct {
	Text  string `json:"text" jsonschema:"required"`
	Delay int    `json:"delay,omitempty"`
}

type StartProgramInput struct {
	Program string   `json:"program" jsonschema:"required"`
	Args    []string `json:"args,omitempty"`
	Delay   int      `json:"delay,omitempty"`
}
type KeyPressInput struct {
	Key   string `json:"key,omitempty" jsonschema:"description,Special key name like Enter Tab Escape"`
	Combo string `json:"combo,omitempty" jsonschema:"description,Key combination like ctrl+c alt+tab"`
	Delay int    `json:"delay,omitempty"`
}

type I3GetTreeInput struct{}

type I3CmdInput struct {
	Command string `json:"command" jsonschema:"required"`
}

func main() {
	// Parse command line flags
	var (
		noWM    = flag.Bool("no-wm", false, "Disable window manager startup")
		wmName  = flag.String("wm-name", "i3 -a", "Window manager to start")
		help    = flag.Bool("help", false, "Show help message")
		version = flag.Bool("version", false, "Show version")
	)
	flag.Parse()
	
	// Show help
	if *help {
		fmt.Println("MCP X11 Controller")
		fmt.Println("\nUsage: mcp-x11-controller [options]")
		fmt.Println("\nOptions:")
		flag.PrintDefaults()
		fmt.Println("\nEnvironment variables:")
		fmt.Println("  DISPLAY        X11 display to connect to (if not set, Xvfb will be started)")
		os.Exit(0)
	}
	
	// Show version
	if *version {
		fmt.Println("mcp-x11-controller v0.3.0")
		os.Exit(0)
	}
	
	// Log startup to stderr
	log.SetOutput(os.Stderr)
	log.Println("Starting MCP X11 Controller...")
	if os.Getenv("DISPLAY") != "" {
		log.Printf("Using existing DISPLAY: %s", os.Getenv("DISPLAY"))
	} else {
		log.Println("No DISPLAY set, will start Xvfb")
	}
	
	// Connect to X11 with options
	opts := x11.ConnectOptions{
		StartXvfb:  os.Getenv("DISPLAY") == "",
		Resolution: "1920x1080",
		StartWM:    !*noWM,
		WMName:     *wmName,
	}
	
	var err error
	client, err = x11.ConnectWithOptions(opts)
	if err != nil {
		log.Fatalf("Failed to connect to X11: %v", err)
	}
	defer client.Close()
	
	// Create MCP server
	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    "x11-controller",
			Version: "0.3.0",
			Title:   "X11 Controller MCP Server",
		},
		&mcp.ServerOptions{
			Instructions: `Control X11 desktop applications through MCP

## Window Management with i3

When i3 window manager is running, use these commands:

1. **i3_get_tree** - Get the window tree to find windows
   - Returns JSON tree structure with window IDs, titles, classes
   - Look for nodes with "window_properties" to find actual windows

2. **i3_cmd** - Control windows with i3 commands
   - Focus window: [con_id=WINDOW_ID] focus
   - Switch workspace: workspace NUMBER
   - Move window: [con_id=WINDOW_ID] move to workspace NUMBER
   - Focus by class: [class="CLASS_NAME"] focus
   - Multiple commands: command1; command2

Example workflow:
1. Use i3_get_tree to find window IDs
2. Use i3_cmd with [con_id=ID] focus to switch to that window`,
		},
	)
	
	// Add tools to the server
	
	// x11_get_screen_info tool
	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "x11_get_screen_info",
			Title:       "X11 Get Screen Info",
			Description: "Get X11 screen information including dimensions and screenshot",
		},
		func(ctx context.Context, session *mcp.ServerSession, params *mcp.CallToolParamsFor[GetScreenInfoInput]) (*mcp.CallToolResultFor[any], error) {
			info, err := client.GetScreenInfo()
			if err != nil {
				return nil, err
			}
			
			// Take screenshot
			pngData, err := client.ScreenshotPNG()
			if err != nil {
				return nil, fmt.Errorf("failed to take screenshot: %w", err)
			}
			
			content := []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Screen: %dx%d", info.Width, info.Height),
				},
				&mcp.ImageContent{
					Data:     pngData,
					MIMEType: "image/png",
				},
			}
			
			return &mcp.CallToolResultFor[any]{
				Content: content,
				Meta: map[string]any{
					"width":  info.Width,
					"height": info.Height,
				},
			}, nil
		},
	)
	
	// x11_take_screenshot tool
	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "x11_take_screenshot",
			Title:       "X11 Take Screenshot",
			Description: "Take a screenshot of the X11 display",
		},
		func(ctx context.Context, session *mcp.ServerSession, params *mcp.CallToolParamsFor[TakeScreenshotInput]) (*mcp.CallToolResultFor[any], error) {
			pngData, err := client.ScreenshotPNG()
			if err != nil {
				return nil, err
			}
			
			content := []mcp.Content{
				&mcp.ImageContent{
					Data:     pngData,
					MIMEType: "image/png",
				},
			}
			
			return &mcp.CallToolResultFor[any]{
				Content: content,
			}, nil
		},
	)
	
	// x11_click_at tool
	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "x11_click_at",
			Title:       "X11 Click At",
			Description: "Move mouse to coordinates and click, returns screenshot after delay",
		},
		func(ctx context.Context, session *mcp.ServerSession, params *mcp.CallToolParamsFor[ClickAtInput]) (*mcp.CallToolResultFor[any], error) {
			button := params.Arguments.Button
			if button == 0 {
				button = 1
			}
			
			delay := params.Arguments.Delay
			if delay == 0 {
				delay = 100 // Default 100ms delay
			}
			
			// Move and click
			if err := client.MouseMove(int(params.Arguments.X), int(params.Arguments.Y)); err != nil {
				return nil, err
			}
			if err := client.MouseClick(button); err != nil {
				return nil, err
			}
			
			// Wait for the specified delay
			time.Sleep(time.Duration(delay) * time.Millisecond)
			
			// Take screenshot
			pngData, err := client.ScreenshotPNG()
			if err != nil {
				return nil, fmt.Errorf("failed to take screenshot: %w", err)
			}
			
			content := []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Clicked at (%d, %d) with button %d", int(params.Arguments.X), int(params.Arguments.Y), button),
				},
				&mcp.ImageContent{
					Data:     pngData,
					MIMEType: "image/png",
				},
			}
			
			return &mcp.CallToolResultFor[any]{
				Content: content,
			}, nil
		},
	)
	
	// x11_type_text tool
	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "x11_type_text",
			Title:       "X11 Type Text",
			Description: "Type text by sending key events, returns screenshot after delay",
		},
		func(ctx context.Context, session *mcp.ServerSession, params *mcp.CallToolParamsFor[TypeTextInput]) (*mcp.CallToolResultFor[any], error) {
			if err := client.Type(params.Arguments.Text); err != nil {
				return nil, err
			}
			
			delay := params.Arguments.Delay
			if delay == 0 {
				delay = 100 // Default 100ms delay
			}
			
			// Wait for the specified delay
			time.Sleep(time.Duration(delay) * time.Millisecond)
			
			// Take screenshot
			pngData, err := client.ScreenshotPNG()
			if err != nil {
				return nil, fmt.Errorf("failed to take screenshot: %w", err)
			}
			
			content := []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Typed: %s", params.Arguments.Text),
				},
				&mcp.ImageContent{
					Data:     pngData,
					MIMEType: "image/png",
				},
			}
			
			return &mcp.CallToolResultFor[any]{
				Content: content,
			}, nil
		},
	)
	
	// x11_start_program tool
	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "x11_start_program",
			Title:       "X11 Start Program",
			Description: "Start a desktop program in the background, returns screenshot after delay",
		},
		func(ctx context.Context, session *mcp.ServerSession, params *mcp.CallToolParamsFor[StartProgramInput]) (*mcp.CallToolResultFor[any], error) {
			pid, err := client.StartApp(params.Arguments.Program, params.Arguments.Args)
			if err != nil {
				return nil, err
			}
			
			delay := params.Arguments.Delay
			if delay == 0 {
				delay = 100 // Default 100ms delay
			}
			
			// Wait for the specified delay
			time.Sleep(time.Duration(delay) * time.Millisecond)
			
			// Take screenshot
			pngData, err := client.ScreenshotPNG()
			if err != nil {
				return nil, fmt.Errorf("failed to take screenshot: %w", err)
			}
			
			content := []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Started %s with PID %d", params.Arguments.Program, pid),
				},
				&mcp.ImageContent{
					Data:     pngData,
					MIMEType: "image/png",
				},
			}
			
			return &mcp.CallToolResultFor[any]{
				Content: content,
				Meta: map[string]any{
					"pid": pid,
				},
			}, nil
		},
	)
	
	// x11_key_press tool
	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "x11_key_press",
			Title:       "X11 Key Press",
			Description: "Press special keys or key combinations, returns screenshot after delay",
		},
		func(ctx context.Context, session *mcp.ServerSession, params *mcp.CallToolParamsFor[KeyPressInput]) (*mcp.CallToolResultFor[any], error) {
			// Handle either single key or key combo
			if params.Arguments.Combo != "" {
				if err := client.KeyCombo(params.Arguments.Combo); err != nil {
					return nil, err
				}
			} else if params.Arguments.Key != "" {
				if err := client.KeyPress(params.Arguments.Key); err != nil {
					return nil, err
				}
			} else {
				return nil, fmt.Errorf("either 'key' or 'combo' must be specified")
			}
			
			delay := params.Arguments.Delay
			if delay == 0 {
				delay = 100 // Default 100ms delay
			}
			
			// Wait for the specified delay
			time.Sleep(time.Duration(delay) * time.Millisecond)
			
			// Take screenshot
			pngData, err := client.ScreenshotPNG()
			if err != nil {
				return nil, fmt.Errorf("failed to take screenshot: %w", err)
			}
			
			content := []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Pressed: %s%s", params.Arguments.Key, params.Arguments.Combo),
				},
				&mcp.ImageContent{
					Data:     pngData,
					MIMEType: "image/png",
				},
			}
			
			return &mcp.CallToolResultFor[any]{
				Content: content,
			}, nil
		},
	)
	
	// i3_get_tree tool (only available when i3 is connected)
	if client.I3Enabled() {
		mcp.AddTool(server,
			&mcp.Tool{
				Name:        "i3_get_tree",
				Title:       "i3 Get Tree",
				Description: "Get the i3 window tree as JSON. Use this to find window IDs and container structure for window management.",
			},
			func(ctx context.Context, session *mcp.ServerSession, params *mcp.CallToolParamsFor[I3GetTreeInput]) (*mcp.CallToolResultFor[any], error) {
				treeJSON, err := client.I3GetTree()
				if err != nil {
					return nil, err
				}
				
				content := []mcp.Content{
					&mcp.TextContent{
						Text: treeJSON,
					},
				}
				
				return &mcp.CallToolResultFor[any]{
					Content: content,
				}, nil
			},
		)
		
		// i3_cmd tool
		mcp.AddTool(server,
			&mcp.Tool{
				Name:        "i3_cmd",
				Title:       "i3 Command",
				Description: "Send a command to i3 window manager. Examples: '[con_id=1234] focus' to focus a window, 'workspace 2' to switch workspace, '[class=\"Firefox\"] move to workspace 3' to move windows.",
			},
			func(ctx context.Context, session *mcp.ServerSession, params *mcp.CallToolParamsFor[I3CmdInput]) (*mcp.CallToolResultFor[any], error) {
				result, err := client.I3Command(params.Arguments.Command)
				if err != nil {
					return nil, err
				}
				
				// Take screenshot to show result
				pngData, err := client.ScreenshotPNG()
				if err != nil {
					return nil, fmt.Errorf("failed to take screenshot: %w", err)
				}
				
				content := []mcp.Content{
					&mcp.TextContent{
						Text: fmt.Sprintf("i3 command result: %s", result),
					},
					&mcp.ImageContent{
						Data:     pngData,
						MIMEType: "image/png",
					},
				}
				
				return &mcp.CallToolResultFor[any]{
					Content: content,
				}, nil
			},
		)
	}
	
	// Run the server
	transport := mcp.NewStdioTransport()
	if err := server.Run(context.Background(), transport); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}