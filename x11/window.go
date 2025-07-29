package x11

import (
	"fmt"
	"strings"

	x "github.com/linuxdeepin/go-x11-client"
)

// Window represents an X11 window
type Window struct {
	ID    x.Window
	Title string
	Class string
}

// ListWindows returns a list of all windows
func (c *Client) ListWindows() ([]Window, error) {
	// Get root window children
	cookie := x.QueryTree(c.conn, c.root)
	reply, err := cookie.Reply(c.conn)
	if err != nil {
		return nil, fmt.Errorf("failed to query tree: %w", err)
	}

	var windows []Window
	for _, win := range reply.Children {
		// Check if window is mapped (visible)
		attrCookie := x.GetWindowAttributes(c.conn, win)
		attrs, err := attrCookie.Reply(c.conn)
		if err != nil {
			continue
		}

		// Skip unmapped windows
		if attrs.MapState != x.MapStateViewable {
			continue
		}

		// Get window properties
		window := Window{ID: win}
		
		// Try to get window name
		if name := c.getWindowName(win); name != "" {
			window.Title = name
		}
		
		// Try to get window class
		if class := c.getWindowClass(win); class != "" {
			window.Class = class
		}

		// Only add windows that have a title or class
		if window.Title != "" || window.Class != "" {
			windows = append(windows, window)
		}
	}

	return windows, nil
}

// FocusWindow sets input focus to the specified window
func (c *Client) FocusWindow(windowID x.Window) error {
	// First, try to raise the window
	values := []uint32{x.StackModeAbove}
	x.ConfigureWindowChecked(c.conn, windowID, x.ConfigWindowStackMode, values).Check(c.conn)
	
	// Set input focus
	x.SetInputFocus(c.conn, x.InputFocusPointerRoot, windowID, x.TimeCurrentTime)
	
	// Ensure the commands are sent
	// Note: go-x11-client doesn't have Sync, but commands are sent immediately
	
	return nil
}

// getWindowName retrieves the window name
func (c *Client) getWindowName(win x.Window) string {
	// Try _NET_WM_NAME first (UTF-8)
	netWmName := c.getAtom("_NET_WM_NAME")
	if netWmName != 0 {
		if name := c.getStringProperty(win, netWmName); name != "" {
			return name
		}
	}
	
	// Fall back to WM_NAME
	wmName := c.getAtom("WM_NAME")
	if wmName != 0 {
		if name := c.getStringProperty(win, wmName); name != "" {
			return name
		}
	}
	
	return ""
}

// getWindowClass retrieves the window class
func (c *Client) getWindowClass(win x.Window) string {
	wmClass := c.getAtom("WM_CLASS")
	if wmClass == 0 {
		return ""
	}
	cookie := x.GetProperty(c.conn, false, win, wmClass, x.GetPropertyTypeAny, 0, 2048)
	reply, err := cookie.Reply(c.conn)
	if err != nil || len(reply.Value) == 0 {
		return ""
	}
	
	// WM_CLASS contains two null-terminated strings
	parts := strings.Split(string(reply.Value), "\x00")
	if len(parts) >= 2 && parts[1] != "" {
		return parts[1] // Return the class name (second part)
	} else if len(parts) >= 1 && parts[0] != "" {
		return parts[0] // Return the instance name if class is empty
	}
	
	return ""
}

// getStringProperty gets a string property from a window
func (c *Client) getStringProperty(win x.Window, prop x.Atom) string {
	cookie := x.GetProperty(c.conn, false, win, prop, x.GetPropertyTypeAny, 0, 2048)
	reply, err := cookie.Reply(c.conn)
	if err != nil || len(reply.Value) == 0 {
		return ""
	}
	
	// Remove null terminators and trim
	str := strings.TrimRight(string(reply.Value), "\x00")
	return strings.TrimSpace(str)
}

// getAtom gets or creates an atom
func (c *Client) getAtom(name string) x.Atom {
	cookie := x.InternAtom(c.conn, false, name)
	reply, err := cookie.Reply(c.conn)
	if err != nil {
		return 0
	}
	return reply.Atom
}