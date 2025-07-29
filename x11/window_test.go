package x11

import (
	"testing"
	"time"
	
	x "github.com/linuxdeepin/go-x11-client"
)

func TestListWindows(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	// Connect to X11
	client, err := Connect()
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	// Start a couple of test applications
	pid1, err := client.StartApp("xterm", []string{"-title", "Test Window 1"})
	if err != nil {
		t.Skipf("xterm not available: %v", err)
	}
	defer client.StopApp(pid1)

	pid2, err := client.StartApp("xclock", []string{"-title", "Test Clock"})
	if err != nil {
		t.Skipf("xclock not available: %v", err)
	}
	defer client.StopApp(pid2)

	// Wait for windows to appear
	time.Sleep(500 * time.Millisecond)

	// List windows
	windows, err := client.ListWindows()
	if err != nil {
		t.Fatalf("Failed to list windows: %v", err)
	}

	// Check if we have at least 2 windows
	if len(windows) < 2 {
		t.Errorf("Expected at least 2 windows, got %d", len(windows))
	}

	// Check if our windows are in the list
	foundXterm := false
	foundXclock := false
	for _, win := range windows {
		t.Logf("Window: ID=%d, Title=%q, Class=%q", win.ID, win.Title, win.Class)
		if win.Title == "Test Window 1" || win.Class == "XTerm" {
			foundXterm = true
		}
		if win.Title == "Test Clock" || win.Class == "XClock" {
			foundXclock = true
		}
	}

	if !foundXterm {
		t.Error("xterm window not found in window list")
	}
	if !foundXclock {
		t.Error("xclock window not found in window list")
	}
}

func TestFocusWindow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	// Connect to X11
	client, err := Connect()
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	// Start two test applications
	pid1, err := client.StartApp("xterm", []string{"-title", "Focus Test 1"})
	if err != nil {
		t.Skipf("xterm not available: %v", err)
	}
	defer client.StopApp(pid1)

	pid2, err := client.StartApp("xterm", []string{"-title", "Focus Test 2"})
	if err != nil {
		t.Skipf("xterm not available: %v", err)
	}
	defer client.StopApp(pid2)

	// Wait for windows to appear
	time.Sleep(500 * time.Millisecond)

	// List windows to find our test windows
	windows, err := client.ListWindows()
	if err != nil {
		t.Fatalf("Failed to list windows: %v", err)
	}

	var window1ID, window2ID uint32
	for _, win := range windows {
		if win.Title == "Focus Test 1" {
			window1ID = uint32(win.ID)
		} else if win.Title == "Focus Test 2" {
			window2ID = uint32(win.ID)
		}
	}

	if window1ID == 0 || window2ID == 0 {
		t.Fatal("Could not find test windows")
	}

	// Focus window 1
	err = client.FocusWindow(x.Window(window1ID))
	if err != nil {
		t.Errorf("Failed to focus window 1: %v", err)
	}

	// Wait a bit
	time.Sleep(200 * time.Millisecond)

	// Focus window 2
	err = client.FocusWindow(x.Window(window2ID))
	if err != nil {
		t.Errorf("Failed to focus window 2: %v", err)
	}

	t.Log("Window focus test completed")
}

func TestWindowManagerStartup(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	// Test with window manager disabled
	opts := ConnectOptions{
		StartXvfb:  true,
		Resolution: "800x600",
		StartWM:    false,
	}

	client, err := ConnectWithOptions(opts)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	// List windows - should be minimal
	windows, err := client.ListWindows()
	if err != nil {
		t.Fatalf("Failed to list windows: %v", err)
	}

	t.Logf("Windows without WM: %d", len(windows))

	// Now test with a simple window manager (if available)
	client2, err := ConnectWithOptions(ConnectOptions{
		StartXvfb:  true,
		Resolution: "800x600",
		StartWM:    true,
		WMName:     "twm", // Try twm as it's more commonly available than i3
	})
	if err != nil {
		t.Fatalf("Failed to connect with WM: %v", err)
	}
	defer client2.Close()

	// Give WM time to start
	time.Sleep(500 * time.Millisecond)

	// List windows again
	windows2, err := client2.ListWindows()
	if err != nil {
		t.Fatalf("Failed to list windows with WM: %v", err)
	}

	t.Logf("Windows with WM: %d", len(windows2))
}