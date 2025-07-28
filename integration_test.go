// +build integration

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestIntegrationFullStack tests the entire MCP server with real X11
func TestIntegrationFullStack(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Start Xvfb
	xvfb := exec.Command("Xvfb", ":99", "-screen", "0", "1024x768x24", "-ac")
	if err := xvfb.Start(); err != nil {
		t.Fatalf("Failed to start Xvfb: %v", err)
	}
	defer xvfb.Process.Kill()

	// Set display
	os.Setenv("DISPLAY", ":99")
	time.Sleep(2 * time.Second)

	// Start the MCP server
	serverCmd := exec.Command("./mcp-x11-controller", "-xvfb=false", "-wm=", "-program=")
	stdin, err := serverCmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to create stdin pipe: %v", err)
	}
	stdout, err := serverCmd.StdoutPipe()
	if err != nil {
		t.Fatalf("Failed to create stdout pipe: %v", err)
	}
	
	if err := serverCmd.Start(); err != nil {
		t.Fatalf("Failed to start MCP server: %v", err)
	}
	defer serverCmd.Process.Kill()

	time.Sleep(1 * time.Second)

	// Test helper to send MCP request and get response
	sendRequest := func(t *testing.T, method string, toolName string, args map[string]interface{}) map[string]interface{} {
		request := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  method,
			"params": map[string]interface{}{
				"name":      toolName,
				"arguments": args,
			},
			"id": time.Now().UnixNano(),
		}

		reqBytes, err := json.Marshal(request)
		if err != nil {
			t.Fatalf("Failed to marshal request: %v", err)
		}

		if _, err := stdin.Write(append(reqBytes, '\n')); err != nil {
			t.Fatalf("Failed to write request: %v", err)
		}

		// Read response
		decoder := json.NewDecoder(stdout)
		var response map[string]interface{}
		if err := decoder.Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		return response
	}

	// Test 1: Get screen info
	t.Run("GetScreenInfo", func(t *testing.T) {
		resp := sendRequest(t, "tools/call", "get_screen_info", map[string]interface{}{})
		
		if errVal, ok := resp["error"]; ok {
			t.Fatalf("Got error response: %v", errVal)
		}

		result, ok := resp["result"].(map[string]interface{})
		if !ok {
			t.Fatalf("Invalid result format: %v", resp["result"])
		}

		content, ok := result["content"].([]interface{})
		if !ok || len(content) == 0 {
			t.Fatalf("No content in response: %v", result)
		}

		textContent := content[0].(map[string]interface{})
		text := textContent["text"].(string)
		
		if !strings.Contains(text, "1024x768") {
			t.Errorf("Expected screen resolution 1024x768, got: %s", text)
		}
	})

	// Test 2: Take screenshot
	t.Run("TakeScreenshot", func(t *testing.T) {
		resp := sendRequest(t, "tools/call", "take_screenshot", map[string]interface{}{})
		
		if errVal, ok := resp["error"]; ok {
			t.Fatalf("Got error response: %v", errVal)
		}

		result := resp["result"].(map[string]interface{})
		content := result["content"].([]interface{})
		
		if len(content) == 0 {
			t.Fatal("No content in screenshot response")
		}

		imageContent := content[0].(map[string]interface{})
		if imageContent["mimeType"] != "image/png" {
			t.Errorf("Expected image/png, got: %s", imageContent["mimeType"])
		}

		// Verify it's valid PNG data
		imageData := imageContent["data"].([]byte)
		if _, err := png.Decode(bytes.NewReader(imageData)); err != nil {
			t.Errorf("Invalid PNG data: %v", err)
		}
	})

	// Test 3: Click at coordinates (new combined tool)
	t.Run("ClickAt", func(t *testing.T) {
		positions := []struct {
			x, y   float64
			button float64
		}{
			{100, 100, 1},
			{500, 300, 2},
			{800, 600, 3},
		}

		for _, pos := range positions {
			resp := sendRequest(t, "tools/call", "click_at", map[string]interface{}{
				"x":      pos.x,
				"y":      pos.y,
				"button": pos.button,
			})
			
			if errVal, ok := resp["error"]; ok {
				t.Errorf("Failed to click at (%.0f, %.0f) with button %.0f: %v", pos.x, pos.y, pos.button, errVal)
			}
		}
	})

	// Test 4: Start program
	t.Run("StartProgram", func(t *testing.T) {
		// Check if xeyes is available
		if _, err := exec.LookPath("xeyes"); err != nil {
			t.Skip("xeyes not installed, skipping test")
		}

		resp := sendRequest(t, "tools/call", "start_program", map[string]interface{}{
			"program": "xeyes",
		})
		
		if errVal, ok := resp["error"]; ok {
			t.Errorf("Failed to start xeyes: %v", errVal)
		}
		
		time.Sleep(1 * time.Second)
		
		// Kill xeyes
		exec.Command("pkill", "xeyes").Run()
	})
}

// TestIntegrationKeyboardWithXterm tests keyboard input with a real application
func TestIntegrationKeyboardWithXterm(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Check if xterm is available
	if _, err := exec.LookPath("xterm"); err != nil {
		t.Skip("xterm not installed, skipping test")
	}

	// Start Xvfb
	xvfb := exec.Command("Xvfb", ":99", "-screen", "0", "1024x768x24", "-ac")
	if err := xvfb.Start(); err != nil {
		t.Fatalf("Failed to start Xvfb: %v", err)
	}
	defer xvfb.Process.Kill()

	os.Setenv("DISPLAY", ":99")
	time.Sleep(2 * time.Second)

	// Start xterm with a command that will capture input
	tmpFile := "/tmp/x11_test_output.txt"
	defer os.Remove(tmpFile)

	xterm := exec.Command("xterm", "-geometry", "80x24+0+0", "-e", 
		fmt.Sprintf("bash -c 'echo Ready for input > %s; cat >> %s'", tmpFile, tmpFile))
	xterm.Env = append(os.Environ(), "DISPLAY=:99")
	if err := xterm.Start(); err != nil {
		t.Fatalf("Failed to start xterm: %v", err)
	}
	defer xterm.Process.Kill()

	time.Sleep(1 * time.Second)

	// Start the MCP server
	serverCmd := exec.Command("./mcp-x11-controller", "-xvfb=false", "-wm=", "-program=")
	stdin, err := serverCmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to create stdin pipe: %v", err)
	}
	stdout, err := serverCmd.StdoutPipe()
	if err != nil {
		t.Fatalf("Failed to create stdout pipe: %v", err)
	}
	
	if err := serverCmd.Start(); err != nil {
		t.Fatalf("Failed to start MCP server: %v", err)
	}
	defer serverCmd.Process.Kill()

	time.Sleep(1 * time.Second)

	// Click on xterm window to focus it using click_at
	sendMCPRequest(t, stdin, stdout, "tools/call", "click_at", map[string]interface{}{
		"x":      400.0,
		"y":      300.0,
		"button": 1.0,
	})

	// Type test text
	testText := "Hello World!\nTesting 123"
	resp := sendMCPRequest(t, stdin, stdout, "tools/call", "type_text", map[string]interface{}{
		"text": testText,
	})

	if errVal, ok := resp["error"]; ok {
		t.Fatalf("Failed to type text: %v", errVal)
	}

	// Send Ctrl+D to close the cat command
	sendMCPRequest(t, stdin, stdout, "tools/call", "type_text", map[string]interface{}{
		"text": "Ctrl+D",
	})

	time.Sleep(500 * time.Millisecond)

	// Check if text was typed
	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	if !strings.Contains(string(content), "Hello World!") {
		t.Errorf("Expected 'Hello World!' in output, got: %s", string(content))
	}

	if !strings.Contains(string(content), "Testing 123") {
		t.Errorf("Expected 'Testing 123' in output, got: %s", string(content))
	}
}

// TestIntegrationKeyboardSpecialChars tests special characters and modifiers
func TestIntegrationKeyboardSpecialChars(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Start Xvfb
	xvfb := exec.Command("Xvfb", ":99", "-screen", "0", "1024x768x24", "-ac")
	if err := xvfb.Start(); err != nil {
		t.Fatalf("Failed to start Xvfb: %v", err)
	}
	defer xvfb.Process.Kill()

	os.Setenv("DISPLAY", ":99")
	time.Sleep(2 * time.Second)

	// Start the MCP server
	serverCmd := exec.Command("./mcp-x11-controller", "-xvfb=false", "-wm=", "-program=")
	stdin, err := serverCmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to create stdin pipe: %v", err)
	}
	stdout, err := serverCmd.StdoutPipe()
	if err != nil {
		t.Fatalf("Failed to create stdout pipe: %v", err)
	}
	
	if err := serverCmd.Start(); err != nil {
		t.Fatalf("Failed to start MCP server: %v", err)
	}
	defer serverCmd.Process.Kill()

	time.Sleep(1 * time.Second)

	// Test various special characters
	testCases := []string{
		"!@#$%^&*()",
		"UPPERCASE lowercase",
		"Line1\nLine2\nLine3",
		"Tab\there\ttest",
		"Quotes: \"single' and double\"",
		"Brackets: []{}<>",
		"Math: 1+2=3, 4*5=20",
	}

	for _, text := range testCases {
		resp := sendMCPRequest(t, stdin, stdout, "tools/call", "type_text", map[string]interface{}{
			"text": text,
		})

		if errVal, ok := resp["error"]; ok {
			t.Errorf("Failed to type text '%s': %v", text, errVal)
		}
	}

	// Test modifier keys
	modifierTests := []string{
		"Ctrl+A",
		"Ctrl+C",
		"Ctrl+V",
		"Alt+F",
		"Alt+Tab",
	}

	for _, text := range modifierTests {
		resp := sendMCPRequest(t, stdin, stdout, "tools/call", "type_text", map[string]interface{}{
			"text": text,
		})

		if errVal, ok := resp["error"]; ok {
			t.Errorf("Failed to type modifier combo '%s': %v", text, errVal)
		}
	}
}

// Helper function to send MCP request
func sendMCPRequest(t *testing.T, stdin io.Writer, stdout io.Reader, method string, toolName string, args map[string]interface{}) map[string]interface{} {
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params": map[string]interface{}{
			"name":      toolName,
			"arguments": args,
		},
		"id": time.Now().UnixNano(),
	}

	reqBytes, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	if _, err := stdin.Write(append(reqBytes, '\n')); err != nil {
		t.Fatalf("Failed to write request: %v", err)
	}

	// Read response
	decoder := json.NewDecoder(stdout)
	var response map[string]interface{}
	if err := decoder.Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	return response
}

// TestBuildAndRun ensures the binary builds and runs
func TestBuildAndRun(t *testing.T) {
	// Build the binary
	cmd := exec.Command("go", "build", "-o", "mcp-x11-controller", "main.go")
	cmd.Env = append(os.Environ(), "GOPATH=/home/mkbrechtel/go:/usr/share/gocode")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to build: %v\nOutput: %s", err, output)
	}

	// Test that it starts up
	serverCmd := exec.Command("./mcp-x11-controller", "-h")
	output, err = serverCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run help: %v\nOutput: %s", err, output)
	}

	if !strings.Contains(string(output), "Usage of ./mcp-x11-controller") {
		t.Errorf("Expected usage output, got: %s", output)
	}
}

// TestFirefoxAddressBarInput tests the specific Firefox address bar input issue
func TestFirefoxAddressBarInput(t *testing.T) {
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
	xvfb := exec.Command("Xvfb", ":99", "-screen", "0", "1024x768x24", "-ac")
	if err := xvfb.Start(); err != nil {
		t.Fatalf("Failed to start Xvfb: %v", err)
	}
	defer xvfb.Process.Kill()

	os.Setenv("DISPLAY", ":99")
	time.Sleep(2 * time.Second)

	// Start the MCP server
	serverCmd := exec.Command("./mcp-x11-controller", "-xvfb=false", "-wm=", "-program=")
	stdin, err := serverCmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to create stdin pipe: %v", err)
	}
	stdout, err := serverCmd.StdoutPipe()
	if err != nil {
		t.Fatalf("Failed to create stdout pipe: %v", err)
	}
	
	if err := serverCmd.Start(); err != nil {
		t.Fatalf("Failed to start MCP server: %v", err)
	}
	defer serverCmd.Process.Kill()

	time.Sleep(1 * time.Second)

	// Start Firefox using the new start_program tool
	resp := sendMCPRequest(t, stdin, stdout, "tools/call", "start_program", map[string]interface{}{
		"program": "firefox",
		"args":    []string{"--new-instance", "about:blank"},
	})
	
	if errVal, ok := resp["error"]; ok {
		t.Fatalf("Failed to start Firefox: %v", errVal)
	}
	defer exec.Command("pkill", "firefox").Run()

	// Wait for Firefox to start
	t.Log("Waiting for Firefox to start...")
	time.Sleep(8 * time.Second)

	// Take initial screenshot
	resp = sendMCPRequest(t, stdin, stdout, "tools/call", "take_screenshot", map[string]interface{}{})
	if errVal, ok := resp["error"]; ok {
		t.Errorf("Failed to take initial screenshot: %v", errVal)
	}

	// Test different methods to input text in address bar
	t.Run("Method1_DirectClick", func(t *testing.T) {
		// Click on address bar
		resp := sendMCPRequest(t, stdin, stdout, "tools/call", "click_at", map[string]interface{}{
			"x": 400.0,
			"y": 63.0,
		})
		if errVal, ok := resp["error"]; ok {
			t.Errorf("Failed to click address bar: %v", errVal)
		}
		time.Sleep(500 * time.Millisecond)

		// Type Ctrl+A to select all
		resp = sendMCPRequest(t, stdin, stdout, "tools/call", "type_text", map[string]interface{}{
			"text": "Ctrl+A",
		})
		if errVal, ok := resp["error"]; ok {
			t.Errorf("Failed to type Ctrl+A: %v", errVal)
		}
		time.Sleep(500 * time.Millisecond)

		// Type URL
		resp = sendMCPRequest(t, stdin, stdout, "tools/call", "type_text", map[string]interface{}{
			"text": "google.com",
		})
		if errVal, ok := resp["error"]; ok {
			t.Errorf("Failed to type URL: %v", errVal)
		}
		time.Sleep(500 * time.Millisecond)

		// Take screenshot
		sendMCPRequest(t, stdin, stdout, "tools/call", "take_screenshot", map[string]interface{}{})
	})

	t.Run("Method2_CtrlL", func(t *testing.T) {
		// Use Ctrl+L to focus address bar
		resp := sendMCPRequest(t, stdin, stdout, "tools/call", "type_text", map[string]interface{}{
			"text": "Ctrl+L",
		})
		if errVal, ok := resp["error"]; ok {
			t.Errorf("Failed to type Ctrl+L: %v", errVal)
		}
		time.Sleep(1000 * time.Millisecond)

		// Type URL
		resp = sendMCPRequest(t, stdin, stdout, "tools/call", "type_text", map[string]interface{}{
			"text": "example.com",
		})
		if errVal, ok := resp["error"]; ok {
			t.Errorf("Failed to type URL: %v", errVal)
		}
		time.Sleep(500 * time.Millisecond)

		// Take screenshot
		sendMCPRequest(t, stdin, stdout, "tools/call", "take_screenshot", map[string]interface{}{})
	})
}