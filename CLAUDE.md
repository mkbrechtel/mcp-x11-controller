## Workflow Tips
- Use go doc to access module documentation before trying to do websearch or searching directly in the files

## Testing MCP X11 Controller with Claude CLI

To test the mcp-x11-controller with Claude CLI, follow these steps:

1. **Use the provided MCP configuration file** (mcp-config.json) which uses `go run` to start the server.
   
   Note: The MCP server will automatically start Xvfb on display :99. It will launch with i3 window manager by default. To disable the window manager, modify the args to `["run", ".", "--no-wm"]`.

2. **Run Claude CLI with the MCP server**:
   ```bash
   claude --model sonnet --print \
     --mcp-config mcp-config.json \
     --dangerously-skip-permissions \
     "Take a screenshot of the X11 display"
   ```

   For more complex operations:
   ```bash
   claude --model sonnet --print \
     --mcp-config mcp-config.json \
     --dangerously-skip-permissions \
     "Start Firefox, navigate to example.com, click on the page, and take a screenshot"
   ```

Notes:
- The `--dangerously-skip-permissions` flag bypasses permission prompts for testing
- The window manager (i3) provides better window management for applications
