package main

import (
	"os"
	"os/exec"
	"testing"
	"time"
)

// TestFirefoxInputDirectly tests Firefox input using the X11Controller directly
func TestFirefoxInputDirectly(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Firefox test in short mode")
	}

	// Check requirements
	for _, cmd := range []string{"Xvfb", "firefox"} {
		if _, err := exec.LookPath(cmd); err != nil {
			t.Skipf("%s not found, skipping test", cmd)
		}
	}

	// Start Xvfb
	display := ":99"
	xvfbCmd := exec.Command("Xvfb", display, "-screen", "0", "1024x768x24", "-ac")
	if err := xvfbCmd.Start(); err != nil {
		t.Fatalf("Failed to start Xvfb: %v", err)
	}
	defer func() {
		xvfbCmd.Process.Kill()
		xvfbCmd.Wait()
	}()

	// Set DISPLAY
	oldDisplay := os.Getenv("DISPLAY")
	os.Setenv("DISPLAY", display)
	defer os.Setenv("DISPLAY", oldDisplay)

	time.Sleep(2 * time.Second)

	// Create X11 controller
	xc, err := NewX11Controller()
	if err != nil {
		t.Fatalf("Failed to create X11Controller: %v", err)
	}
	defer xc.Close()

	// Start Firefox using StartProgram
	t.Log("Starting Firefox...")
	err = xc.StartProgram("firefox", []string{"--new-instance", "about:blank"})
	if err != nil {
		t.Fatalf("Failed to start Firefox: %v", err)
	}
	defer exec.Command("pkill", "firefox").Run()

	// Wait for Firefox to fully start
	t.Log("Waiting for Firefox to start...")
	time.Sleep(8 * time.Second)

	// Take initial screenshot
	data1, err := xc.GetScreenshotData()
	if err != nil {
		t.Errorf("Failed to take initial screenshot: %v", err)
	}
	t.Logf("Initial screenshot: %d bytes", len(data1))

	// Test different input methods
	testCases := []struct {
		name     string
		setup    func() error
		input    string
		waitTime time.Duration
	}{
		{
			name: "DirectClick",
			setup: func() error {
				// Click on address bar
				return xc.ClickAt(400, 63, 1)
			},
			input:    "google.com",
			waitTime: 500 * time.Millisecond,
		},
		{
			name: "CtrlL",
			setup: func() error {
				// Use Ctrl+L to focus address bar
				return xc.TypeText("Ctrl+L")
			},
			input:    "example.com",
			waitTime: 1000 * time.Millisecond,
		},
		{
			name: "F6",
			setup: func() error {
				// Use F6 to focus address bar
				return xc.TypeText("F6")
			},
			input:    "test.com",
			waitTime: 1000 * time.Millisecond,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup (click or keyboard shortcut)
			if err := tc.setup(); err != nil {
				t.Errorf("Failed setup for %s: %v", tc.name, err)
				return
			}
			time.Sleep(tc.waitTime)

			// Type Ctrl+A to select all
			if err := xc.TypeText("Ctrl+A"); err != nil {
				t.Errorf("Failed to type Ctrl+A: %v", err)
			}
			time.Sleep(200 * time.Millisecond)

			// Type the URL
			if err := xc.TypeText(tc.input); err != nil {
				t.Errorf("Failed to type URL: %v", err)
			}
			time.Sleep(500 * time.Millisecond)

			// Take screenshot
			data, err := xc.GetScreenshotData()
			if err != nil {
				t.Errorf("Failed to take screenshot: %v", err)
			}
			t.Logf("%s: Screenshot %d bytes", tc.name, len(data))

			// Save screenshot for debugging if test fails
			if t.Failed() {
				filename := "/tmp/firefox_test_" + tc.name + ".png"
				if err := os.WriteFile(filename, data, 0644); err == nil {
					t.Logf("Debug screenshot saved to: %s", filename)
				}
			}
		})
	}

	// Additional test: Try typing in a focused text field
	t.Run("XtermComparison", func(t *testing.T) {
		// Start xterm for comparison
		err := xc.StartProgram("xterm", []string{"-geometry", "80x24+100+100"})
		if err != nil {
			t.Skip("Failed to start xterm")
		}
		defer exec.Command("pkill", "xterm").Run()

		time.Sleep(2 * time.Second)

		// Click in xterm
		if err := xc.ClickAt(300, 300, 1); err != nil {
			t.Errorf("Failed to click in xterm: %v", err)
		}
		time.Sleep(500 * time.Millisecond)

		// Type some text
		testText := "echo 'This should appear in xterm'"
		if err := xc.TypeText(testText); err != nil {
			t.Errorf("Failed to type in xterm: %v", err)
		}
		time.Sleep(500 * time.Millisecond)

		// Take screenshot
		data, _ := xc.GetScreenshotData()
		t.Logf("Xterm test: Screenshot %d bytes", len(data))
	})
}

// TestKeyboardInputDebugging provides detailed debugging of keyboard input
func TestKeyboardInputDebugging(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	if _, err := exec.LookPath("Xvfb"); err != nil {
		t.Skip("Xvfb not found")
	}

	// Start Xvfb
	display := ":99"
	xvfbCmd := exec.Command("Xvfb", display, "-screen", "0", "1024x768x24", "-ac")
	if err := xvfbCmd.Start(); err != nil {
		t.Fatalf("Failed to start Xvfb: %v", err)
	}
	defer func() {
		xvfbCmd.Process.Kill()
		xvfbCmd.Wait()
	}()

	os.Setenv("DISPLAY", display)
	defer os.Setenv("DISPLAY", os.Getenv("DISPLAY"))

	time.Sleep(2 * time.Second)

	xc, err := NewX11Controller()
	if err != nil {
		t.Fatalf("Failed to create X11Controller: %v", err)
	}
	defer xc.Close()

	// Test individual character typing
	t.Run("IndividualCharacters", func(t *testing.T) {
		chars := "abcABC123!@#"
		for _, char := range chars {
			err := xc.typeChar(xc.root, char)
			if err != nil {
				t.Errorf("Failed to type character '%c': %v", char, err)
			}
			time.Sleep(50 * time.Millisecond)
		}
	})

	// Test modifier combinations
	t.Run("ModifierCombinations", func(t *testing.T) {
		modTests := []struct {
			char     rune
			modifier string
		}{
			{'a', "Ctrl"},
			{'l', "Ctrl"},
			{'c', "Ctrl"},
			{'v', "Ctrl"},
		}

		for _, test := range modTests {
			err := xc.typeCharWithModifier(xc.root, test.char, test.modifier)
			if err != nil {
				t.Errorf("Failed to type %s+%c: %v", test.modifier, test.char, err)
			}
			time.Sleep(50 * time.Millisecond)
		}
	})

	// Test focus detection
	t.Run("FocusDetection", func(t *testing.T) {
		// This would need a window to test properly
		t.Log("Focus detection test would require an active window")
	})
}