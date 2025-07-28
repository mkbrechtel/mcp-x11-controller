package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
	"time"

	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/util/keysyms"
)

// Test configuration parsing
func TestConfig(t *testing.T) {
	config := &Config{
		UseXvfb:       true,
		Display:       ":99",
		Resolution:    "1920x1080",
		WindowManager: "openbox",
		Program:       "firefox",
	}

	if config.Display != ":99" {
		t.Errorf("Expected display :99, got %s", config.Display)
	}

	if config.Resolution != "1920x1080" {
		t.Errorf("Expected resolution 1920x1080, got %s", config.Resolution)
	}
}

// Test X11Controller creation with real connection (only if DISPLAY is set)
func TestNewX11ControllerReal(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("No DISPLAY set, skipping real X11 test")
	}

	xc, err := NewX11Controller()
	if err != nil {
		t.Fatalf("Failed to create X11Controller: %v", err)
	}
	defer xc.Close()

	if xc.conn == nil {
		t.Error("X11 connection is nil")
	}

	if xc.screen == nil {
		t.Error("Screen is nil")
	}

	if xc.keySymbols == nil {
		t.Error("KeySymbols is nil")
	}

	if !xc.connAlive {
		t.Error("Connection should be alive after creation")
	}
}

// TestGetScreenInfo tests getting screen information
func TestGetScreenInfo(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("No DISPLAY set, skipping real X11 test")
	}

	xc, err := NewX11Controller()
	if err != nil {
		t.Fatalf("Failed to create X11Controller: %v", err)
	}
	defer xc.Close()

	info, err := xc.GetScreenInfo()
	if err != nil {
		t.Fatalf("Failed to get screen info: %v", err)
	}

	if _, ok := info["width"]; !ok {
		t.Error("Screen info missing width")
	}

	if _, ok := info["height"]; !ok {
		t.Error("Screen info missing height")
	}

	if _, ok := info["root"]; !ok {
		t.Error("Screen info missing root window")
	}
}

// Test image processing functionality  
func TestImageProcessing(t *testing.T) {
	// Create a test image
	width, height := 100, 100
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	
	// Fill with test pattern
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8(x * 255 / width),
				G: uint8(y * 255 / height),
				B: 128,
				A: 255,
			})
		}
	}

	// Encode to PNG
	var buf bytes.Buffer
	err := png.Encode(&buf, img)
	if err != nil {
		t.Fatalf("Failed to encode PNG: %v", err)
	}

	// Decode back
	decoded, err := png.Decode(&buf)
	if err != nil {
		t.Fatalf("Failed to decode PNG: %v", err)
	}

	// Verify dimensions
	bounds := decoded.Bounds()
	if bounds.Dx() != width || bounds.Dy() != height {
		t.Errorf("Image dimensions mismatch: got %dx%d, want %dx%d", 
			bounds.Dx(), bounds.Dy(), width, height)
	}
}

// Test keysym functionality
func TestKeysymConversion(t *testing.T) {
	tests := []struct {
		char     rune
		needShift bool
	}{
		{'a', false},
		{'A', true},
		{'1', false},
		{'!', true},
		{' ', false},
		{'@', true},
	}

	for _, test := range tests {
		keysym := x.Keysym(test.char)
		lower, upper := keysyms.ConvertCase(keysym)
		
		if test.char >= 'A' && test.char <= 'Z' {
			if lower == upper {
				t.Errorf("Expected different lower/upper for %c", test.char)
			}
		}
	}
}

// TestConnectionMonitoring tests connection monitoring
func TestConnectionMonitoring(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("No DISPLAY set, skipping real X11 test")
	}

	xc, err := NewX11Controller()
	if err != nil {
		t.Fatalf("Failed to create X11Controller: %v", err)
	}

	// Test that connection is alive
	if !xc.connAlive {
		t.Error("Connection should be alive")
	}

	// Close connection
	xc.Close()

	// Test that connection is marked as dead
	if xc.connAlive {
		t.Error("Connection should not be alive after closing")
	}

	// Test that operations fail after closing
	err = xc.checkConnection()
	if err == nil {
		t.Error("Expected error when checking closed connection")
	}
}

// TestMouseMovement tests mouse movement functionality
func TestMouseMovement(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("No DISPLAY set, skipping real X11 test")
	}

	xc, err := NewX11Controller()
	if err != nil {
		t.Fatalf("Failed to create X11Controller: %v", err)
	}
	defer xc.Close()

	// Test moving mouse to various positions
	positions := []struct {
		x, y int16
	}{
		{100, 100},
		{200, 200},
		{0, 0},
		{500, 500},
	}

	for _, pos := range positions {
		err := xc.MoveMouse(pos.x, pos.y)
		if err != nil {
			t.Errorf("Failed to move mouse to (%d, %d): %v", pos.x, pos.y, err)
		}
		// Small delay to allow X11 to process
		time.Sleep(10 * time.Millisecond)
	}
}

// TestMouseClick tests mouse clicking functionality
func TestMouseClick(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("No DISPLAY set, skipping real X11 test")
	}

	xc, err := NewX11Controller()
	if err != nil {
		t.Fatalf("Failed to create X11Controller: %v", err)
	}
	defer xc.Close()

	// Test different mouse buttons
	buttons := []uint8{1, 2, 3} // Left, Middle, Right

	for _, button := range buttons {
		err := xc.Click(button)
		if err != nil {
			t.Errorf("Failed to click button %d: %v", button, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestSendKeyEvent tests the key event sending
func TestSendKeyEvent(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("No DISPLAY set, skipping real X11 test")
	}

	xc, err := NewX11Controller()
	if err != nil {
		t.Fatalf("Failed to create X11Controller: %v", err)
	}
	defer xc.Close()

	// Test sending key press and release
	testKeycode := x.Keycode(38) // Usually 'a'

	// Press
	err = xc.sendKeyEvent(xc.root, testKeycode, true)
	if err != nil {
		t.Errorf("Failed to send key press: %v", err)
	}

	// Release
	err = xc.sendKeyEvent(xc.root, testKeycode, false)
	if err != nil {
		t.Errorf("Failed to send key release: %v", err)
	}
}

// TestTypeChar tests typing individual characters
func TestTypeChar(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("No DISPLAY set, skipping real X11 test")
	}

	xc, err := NewX11Controller()
	if err != nil {
		t.Fatalf("Failed to create X11Controller: %v", err)
	}
	defer xc.Close()

	// Test various characters
	testChars := []rune{
		'a', 'A', // Lowercase and uppercase
		'1', '!', // Number and shifted number
		' ',  // Space
		'\n', // Newline
		'\t', // Tab
	}

	for _, char := range testChars {
		err := xc.typeChar(xc.root, char)
		if err != nil {
			t.Errorf("Failed to type character %c: %v", char, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestTypeCharWithModifier tests typing with modifier keys
func TestTypeCharWithModifier(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("No DISPLAY set, skipping real X11 test")
	}

	xc, err := NewX11Controller()
	if err != nil {
		t.Fatalf("Failed to create X11Controller: %v", err)
	}
	defer xc.Close()

	// Test modifier combinations
	tests := []struct {
		char     rune
		modifier string
	}{
		{'a', "Ctrl"},
		{'c', "Ctrl"},
		{'v', "Ctrl"},
		{'a', "Alt"},
		{'F', "Alt"},
	}

	for _, test := range tests {
		err := xc.typeCharWithModifier(xc.root, test.char, test.modifier)
		if err != nil {
			t.Errorf("Failed to type %s+%c: %v", test.modifier, test.char, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestTypeText tests the full text typing functionality
func TestTypeText(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("No DISPLAY set, skipping real X11 test")
	}

	xc, err := NewX11Controller()
	if err != nil {
		t.Fatalf("Failed to create X11Controller: %v", err)
	}
	defer xc.Close()

	// Test various text inputs
	testTexts := []string{
		"Hello World",
		"Testing 123",
		"Special chars: !@#$%",
		"Mixed CASE text",
		"Line1\nLine2\nLine3",
		"Ctrl+A",
		"Ctrl+C",
		"Alt+Tab",
	}

	for _, text := range testTexts {
		err := xc.TypeText(text)
		if err != nil {
			t.Errorf("Failed to type text '%s': %v", text, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// TestCaptureScreen tests screenshot capture
func TestCaptureScreen(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("No DISPLAY set, skipping real X11 test")
	}

	xc, err := NewX11Controller()
	if err != nil {
		t.Fatalf("Failed to create X11Controller: %v", err)
	}
	defer xc.Close()

	// Capture screen
	img, err := xc.captureScreen()
	if err != nil {
		t.Fatalf("Failed to capture screen: %v", err)
	}

	// Verify image dimensions
	bounds := img.Bounds()
	if bounds.Dx() != int(xc.screen.WidthInPixels) {
		t.Errorf("Image width mismatch: got %d, want %d", bounds.Dx(), xc.screen.WidthInPixels)
	}

	if bounds.Dy() != int(xc.screen.HeightInPixels) {
		t.Errorf("Image height mismatch: got %d, want %d", bounds.Dy(), xc.screen.HeightInPixels)
	}
}

// TestGetScreenshotData tests getting screenshot as PNG data
func TestGetScreenshotData(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("No DISPLAY set, skipping real X11 test")
	}

	xc, err := NewX11Controller()
	if err != nil {
		t.Fatalf("Failed to create X11Controller: %v", err)
	}
	defer xc.Close()

	// Get screenshot data
	data, err := xc.GetScreenshotData()
	if err != nil {
		t.Fatalf("Failed to get screenshot data: %v", err)
	}

	// Verify it's valid PNG data
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Failed to decode PNG data: %v", err)
	}

	// Verify dimensions
	bounds := img.Bounds()
	if bounds.Dx() != int(xc.screen.WidthInPixels) || bounds.Dy() != int(xc.screen.HeightInPixels) {
		t.Error("Screenshot dimensions don't match screen dimensions")
	}
}

// TestTakeScreenshot tests saving screenshot to file
func TestTakeScreenshot(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("No DISPLAY set, skipping real X11 test")
	}

	xc, err := NewX11Controller()
	if err != nil {
		t.Fatalf("Failed to create X11Controller: %v", err)
	}
	defer xc.Close()

	// Take screenshot to temporary file
	tmpfile := "/tmp/test_screenshot.png"
	err = xc.TakeScreenshot(tmpfile)
	if err != nil {
		t.Fatalf("Failed to take screenshot: %v", err)
	}

	// Verify file exists and is valid PNG
	file, err := os.Open(tmpfile)
	if err != nil {
		t.Fatalf("Failed to open screenshot file: %v", err)
	}
	defer file.Close()
	defer os.Remove(tmpfile)

	_, err = png.Decode(file)
	if err != nil {
		t.Fatalf("Screenshot file is not valid PNG: %v", err)
	}
}

// TestErrorHandling tests error handling for invalid operations
func TestErrorHandling(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("No DISPLAY set, skipping real X11 test")
	}

	xc, err := NewX11Controller()
	if err != nil {
		t.Fatalf("Failed to create X11Controller: %v", err)
	}

	// Close connection
	xc.Close()

	// Test operations after closing
	_, err = xc.GetScreenInfo()
	if err == nil {
		t.Error("Expected error when getting screen info on closed connection")
	}

	err = xc.MoveMouse(100, 100)
	if err == nil {
		t.Error("Expected error when moving mouse on closed connection")
	}

	err = xc.Click(1)
	if err == nil {
		t.Error("Expected error when clicking on closed connection")
	}

	err = xc.TypeText("test")
	if err == nil {
		t.Error("Expected error when typing on closed connection")
	}

	_, err = xc.captureScreen()
	if err == nil {
		t.Error("Expected error when capturing screen on closed connection")
	}
}

// BenchmarkTypeText benchmarks text typing performance
func BenchmarkTypeText(b *testing.B) {
	xc, err := NewX11Controller()
	if err != nil {
		b.Fatalf("Failed to create X11Controller: %v", err)
	}
	defer xc.Close()

	testText := "Hello World!"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := xc.TypeText(testText)
		if err != nil {
			b.Fatalf("Failed to type text: %v", err)
		}
	}
}

// BenchmarkCaptureScreen benchmarks screenshot performance
func BenchmarkCaptureScreen(b *testing.B) {
	xc, err := NewX11Controller()
	if err != nil {
		b.Fatalf("Failed to create X11Controller: %v", err)
	}
	defer xc.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := xc.captureScreen()
		if err != nil {
			b.Fatalf("Failed to capture screen: %v", err)
		}
	}
}

// BenchmarkMouseMovement benchmarks mouse movement performance
func BenchmarkMouseMovement(b *testing.B) {
	xc, err := NewX11Controller()
	if err != nil {
		b.Fatalf("Failed to create X11Controller: %v", err)
	}
	defer xc.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		x := int16(i % 1024)
		y := int16(i % 768)
		err := xc.MoveMouse(x, y)
		if err != nil {
			b.Fatalf("Failed to move mouse: %v", err)
		}
	}
}
