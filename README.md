# X11 MCP Controller

An MCP (Model Context Protocol) server that allows LLMs to control X11 displays. It can automatically start Xvfb (virtual display), a window manager, and a program of your choice.

## Features

- **Automatic Xvfb setup** - Creates a virtual X11 display automatically
- **Window manager support** - Launches i3 or other window managers
- **Program launcher** - Starts Firefox or any specified program
- **X11 Control Tools**:
  - Get screen information (dimensions, root window)
  - Move mouse cursor to specific coordinates
  - Click mouse buttons (left, middle, right)
  - Type text by sending key events

## Building

```bash
go build -o mcp-x11-controller .
```

## Usage

### Default mode (with Xvfb, i3, and Firefox)

```bash
./mcp-x11-controller
```

This will:
1. Start Xvfb on display :99 with 1024x768 resolution
2. Launch i3 window manager
3. Start Firefox
4. Begin accepting MCP commands

### Command-line options

```bash
./mcp-x11-controller [flags]
```

Flags:
- `-xvfb` (bool): Use Xvfb virtual display (default: true)
- `-display` (string): X11 display number (default: ":99")
- `-resolution` (string): Screen resolution WIDTHxHEIGHT (default: "1024x768")
- `-wm` (string): Window manager to use, e.g., i3, openbox (default: "i3")
- `-program` (string): Program to launch (default: "firefox")

### Examples

Run with a different resolution and Chrome:
```bash
./mcp-x11-controller -resolution "1920x1080" -program "google-chrome"
```

Run without window manager:
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

### move_mouse
Move the mouse cursor to specific coordinates.

**Arguments:**
- `x` (number): X coordinate
- `y` (number): Y coordinate

### click
Click a mouse button.

**Arguments:**
- `button` (number, optional): Button number (1=left, 2=middle, 3=right). Default: 1

### type_text
Type text by sending keyboard events.

**Arguments:**
- `text` (string): Text to type

**Note:** Currently supports only basic characters: a, e, h, l, o, space, and newline.

### take_screenshot
Take a screenshot of the X11 display.

**Arguments:**
- `filename` (string, optional): Filename for the screenshot. Default: "screenshot.png"

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

Move mouse:
```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "move_mouse",
    "arguments": {"x": 100, "y": 200}
  },
  "id": 2
}
```