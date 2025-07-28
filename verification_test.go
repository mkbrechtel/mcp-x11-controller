package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestActualFunctionality runs the MCP server and verifies it actually works
func TestActualFunctionality(t *testing.T) {
	// This test requires Xvfb
	if _, err := exec.LookPath("Xvfb"); err != nil {
		t.Skip("Xvfb not installed")
	}

	// Start Xvfb on a unique display
	display := fmt.Sprintf(":%d", 99+os.Getpid()%100)
	xvfb := exec.Command("Xvfb", display, "-screen", "0", "1024x768x24", "-ac")
	if err := xvfb.Start(); err != nil {
		t.Fatalf("Failed to start Xvfb: %v", err)
	}
	defer func() {
		xvfb.Process.Kill()
		xvfb.Wait()
	}()

	// Wait for Xvfb to start
	time.Sleep(2 * time.Second)

	// Set DISPLAY for child process
	os.Setenv("DISPLAY", display)

	// Build the binary first
	build := exec.Command("go", "build", "-o", "test-mcp-x11", "main.go")
	build.Env = append(os.Environ(), "GOPATH=/home/mkbrechtel/go:/usr/share/gocode")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build: %v\nOutput: %s", err, output)
	}
	defer os.Remove("test-mcp-x11")

	// Start the MCP server
	server := exec.Command("./test-mcp-x11", "-xvfb=false", "-wm=", "-program=")
	server.Env = append(os.Environ(), fmt.Sprintf("DISPLAY=%s", display))
	
	stdin, err := server.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to create stdin pipe: %v", err)
	}
	
	stdout, err := server.StdoutPipe()
	if err != nil {
		t.Fatalf("Failed to create stdout pipe: %v", err)
	}
	
	stderr, err := server.StderrPipe()
	if err != nil {
		t.Fatalf("Failed to create stderr pipe: %v", err)
	}

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() {
		server.Process.Kill()
		server.Wait()
	}()

	// Read stderr in background
	go io.Copy(os.Stderr, stderr)

	// Wait for server to start
	time.Sleep(2 * time.Second)

	// Initialize the MCP session
	initRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
		"id": 0,
	}

	reqBytes, _ := json.Marshal(initRequest)
	stdin.Write(append(reqBytes, '\n'))

	decoder := json.NewDecoder(stdout)
	var initResponse map[string]interface{}
	if err := decoder.Decode(&initResponse); err != nil {
		t.Fatalf("Failed to decode init response: %v", err)
	}

	if initResponse["error"] != nil {
		t.Fatalf("Failed to initialize: %v", initResponse["error"])
	}

	// Send initialized notification
	initializedNotif := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}
	
	notifBytes, _ := json.Marshal(initializedNotif)
	stdin.Write(append(notifBytes, '\n'))

	// Test 1: Can we get screen info?
	t.Run("ScreenInfo", func(t *testing.T) {
		request := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "tools/call",
			"params": map[string]interface{}{
				"name":      "get_screen_info",
				"arguments": map[string]interface{}{},
			},
			"id": 1,
		}

		reqBytes, _ := json.Marshal(request)
		stdin.Write(append(reqBytes, '\n'))

		decoder := json.NewDecoder(stdout)
		var response map[string]interface{}
		if err := decoder.Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response["error"] != nil {
			t.Fatalf("Got error: %v", response["error"])
		}

		// Check that we got screen dimensions
		result := response["result"].(map[string]interface{})
		content := result["content"].([]interface{})
		if len(content) == 0 {
			t.Fatal("No content in response")
		}
		
		textContent := content[0].(map[string]interface{})
		text := textContent["text"].(string)
		
		if !strings.Contains(text, "1024x768") {
			t.Errorf("Expected 1024x768 in response, got: %s", text)
		}
	})

	// Test 2: Can we click at coordinates (new combined tool)?
	t.Run("ClickAt", func(t *testing.T) {
		request := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "tools/call",
			"params": map[string]interface{}{
				"name": "click_at",
				"arguments": map[string]interface{}{
					"x": 500,
					"y": 400,
					"button": 1,
				},
			},
			"id": 2,
		}

		reqBytes, _ := json.Marshal(request)
		stdin.Write(append(reqBytes, '\n'))

		decoder := json.NewDecoder(stdout)
		var response map[string]interface{}
		if err := decoder.Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response["error"] != nil {
			t.Fatalf("Got error: %v", response["error"])
		}

		// Verify response
		result := response["result"].(map[string]interface{})
		content := result["content"].([]interface{})
		if len(content) == 0 {
			t.Fatal("No content in response")
		}
		
		textContent := content[0].(map[string]interface{})
		text := textContent["text"].(string)
		
		if !strings.Contains(text, "Clicked button 1 at (500, 400)") {
			t.Errorf("Expected click confirmation, got: %s", text)
		}
	})

	// Test 3: Can we start a program?
	t.Run("StartProgram", func(t *testing.T) {
		// Try to find a simple X11 program
		testProgram := ""
		for _, prog := range []string{"xeyes", "xclock", "xterm"} {
			if _, err := exec.LookPath(prog); err == nil {
				testProgram = prog
				break
			}
		}
		
		if testProgram == "" {
			t.Skip("No suitable test program found")
		}

		request := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "tools/call",
			"params": map[string]interface{}{
				"name": "start_program",
				"arguments": map[string]interface{}{
					"program": testProgram,
				},
			},
			"id": 3,
		}

		reqBytes, _ := json.Marshal(request)
		stdin.Write(append(reqBytes, '\n'))

		decoder := json.NewDecoder(stdout)
		var response map[string]interface{}
		if err := decoder.Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response["error"] != nil {
			t.Fatalf("Got error: %v", response["error"])
		}

		// Clean up
		time.Sleep(500 * time.Millisecond)
		exec.Command("pkill", testProgram).Run()
	})

	// Test 4: Can we type text?
	t.Run("TypeText", func(t *testing.T) {
		request := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "tools/call",
			"params": map[string]interface{}{
				"name": "type_text",
				"arguments": map[string]interface{}{
					"text": "Hello World! Testing 123",
				},
			},
			"id": 4,
		}

		reqBytes, _ := json.Marshal(request)
		stdin.Write(append(reqBytes, '\n'))

		decoder := json.NewDecoder(stdout)
		var response map[string]interface{}
		if err := decoder.Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response["error"] != nil {
			t.Fatalf("Got error: %v", response["error"])
		}

		// Verify we got a success response
		result := response["result"].(map[string]interface{})
		content := result["content"].([]interface{})
		if len(content) == 0 {
			t.Fatal("No content in response")
		}
		
		textContent := content[0].(map[string]interface{})
		text := textContent["text"].(string)
		
		if !strings.Contains(text, "Typed:") {
			t.Errorf("Expected 'Typed:' in response, got: %s", text)
		}
	})

	// Test 5: Can we take a screenshot?
	t.Run("Screenshot", func(t *testing.T) {
		request := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "tools/call",
			"params": map[string]interface{}{
				"name":      "take_screenshot",
				"arguments": map[string]interface{}{},
			},
			"id": 5,
		}

		reqBytes, _ := json.Marshal(request)
		stdin.Write(append(reqBytes, '\n'))

		decoder := json.NewDecoder(stdout)
		var response map[string]interface{}
		if err := decoder.Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response["error"] != nil {
			t.Fatalf("Got error: %v", response["error"])
		}

		// Verify we got image data
		result := response["result"].(map[string]interface{})
		content := result["content"].([]interface{})
		if len(content) == 0 {
			t.Fatal("No content in response")
		}
		
		imageContent := content[0].(map[string]interface{})
		if imageContent["mimeType"] != "image/png" {
			t.Errorf("Expected image/png, got: %v", imageContent["mimeType"])
		}
		
		if imageContent["data"] == nil {
			t.Error("No image data in response")
		}
	})
}