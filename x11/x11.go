package x11

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/ext/test"
)

// Client represents an X11 connection
type Client struct {
	conn        *x.Conn
	screen      *x.Screen
	root        x.Window
	xvfbProcess *exec.Cmd // Track Xvfb if we started it
	display     string    // The display we're connected to
	i3Connected bool      // Whether i3 is available
}

// ScreenInfo contains display information
type ScreenInfo struct {
	Width  uint16
	Height uint16
	Root   x.Window
}

// ConnectOptions allows configuring the X11 connection
type ConnectOptions struct {
	Display      string // X11 display to use
	StartXvfb    bool   // Whether to start Xvfb if no display
	Resolution   string // Xvfb resolution (default: 1920x1080)
	StartWM      bool   // Whether to start a window manager
	WMName       string // Window manager command (default: "i3 -a")
}

// Connect establishes a connection to the X server with default options
func Connect() (*Client, error) {
	return ConnectWithOptions(ConnectOptions{
		StartXvfb:  true,
		Resolution: "1920x1080",
		StartWM:    true,
		WMName:     "i3 -a",
	})
}

// ConnectWithOptions establishes a connection to the X server with options
func ConnectWithOptions(opts ConnectOptions) (*Client, error) {
	client := &Client{}
	
	// Use provided display or environment variable
	display := opts.Display
	if display == "" {
		display = os.Getenv("DISPLAY")
	}
	
	// If no DISPLAY and StartXvfb is true, start Xvfb
	if display == "" && opts.StartXvfb {
		// Check if Xvfb is available
		if _, err := exec.LookPath("Xvfb"); err != nil {
			return nil, fmt.Errorf("no DISPLAY set and Xvfb not found")
		}
		
		// Find an available display number
		foundDisplay := false
		for i := 99; i < 200; i++ {
			testDisplay := fmt.Sprintf(":%d", i)
			lockFile := fmt.Sprintf("/tmp/.X%d-lock", i)
			
			// Check if display is in use
			if _, err := os.Stat(lockFile); os.IsNotExist(err) {
				// Try to start Xvfb on this display to check if it's really available
				testCmd := exec.Command("Xvfb", testDisplay, "-screen", "0", "320x240x8")
				if err := testCmd.Start(); err == nil {
					// Successfully started, this display is available
					testCmd.Process.Kill()
					testCmd.Wait()
					display = testDisplay
					foundDisplay = true
					break
				}
			}
		}
		
		if !foundDisplay {
			return nil, fmt.Errorf("could not find available display number")
		}
		
		// Start Xvfb
		resolution := opts.Resolution
		if resolution == "" {
			resolution = "1920x1080"
		}
		
		client.xvfbProcess = exec.Command("Xvfb", display, "-screen", "0", resolution+"x24", "-ac")
		client.xvfbProcess.Stdout = os.Stdout
		client.xvfbProcess.Stderr = os.Stderr
		
		if err := client.xvfbProcess.Start(); err != nil {
			return nil, fmt.Errorf("failed to start Xvfb: %w", err)
		}
		
		// Set DISPLAY for this process
		os.Setenv("DISPLAY", display)
		
		// Wait for Xvfb to start and be ready
		startTime := time.Now()
		for time.Since(startTime) < 5*time.Second {
			// Try to connect
			if testConn, err := x.NewConn(); err == nil {
				testConn.Close()
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
	} else if display == "" {
		return nil, fmt.Errorf("no DISPLAY specified")
	} else {
		// Use the provided display
		os.Setenv("DISPLAY", display)
	}

	// Connect to X server
	conn, err := x.NewConn()
	if err != nil {
		if client.xvfbProcess != nil {
			client.xvfbProcess.Process.Kill()
			client.xvfbProcess.Wait()
		}
		return nil, fmt.Errorf("failed to connect to X11: %w", err)
	}

	setup := conn.GetSetup()
	if len(setup.Roots) == 0 {
		return nil, fmt.Errorf("no screens found")
	}

	screen := &setup.Roots[0]

	// Initialize XTEST extension
	extReply, err := x.QueryExtension(conn, "XTEST").Reply(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to query XTEST extension: %w", err)
	}
	if !extReply.Present {
		conn.Close()
		return nil, fmt.Errorf("XTEST extension not present")
	}

	// Query XTEST version
	cookie := test.GetVersion(conn, test.MajorVersion, test.MinorVersion)
	_, err = cookie.Reply(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to get XTEST version: %w", err)
	}

	client.conn = conn
	client.screen = screen
	client.root = screen.Root
	client.display = display
	
	// Start window manager if requested
	if opts.StartWM && opts.WMName != "" {
		// Split the window manager command into program and args
		parts := strings.Fields(opts.WMName)
		if len(parts) > 0 {
			program := parts[0]
			args := parts[1:]
			if _, err := client.StartApp(program, args); err != nil {
				// Log warning but don't fail - window manager is optional
				fmt.Fprintf(os.Stderr, "Warning: failed to start window manager %s: %v\n", opts.WMName, err)
			}
			
			// If we started i3, wait a bit and try to connect
			if strings.Contains(program, "i3") {
				time.Sleep(500 * time.Millisecond)
				if err := client.ConnectI3(""); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to connect to i3: %v\n", err)
				}
			}
		}
	} else {
		// Try to connect to i3 if it's already running
		client.ConnectI3("")
	}
	
	return client, nil
}

// Close closes the X11 connection
func (c *Client) Close() error {
	if c.conn != nil {
		c.conn.Close()
	}
	
	// No need to close i3 connection as the library manages it internally
	
	// If we started Xvfb, stop it
	if c.xvfbProcess != nil {
		c.xvfbProcess.Process.Kill()
		c.xvfbProcess.Wait()
	}
	
	return nil
}

// GetScreenInfo returns information about the screen
func (c *Client) GetScreenInfo() (*ScreenInfo, error) {
	return &ScreenInfo{
		Width:  c.screen.WidthInPixels,
		Height: c.screen.HeightInPixels,
		Root:   c.root,
	}, nil
}

// GetDisplay returns the display string we're connected to
func (c *Client) GetDisplay() string {
	return c.display
}

// IsXvfbManaged returns true if we started Xvfb for this connection
func (c *Client) IsXvfbManaged() bool {
	return c.xvfbProcess != nil
}