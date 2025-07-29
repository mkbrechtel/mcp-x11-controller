package x11

import (
	"os"
	"testing"
	"time"
)

// TestStartApp tests basic application launching
func TestStartApp(t *testing.T) {
	// Clear DISPLAY to force new Xvfb
	origDisplay := os.Getenv("DISPLAY")
	os.Unsetenv("DISPLAY")
	defer os.Setenv("DISPLAY", origDisplay)
	
	client, err := Connect()
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()
	
	// Test starting xterm
	pid, err := client.StartApp("xterm", []string{"-geometry", "80x24"})
	if err != nil {
		// If xterm not available, skip
		if os.IsNotExist(err) {
			t.Skip("xterm not available")
		}
		t.Fatalf("Failed to start xterm: %v", err)
	}
	
	t.Logf("Started xterm with PID %d", pid)
	
	// Give it time to start
	time.Sleep(500 * time.Millisecond)
	
	// Take a screenshot to verify it's running
	img, err := client.Screenshot()
	if err != nil {
		t.Errorf("Failed to take screenshot: %v", err)
	} else {
		bounds := img.Bounds()
		t.Logf("Screenshot after xterm launch: %dx%d", bounds.Dx(), bounds.Dy())
	}
	
	// Kill the app
	err = client.StopApp(pid)
	if err != nil {
		t.Errorf("Failed to stop xterm: %v", err)
	}
}

// TestStartMultipleApps tests launching multiple applications
func TestStartMultipleApps(t *testing.T) {
	// Clear DISPLAY to force new Xvfb
	origDisplay := os.Getenv("DISPLAY")
	os.Unsetenv("DISPLAY")
	defer os.Setenv("DISPLAY", origDisplay)
	
	client, err := Connect()
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()
	
	// Try to start multiple xclock instances
	var pids []int
	
	for i := 0; i < 3; i++ {
		pid, err := client.StartApp("xclock", nil)
		if err != nil {
			if os.IsNotExist(err) {
				t.Skip("xclock not available")
			}
			t.Fatalf("Failed to start xclock #%d: %v", i+1, err)
		}
		pids = append(pids, pid)
		t.Logf("Started xclock #%d with PID %d", i+1, pid)
		time.Sleep(100 * time.Millisecond)
	}
	
	// Clean up all apps
	for i, pid := range pids {
		err = client.StopApp(pid)
		if err != nil {
			t.Errorf("Failed to stop xclock #%d (PID %d): %v", i+1, pid, err)
		}
	}
}

// TestStartAppWithEnv tests launching app with environment variables
func TestStartAppWithEnv(t *testing.T) {
	// Clear DISPLAY to force new Xvfb
	origDisplay := os.Getenv("DISPLAY")
	os.Unsetenv("DISPLAY")
	defer os.Setenv("DISPLAY", origDisplay)
	
	client, err := Connect()
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()
	
	// Test with environment variables
	env := map[string]string{
		"TEST_VAR": "test_value",
	}
	
	pid, err := client.StartAppWithEnv("xterm", []string{"-e", "env"}, env)
	if err != nil {
		if os.IsNotExist(err) {
			t.Skip("xterm not available")
		}
		t.Fatalf("Failed to start xterm with env: %v", err)
	}
	
	t.Logf("Started xterm with env, PID %d", pid)
	time.Sleep(500 * time.Millisecond)
	
	// Clean up
	client.StopApp(pid)
}