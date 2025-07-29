package x11

import (
	"encoding/json"
	"fmt"

	"go.i3wm.org/i3/v4"
)

// I3Enabled returns true if i3 connection is available
func (c *Client) I3Enabled() bool {
	return c.i3Connected
}

// ConnectI3 establishes a connection to i3 window manager
func (c *Client) ConnectI3(socketPath string) error {
	// Test if i3 is running by trying to get version
	if socketPath != "" {
		// Override socket path for testing
		oldHook := i3.SocketPathHook
		i3.SocketPathHook = func() (string, error) {
			return socketPath, nil
		}
		defer func() {
			i3.SocketPathHook = oldHook
		}()
	}
	
	// Try to get i3 version to test connection
	_, err := i3.GetVersion()
	if err != nil {
		// Not an error if i3 is not running, just means i3 features won't be available
		c.i3Connected = false
		return nil
	}
	
	c.i3Connected = true
	return nil
}

// I3GetTree returns the i3 window tree as JSON
func (c *Client) I3GetTree() (string, error) {
	if !c.I3Enabled() {
		return "", fmt.Errorf("i3 is not connected")
	}
	
	tree, err := i3.GetTree()
	if err != nil {
		return "", fmt.Errorf("failed to get i3 tree: %w", err)
	}
	
	// Convert to JSON for easy consumption
	jsonData, err := json.MarshalIndent(tree.Root, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal tree: %w", err)
	}
	
	return string(jsonData), nil
}

// I3Command sends a command to i3
func (c *Client) I3Command(command string) (string, error) {
	if !c.I3Enabled() {
		return "", fmt.Errorf("i3 is not connected")
	}
	
	if command == "" {
		return "", fmt.Errorf("command cannot be empty")
	}
	
	replies, err := i3.RunCommand(command)
	if err != nil {
		return "", fmt.Errorf("failed to run i3 command: %w", err)
	}
	
	// Format responses
	var results []string
	for _, reply := range replies {
		if reply.Success {
			results = append(results, "Success")
		} else {
			if reply.Error != "" {
				results = append(results, fmt.Sprintf("Error: %s", reply.Error))
			} else {
				results = append(results, "Failed")
			}
		}
	}
	
	// Return a simple string representation
	if len(results) == 1 {
		return results[0], nil
	}
	return fmt.Sprintf("%v", results), nil
}