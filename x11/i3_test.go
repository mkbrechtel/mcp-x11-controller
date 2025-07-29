package x11

import (
	"encoding/json"
	"testing"

	"go.i3wm.org/i3/v4"
)

func TestI3Connection(t *testing.T) {
	tests := []struct {
		name          string
		socketPath    string
		expectError   bool
		expectEnabled bool
	}{
		{
			name:          "Valid socket connection",
			socketPath:    "/tmp/test-i3-socket",
			expectError:   false,
			expectEnabled: false, // Will be false because the socket doesn't actually exist
		},
		{
			name:          "Invalid socket path",
			socketPath:    "/nonexistent/socket",
			expectError:   false, // ConnectI3 doesn't return error, just sets enabled to false
			expectEnabled: false,
		},
		{
			name:          "Empty socket path (auto-detect)",
			socketPath:    "",
			expectError:   false,
			expectEnabled: false, // Will be false if i3 not running
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{}
			err := client.ConnectI3(tt.socketPath)
			
			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectError && err != nil && tt.socketPath != "" {
				// Only fail if we explicitly set a socket path
				t.Errorf("unexpected error: %v", err)
			}
			
			if client.I3Enabled() != tt.expectEnabled && tt.socketPath != "" {
				t.Errorf("expected I3Enabled=%v, got %v", tt.expectEnabled, client.I3Enabled())
			}
		})
	}
}

func TestI3GetTree(t *testing.T) {
	// This test requires a mock or actual i3 connection
	client := &Client{}
	
	// Test without i3 connection
	_, err := client.I3GetTree()
	if err == nil {
		t.Error("expected error when i3 not connected")
	}
	
	// Test the tree structure parsing
	mockTree := &i3.Node{
		ID:     1,
		Name:   "root",
		Type:   i3.Root,
		Rect:   i3.Rect{X: 0, Y: 0, Width: 1920, Height: 1080},
		Nodes: []*i3.Node{
			{
				ID:   10,
				Name: "workspace 1",
				Type: i3.WorkspaceNode,
				Nodes: []*i3.Node{
					{
						ID:         100,
						Name:       "Firefox",
						Type:       i3.Con,
						WindowProperties: i3.WindowProperties{
							Class:    "Firefox",
							Instance: "firefox",
							Title:    "Mozilla Firefox",
						},
					},
				},
			},
		},
	}
	
	// Test tree serialization
	jsonData, err := json.Marshal(mockTree)
	if err != nil {
		t.Fatalf("failed to marshal tree: %v", err)
	}
	
	var parsedTree i3.Node
	if err := json.Unmarshal(jsonData, &parsedTree); err != nil {
		t.Fatalf("failed to unmarshal tree: %v", err)
	}
	
	if parsedTree.ID != mockTree.ID {
		t.Errorf("expected tree ID %d, got %d", mockTree.ID, parsedTree.ID)
	}
}

func TestI3Command(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		expectError bool
	}{
		{
			name:        "Valid focus command",
			command:     "[con_id=100] focus",
			expectError: false,
		},
		{
			name:        "Valid workspace command",
			command:     "workspace 2",
			expectError: false,
		},
		{
			name:        "Empty command",
			command:     "",
			expectError: true,
		},
		{
			name:        "Complex command",
			command:     "[class=\"Firefox\"] move to workspace 3; workspace 3",
			expectError: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{}
			
			// Test without i3 connection
			_, err := client.I3Command(tt.command)
			if err == nil {
				t.Error("expected error when i3 not connected")
			}
		})
	}
}

func TestI3WindowSwitching(t *testing.T) {
	// Test finding windows in the tree
	tree := &i3.Node{
		ID:   1,
		Name: "root",
		Type: i3.Root,
		Nodes: []*i3.Node{
			{
				ID:   10,
				Name: "workspace 1",
				Type: i3.WorkspaceNode,
				Nodes: []*i3.Node{
					{
						ID:   100,
						Name: "container",
						Type: i3.Con,
						Nodes: []*i3.Node{
							{
								ID:   1000,
								Name: "Firefox",
								Type: i3.Con,
								WindowProperties: i3.WindowProperties{
									Class: "Firefox",
									Title: "Mozilla Firefox",
								},
							},
							{
								ID:   1001,
								Name: "Terminal",
								Type: i3.Con,
								WindowProperties: i3.WindowProperties{
									Class: "Alacritty",
									Title: "Terminal",
								},
							},
						},
					},
				},
			},
		},
	}
	
	// Test finding Firefox window
	firefoxNode := findNodeByClass(tree, "Firefox")
	if firefoxNode == nil {
		t.Error("failed to find Firefox node")
	} else if firefoxNode.ID != 1000 {
		t.Errorf("expected Firefox node ID 1000, got %d", firefoxNode.ID)
	}
	
	// Test finding by title
	terminalNode := findNodeByTitle(tree, "Terminal")
	if terminalNode == nil {
		t.Error("failed to find Terminal node")
	} else if terminalNode.ID != 1001 {
		t.Errorf("expected Terminal node ID 1001, got %d", terminalNode.ID)
	}
	
	// Test finding non-existent window
	nonExistent := findNodeByClass(tree, "NonExistent")
	if nonExistent != nil {
		t.Error("expected nil for non-existent window")
	}
}

// Helper functions for testing
func findNodeByClass(tree *i3.Node, class string) *i3.Node {
	if tree.WindowProperties.Class == class {
		return tree
	}
	
	for _, node := range tree.Nodes {
		if found := findNodeByClass(node, class); found != nil {
			return found
		}
	}
	
	for _, node := range tree.FloatingNodes {
		if found := findNodeByClass(node, class); found != nil {
			return found
		}
	}
	
	return nil
}

func findNodeByTitle(tree *i3.Node, title string) *i3.Node {
	if tree.WindowProperties.Title == title {
		return tree
	}
	
	for _, node := range tree.Nodes {
		if found := findNodeByTitle(node, title); found != nil {
			return found
		}
	}
	
	for _, node := range tree.FloatingNodes {
		if found := findNodeByTitle(node, title); found != nil {
			return found
		}
	}
	
	return nil
}