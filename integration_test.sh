#!/bin/bash
# Simple integration test for X11 MCP Controller

set -e

echo "=== X11 MCP Controller Integration Test ==="
echo

# Build the controller
echo "Building controller..."
GOPATH=$HOME/go:/usr/share/gocode go build -o mcp-x11-controller main.go || {
    echo "Build failed!"
    exit 1
}

echo "Build successful!"
echo

# Test 1: Run with xterm
echo "Test 1: Testing with xterm..."
cat > test_xterm.exp << 'EOF'
#!/usr/bin/expect -f
set timeout 10

# Start the controller with xterm
spawn ./mcp-x11-controller -program="xterm -e bash" -wm="openbox"

# Wait for startup
sleep 3

# Send get_screen_info command
send_user "\nSending screen info request...\n"
send "{\"jsonrpc\":\"2.0\",\"method\":\"tools/call\",\"params\":{\"name\":\"get_screen_info\",\"arguments\":{}},\"id\":1}\n"

# Expect response
expect {
    "*1024x768*" {
        send_user "✓ Got screen info\n"
    }
    timeout {
        send_user "✗ Timeout waiting for screen info\n"
        exit 1
    }
}

# Send screenshot command
send_user "Taking screenshot...\n"
send "{\"jsonrpc\":\"2.0\",\"method\":\"tools/call\",\"params\":{\"name\":\"take_screenshot\",\"arguments\":{\"filename\":\"test_shot.png\"}},\"id\":2}\n"

# Wait for response
expect {
    "*image/png*" {
        send_user "✓ Screenshot taken\n"
    }
    timeout {
        send_user "✗ Timeout waiting for screenshot\n"
        exit 1
    }
}

# Type some text
send_user "Typing test text...\n"
send "{\"jsonrpc\":\"2.0\",\"method\":\"tools/call\",\"params\":{\"name\":\"type_text\",\"arguments\":{\"text\":\"echo Hello from X11 controller\"}},\"id\":3}\n"

expect {
    "*Typed:*" {
        send_user "✓ Text typed\n"
    }
    timeout {
        send_user "✗ Timeout waiting for type response\n"
    }
}

# Clean up
send_user "Test completed!\n"
send "\003"
exit 0
EOF

if command -v expect > /dev/null 2>&1; then
    chmod +x test_xterm.exp
    if xvfb-run -s "-screen 0 1024x768x24" ./test_xterm.exp; then
        echo "✓ xterm test passed"
    else
        echo "✗ xterm test failed"
    fi
    rm -f test_xterm.exp test_shot.png
else
    echo "Skipping expect test (expect not installed)"
fi

echo

# Test 2: Run Go tests
echo "Test 2: Running Go unit tests..."
if GOPATH=$HOME/go:/usr/share/gocode go test -short -v; then
    echo "✓ Go tests passed"
else
    echo "✗ Go tests failed"
fi

echo
echo "Integration tests completed!"