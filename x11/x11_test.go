package x11

import (
	"os"
	"testing"
)

// TestConnect tests basic X11 connection
func TestConnect(t *testing.T) {
	// Save original DISPLAY
	origDisplay := os.Getenv("DISPLAY")
	defer os.Setenv("DISPLAY", origDisplay)
	
	// Clear DISPLAY to force Xvfb
	os.Unsetenv("DISPLAY")
	
	client, err := Connect()
	if err != nil {
		t.Fatalf("Failed to connect to X11: %v", err)
	}
	defer client.Close()

	// Verify connection is alive
	if client.conn == nil {
		t.Error("Connection is nil")
	}
	
	// Verify Xvfb was started
	if !client.IsXvfbManaged() {
		t.Error("Expected Xvfb to be managed")
	}
	
	// Verify we have a display
	display := client.GetDisplay()
	if display == "" {
		t.Error("No display set")
	}
	
	t.Logf("Successfully connected to X11 on display %s", display)
}

// TestConnectWithExistingDisplay tests connection to existing display
func TestConnectWithExistingDisplay(t *testing.T) {
	// Use the display from previous test or :99
	display := os.Getenv("DISPLAY")
	if display == "" {
		display = ":99"
	}
	
	client, err := ConnectWithOptions(ConnectOptions{
		Display:   display,
		StartXvfb: false,
	})
	if err != nil {
		t.Skipf("No existing display %s available: %v", display, err)
	}
	defer client.Close()
	
	// Verify Xvfb was NOT started
	if client.IsXvfbManaged() {
		t.Error("Expected Xvfb NOT to be managed")
	}
	
	t.Logf("Connected to existing display %s", client.GetDisplay())
}

// TestGetScreenInfo tests getting screen information
func TestGetScreenInfo(t *testing.T) {
	// Clear DISPLAY to force new Xvfb
	origDisplay := os.Getenv("DISPLAY")
	os.Unsetenv("DISPLAY")
	defer os.Setenv("DISPLAY", origDisplay)
	
	client, err := Connect()
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	info, err := client.GetScreenInfo()
	if err != nil {
		t.Fatalf("Failed to get screen info: %v", err)
	}

	// Basic validation
	if info.Width == 0 || info.Height == 0 {
		t.Errorf("Invalid screen dimensions: %dx%d", info.Width, info.Height)
	}

	t.Logf("Screen info: %dx%d on display %s", info.Width, info.Height, client.GetDisplay())
}

// TestMultipleConnections tests that we can handle multiple connections
func TestMultipleConnections(t *testing.T) {
	// Clear DISPLAY
	origDisplay := os.Getenv("DISPLAY")
	os.Unsetenv("DISPLAY")
	defer os.Setenv("DISPLAY", origDisplay)
	
	// Create first connection
	client1, err := Connect()
	if err != nil {
		t.Fatalf("Failed to create first connection: %v", err)
	}
	defer client1.Close()
	
	display1 := client1.GetDisplay()
	t.Logf("First connection on display %s", display1)
	
	// Clear DISPLAY again before second connection
	os.Unsetenv("DISPLAY")
	
	// Create second connection - should get a different display
	client2, err := Connect()
	if err != nil {
		t.Fatalf("Failed to create second connection: %v", err)
	}
	defer client2.Close()
	
	display2 := client2.GetDisplay()
	t.Logf("Second connection on display %s", display2)
	
	// Verify different displays
	if display1 == display2 {
		t.Errorf("Expected different displays, got %s for both", display1)
	}
}