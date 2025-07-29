package x11

import (
	"os"
	"testing"
)

// TestFirefoxIntegration tests the complete X11 functionality with Firefox
func TestFirefoxIntegration(t *testing.T) {
	// Skip if Firefox not available
	if _, err := os.Stat("/usr/bin/firefox"); os.IsNotExist(err) {
		t.Skip("Firefox not available")
	}
	
	// Clear DISPLAY to force new Xvfb
	origDisplay := os.Getenv("DISPLAY")
	os.Unsetenv("DISPLAY")
	defer os.Setenv("DISPLAY", origDisplay)
	
	// Connect with a larger resolution for Firefox
	client, err := ConnectWithOptions(ConnectOptions{
		StartXvfb:  true,
		Resolution: "1280x1024",
	})
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()
	
	// Get screen info
	info, err := client.GetScreenInfo()
	if err != nil {
		t.Fatalf("Failed to get screen info: %v", err)
	}
	t.Logf("Screen: %dx%d on display %s", info.Width, info.Height, client.GetDisplay())
	
	// Start Firefox with specific profile to avoid first-run dialogs
	pid, err := client.StartApp("firefox", []string{
		"--new-instance",
		"--width=1200",
		"--height=900",
		"https://example.com",
	})
	if err != nil {
		t.Fatalf("Failed to start Firefox: %v", err)
	}
	defer client.StopApp(pid)
	
	t.Logf("Started Firefox with PID %d", pid)
	
	// Wait for Firefox to start and load
	t.Log("Waiting for Firefox to start...")
	client.Wait(5000) // 5 seconds for Firefox to fully load
	
	// Take initial screenshot
	screenshot1, err := client.ScreenshotPNG()
	if err != nil {
		t.Errorf("Failed to take initial screenshot: %v", err)
	} else {
		t.Logf("Initial screenshot: %d bytes", len(screenshot1))
		if os.Getenv("SAVE_TEST_SCREENSHOTS") != "" {
			os.WriteFile("firefox_initial.png", screenshot1, 0644)
			t.Log("Saved initial screenshot to firefox_initial.png")
		}
	}
	
	// Click on the address bar (typically at the top)
	t.Log("Clicking on address bar...")
	err = client.MouseMove(640, 50)
	if err != nil {
		t.Errorf("Failed to move mouse to address bar: %v", err)
	}
	
	err = client.MouseClick(1)
	if err != nil {
		t.Errorf("Failed to click address bar: %v", err)
	}
	
	// Select all text and clear
	t.Log("Clearing address bar...")
	err = client.KeyCombo("ctrl+a")
	if err != nil {
		t.Errorf("Failed to select all: %v", err)
	}
	
	// Type new URL
	t.Log("Typing example.com...")
	err = client.Type("example.com")
	if err != nil {
		t.Errorf("Failed to type URL: %v", err)
	}
	
	// Press Enter to navigate
	t.Log("Pressing Enter to navigate...")
	err = client.KeyPress("Return")
	if err != nil {
		t.Errorf("Failed to press Return: %v", err)
	}
	
	// Wait for page to load
	t.Log("Waiting for page to load...")
	client.Wait(3000)
	
	// Take final screenshot
	screenshot2, err := client.ScreenshotPNG()
	if err != nil {
		t.Errorf("Failed to take final screenshot: %v", err)
	} else {
		t.Logf("Final screenshot: %d bytes", len(screenshot2))
		if os.Getenv("SAVE_TEST_SCREENSHOTS") != "" {
			os.WriteFile("firefox_example_com.png", screenshot2, 0644)
			t.Log("Saved final screenshot to firefox_example_com.png")
		}
	}
	
	// Test scrolling
	t.Log("Testing scrolling...")
	err = client.KeyPress("Page_Down")
	if err != nil {
		t.Errorf("Failed to scroll down: %v", err)
	}
	
	client.Wait(500)
	
	err = client.KeyPress("Page_Up")
	if err != nil {
		t.Errorf("Failed to scroll up: %v", err)
	}
	
	// Test keyboard shortcuts
	t.Log("Testing keyboard shortcuts...")
	
	// Zoom in
	err = client.KeyCombo("ctrl+plus")
	if err != nil {
		// Try alternative
		client.KeyCombo("ctrl+=")
	}
	client.Wait(500)
	
	// Zoom out
	err = client.KeyCombo("ctrl+minus")
	if err != nil {
		client.KeyCombo("ctrl+-")
	}
	client.Wait(500)
	
	// Reset zoom
	err = client.KeyCombo("ctrl+0")
	if err != nil {
		t.Errorf("Failed to reset zoom: %v", err)
	}
	
	t.Log("Firefox integration test completed successfully")
}