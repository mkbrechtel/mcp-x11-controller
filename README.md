# X11 MCP Controller

An MCP (Model Context Protocol) server that allows LLMs to control X11 displays. It can automatically start Xvfb (virtual display), a window manager, and a program of your choice.

## ⚠️ WARNING

**This code is 100% vibe-coded and has not been reviewed by any human. It is not quality controlled code. Use at your own risk!**

## Features

- **Automatic Xvfb setup** - Creates a virtual X11 display automatically
- **Window manager support** - Automatically launches i3 (with -a flag) or other window managers
- **Program launcher** - Starts Firefox or any specified program
- **X11 Control Tools**:
  - Get screen information (dimensions, root window)
  - Move mouse cursor to specific coordinates
  - Click mouse buttons (left, middle, right)
  - Type text by sending key events

## Building

```bash
go build
```

## Usage

### Default mode

```bash
./mcp-x11-controller
```

This will:
1. Connect to X11 (or start Xvfb if no DISPLAY is set)
2. Launch i3 window manager with -a flag (disables autostart/wizard)
3. Begin accepting MCP commands

### Command-line options

```bash
./mcp-x11-controller [flags]
```

Flags:
- `--no-wm` (bool): Disable automatic window manager startup
- `--wm-name` (string): Window manager to start (default: "i3 -a")
- `--help` (bool): Show help message
- `--version` (bool): Show version

### Examples

Run with a different resolution and Chrome:
```bash
./mcp-x11-controller -resolution "1920x1080" -program "google-chrome"
```

Run without window manager:
```bash
./mcp-x11-controller --no-wm
```

Run with a different window manager:
```bash
./mcp-x11-controller -wm ""
```

Use existing X11 display:
```bash
./mcp-x11-controller -xvfb=false -display ":0"
```

## MCP Tools

### get_screen_info
Get information about the X11 screen.

**Arguments:** None

**Returns:** Screen width, height, and root window ID

### click_at
Move the mouse cursor to specific coordinates and click.

**Arguments:**
- `x` (number): X coordinate
- `y` (number): Y coordinate
- `button` (number, optional): Button number (1=left, 2=middle, 3=right). Default: 1

### type_text
Type text by sending keyboard events.

**Arguments:**
- `text` (string): Text to type

**Note:** Currently supports:
- All ASCII characters and symbols
- Uppercase/lowercase with proper shift handling
- Special characters (!@#$%^&*() etc.)
- Newline character (\n) is automatically converted to Enter key

**Limitations:**
- Does NOT support other special keys like Tab or Backspace
- Does NOT support modifier key combinations (Ctrl+A, Alt+Tab, etc.)
- For other special keys and combinations, use the `key_press` tool

### key_press
Press special keys or key combinations.

**Arguments:**
- `key` (string, optional): Special key name (e.g., "Enter", "Tab", "Escape", "BackSpace", "Delete", "Home", "End", "PageUp", "PageDown", "Left", "Right", "Up", "Down")
- `combo` (string, optional): Key combination (e.g., "ctrl+c", "alt+tab", "ctrl+shift+t", "super+l")

**Note:** You must provide either `key` OR `combo`, not both.

**Supported modifiers for combinations:**
- `ctrl` - Control key
- `shift` - Shift key
- `alt` - Alt key
- `super` / `win` / `cmd` - Super/Windows/Command key

### take_screenshot
Take a screenshot of the X11 display and return the image data directly.

**Arguments:**
- `filename` (string, optional): If provided, also saves the screenshot to this file

**Returns:** PNG image data that can be viewed directly

### start_program
Start a desktop program in the background.

**Arguments:**
- `program` (string): Program name or path to executable
- `args` (array of strings, optional): Command line arguments

## Testing with Xvfb

To test without a real display:

```bash
# Start Xvfb
Xvfb :99 -screen 0 1024x768x24 &

# Start xterm (optional, to see the effects)
export DISPLAY=:99
xterm &

# Run the controller
export DISPLAY=:99
./mcp-x11-controller
```

## Testing

### Unit Tests

Run the Go unit tests:
```bash
go test -v
```

Run with X11 (requires display):
```bash
DISPLAY=:0 go test -v
```

Run in short mode (skips X11 tests):
```bash
go test -short -v
```

### Integration Tests

Run the comprehensive test suite:
```bash
./test_x11_controller.sh
```

Run the simple integration test:
```bash
./integration_test.sh
```

### Benchmarks

Run performance benchmarks:
```bash
go test -bench=. -benchmem
```

## Example MCP Requests

Get screen info:
```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "get_screen_info",
    "arguments": {}
  },
  "id": 1
}
```

Click at coordinates:
```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "click_at",
    "arguments": {"x": 100, "y": 200, "button": 1}
  },
  "id": 2
}
```

Type text with newlines:
```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "type_text",
    "arguments": {"text": "Hello World!\nThis is a new line"}
  },
  "id": 3
}
```

Press special keys:
```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "key_press",
    "arguments": {"key": "Enter"}
  },
  "id": 4
}
```

Press key combinations:
```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "key_press",
    "arguments": {"combo": "ctrl+c"}
  },
  "id": 5
}
```