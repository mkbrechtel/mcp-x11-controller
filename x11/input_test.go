package x11

import (
	"os"
	"testing"
	"time"
)

// TestMouseMove tests mouse movement
func TestMouseMove(t *testing.T) {
	// Clear DISPLAY to force new Xvfb
	origDisplay := os.Getenv("DISPLAY")
	os.Unsetenv("DISPLAY")
	defer os.Setenv("DISPLAY", origDisplay)
	
	client, err := Connect()
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()
	
	// Test moving to various positions
	positions := []struct {
		x, y int
	}{
		{100, 100},
		{500, 400},
		{0, 0},
		{1023, 767}, // Max coordinates for 1024x768
	}
	
	for _, pos := range positions {
		err := client.MouseMove(pos.x, pos.y)
		if err != nil {
			t.Errorf("Failed to move mouse to (%d, %d): %v", pos.x, pos.y, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	
	t.Log("Mouse movement tests completed")
}

// TestMouseClick tests mouse clicking
func TestMouseClick(t *testing.T) {
	// Clear DISPLAY to force new Xvfb
	origDisplay := os.Getenv("DISPLAY")
	os.Unsetenv("DISPLAY")
	defer os.Setenv("DISPLAY", origDisplay)
	
	client, err := Connect()
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()
	
	// Test different buttons
	buttons := []struct {
		button int
		name   string
	}{
		{1, "left"},
		{2, "middle"},
		{3, "right"},
	}
	
	// Move to center first
	client.MouseMove(512, 384)
	
	for _, b := range buttons {
		err := client.MouseClick(b.button)
		if err != nil {
			t.Errorf("Failed to click %s button: %v", b.name, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	
	t.Log("Mouse click tests completed")
}

// TestType tests typing text
func TestType(t *testing.T) {
	// Clear DISPLAY to force new Xvfb
	origDisplay := os.Getenv("DISPLAY")
	os.Unsetenv("DISPLAY")
	defer os.Setenv("DISPLAY", origDisplay)
	
	client, err := Connect()
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()
	
	// Test various text inputs
	texts := []string{
		"hello world",
		"HELLO WORLD",
		"123456789",
		"special!@#$%^&*()",
		"mixed Case 123!",
	}
	
	for _, text := range texts {
		err := client.Type(text)
		if err != nil {
			t.Errorf("Failed to type '%s': %v", text, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
	
	t.Log("Text typing tests completed")
}

// TestKeyPress tests individual key presses
func TestKeyPress(t *testing.T) {
	// Clear DISPLAY to force new Xvfb
	origDisplay := os.Getenv("DISPLAY")
	os.Unsetenv("DISPLAY")
	defer os.Setenv("DISPLAY", origDisplay)
	
	client, err := Connect()
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()
	
	// Test special keys
	keys := []string{
		"Return",
		"Tab",
		"Escape",
		"BackSpace",
		"Delete",
		"Home",
		"End",
		"Page_Up",
		"Page_Down",
		"Left",
		"Right",
		"Up",
		"Down",
	}
	
	for _, key := range keys {
		err := client.KeyPress(key)
		if err != nil {
			t.Errorf("Failed to press key '%s': %v", key, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	
	t.Log("Key press tests completed")
}

// TestKeyCombo tests key combinations
func TestKeyCombo(t *testing.T) {
	// Clear DISPLAY to force new Xvfb
	origDisplay := os.Getenv("DISPLAY")
	os.Unsetenv("DISPLAY")
	defer os.Setenv("DISPLAY", origDisplay)
	
	client, err := Connect()
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()
	
	// Test common key combinations
	combos := []string{
		"ctrl+a",
		"ctrl+c",
		"ctrl+v",
		"ctrl+z",
		"ctrl+shift+z",
		"alt+tab",
		"ctrl+alt+delete",
		"super+l",
	}
	
	for _, combo := range combos {
		err := client.KeyCombo(combo)
		if err != nil {
			t.Errorf("Failed to execute key combo '%s': %v", combo, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
	
	t.Log("Key combo tests completed")
}

// TestInputWithXterm tests input in a real application
func TestInputWithXterm(t *testing.T) {
	// Skip if xterm not available
	if _, err := os.Stat("/usr/bin/xterm"); os.IsNotExist(err) {
		t.Skip("xterm not available")
	}
	
	// Clear DISPLAY to force new Xvfb
	origDisplay := os.Getenv("DISPLAY")
	os.Unsetenv("DISPLAY")
	defer os.Setenv("DISPLAY", origDisplay)
	
	client, err := Connect()
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()
	
	// Start xterm
	pid, err := client.StartApp("xterm", []string{"-geometry", "80x24+100+100"})
	if err != nil {
		t.Fatalf("Failed to start xterm: %v", err)
	}
	defer client.StopApp(pid)
	
	// Wait for xterm to appear
	client.Wait(500)
	
	// Click in the xterm window to focus it
	err = client.MouseMove(300, 300)
	if err != nil {
		t.Errorf("Failed to move mouse: %v", err)
	}
	
	err = client.MouseClick(1)
	if err != nil {
		t.Errorf("Failed to click: %v", err)
	}
	
	// Type some text
	err = client.Type("echo 'Hello from X11!'")
	if err != nil {
		t.Errorf("Failed to type: %v", err)
	}
	
	// Press Enter
	err = client.KeyPress("Return")
	if err != nil {
		t.Errorf("Failed to press Return: %v", err)
	}
	
	// Wait a bit
	client.Wait(100)
	
	// Take a screenshot
	img, err := client.Screenshot()
	if err != nil {
		t.Errorf("Failed to take screenshot: %v", err)
	} else {
		bounds := img.Bounds()
		t.Logf("Screenshot after typing: %dx%d", bounds.Dx(), bounds.Dy())
	}
	
	// Test a key combo (Ctrl+C)
	err = client.KeyCombo("ctrl+c")
	if err != nil {
		t.Errorf("Failed to press Ctrl+C: %v", err)
	}
	
	t.Log("Input test with xterm completed")
}