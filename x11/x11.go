package x11

import (
	"fmt"
	"os"
	"os/exec"
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
	Resolution   string // Xvfb resolution (default: 1024x768)
}

// Connect establishes a connection to the X server with default options
func Connect() (*Client, error) {
	return ConnectWithOptions(ConnectOptions{
		StartXvfb:  true,
		Resolution: "1024x768",
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
				// Also check if we can bind to the socket
				socketPath := fmt.Sprintf("/tmp/.X11-unix/X%d", i)
				if _, err := os.Stat(socketPath); os.IsNotExist(err) {
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
			resolution = "1024x768"
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
	
	return client, nil
}

// Close closes the X11 connection
func (c *Client) Close() error {
	if c.conn != nil {
		c.conn.Close()
	}
	
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